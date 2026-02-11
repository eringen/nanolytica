package analytics

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/eringen/nanolytica/analytics/templates"
	"github.com/labstack/echo/v4"
)

// Handler handles analytics HTTP requests.
type Handler struct {
	store *Store
}

// NewHandler creates a new analytics handler.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// CollectRequest is the expected request body for the collect endpoint.
type CollectRequest struct {
	Path        string `json:"path"`
	Referrer    string `json:"referrer"`
	ScreenSize  string `json:"screen_size"`
	UserAgent   string `json:"user_agent"`
	DurationSec int    `json:"duration_sec"`
}

// Input validation limits for the collect endpoint.
const (
	maxPathLen       = 2048
	maxReferrerLen   = 2048
	maxScreenSizeLen = 32
	maxUserAgentLen  = 512
	maxDurationSec   = 86400 // 24 hours
)

// validateCollectRequest checks field lengths and value ranges.
func validateCollectRequest(req *CollectRequest) error {
	if len(req.Path) > maxPathLen {
		return fmt.Errorf("path exceeds maximum length of %d", maxPathLen)
	}
	if len(req.Referrer) > maxReferrerLen {
		return fmt.Errorf("referrer exceeds maximum length of %d", maxReferrerLen)
	}
	if len(req.ScreenSize) > maxScreenSizeLen {
		return fmt.Errorf("screen_size exceeds maximum length of %d", maxScreenSizeLen)
	}
	if len(req.UserAgent) > maxUserAgentLen {
		return fmt.Errorf("user_agent exceeds maximum length of %d", maxUserAgentLen)
	}
	if req.DurationSec < 0 {
		return fmt.Errorf("duration_sec must not be negative")
	}
	if req.DurationSec > maxDurationSec {
		return fmt.Errorf("duration_sec exceeds maximum of %d", maxDurationSec)
	}
	return nil
}

// Collect handles incoming analytics data from clients.
func (h *Handler) Collect(c echo.Context) error {
	// Check for Do Not Track
	if c.Request().Header.Get("DNT") == "1" {
		return c.NoContent(http.StatusNoContent)
	}

	// Parse request
	var req CollectRequest
	if err := c.Bind(&req); err != nil {
		return c.String(http.StatusBadRequest, "Invalid request")
	}

	// Validate input
	if err := validateCollectRequest(&req); err != nil {
		return c.String(http.StatusBadRequest, "Invalid request")
	}

	// Get User-Agent from request if not provided
	userAgent := req.UserAgent
	if userAgent == "" {
		userAgent = c.Request().UserAgent()
	}

	// Get client IP
	ip := c.RealIP()

	// Handle bot visits separately
	if IsBot(userAgent) {
		botVisit := &BotVisit{
			BotName:   ExtractBotName(userAgent),
			IPHash:    HashIP(ip),
			UserAgent: userAgent,
			Path:      req.Path,
			Timestamp: time.Now().UTC(),
		}
		if err := h.store.SaveBotVisit(botVisit); err != nil {
			c.Logger().Errorf("Failed to save bot visit: %v", err)
		}
		return c.NoContent(http.StatusNoContent)
	}

	// Generate visitor ID
	visitorID := GenerateVisitorID(ip, userAgent)

	// Parse browser, OS, device
	browser, os, device := ParseUserAgent(userAgent)

	// Clean referrer
	referrer := CleanReferrer(req.Referrer)

	// Create visit
	visit := &Visit{
		VisitorID:   visitorID,
		SessionID:   generateSessionID(visitorID),
		IPHash:      HashIP(ip),
		Browser:     browser,
		OS:          os,
		Device:      device,
		Path:        req.Path,
		Referrer:    referrer,
		ScreenSize:  req.ScreenSize,
		Timestamp:   time.Now().UTC(),
		DurationSec: req.DurationSec,
	}

	// Save to database
	if err := h.store.SaveVisit(visit); err != nil {
		c.Logger().Errorf("Failed to save visit: %v", err)
	}

	return c.NoContent(http.StatusNoContent)
}

// StatsResponse is the JSON response for stats endpoint.
type StatsResponse struct {
	Stats      *Stats `json:"stats"`
	Realtime   int    `json:"realtime_visitors"`
	PeriodDays int    `json:"period_days"`
	Hourly     bool   `json:"hourly"`
	Monthly    bool   `json:"monthly"`
}

// GetStats returns analytics statistics as JSON.
func (h *Handler) GetStats(c echo.Context) error {
	_, days, hourly, monthly := parsePeriod(c.QueryParam("period"))

	from := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)
	to := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)

	stats, err := h.store.GetStats(from, to, hourly, monthly)
	if err != nil {
		c.Logger().Errorf("Failed to get stats: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
	}

	realtime, _ := h.store.GetRealtimeVisitors()

	return c.JSON(http.StatusOK, StatsResponse{
		Stats:      stats,
		Realtime:   realtime,
		PeriodDays: days,
		Hourly:     hourly,
		Monthly:    monthly,
	})
}

// GetStatsFragment returns HTML fragment for visitor stats (htmx)
func (h *Handler) GetStatsFragment(c echo.Context) error {
	_, days, hourly, monthly := parsePeriod(c.QueryParam("period"))

	from := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)
	to := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)

	stats, err := h.store.GetStats(from, to, hourly, monthly)
	if err != nil {
		c.Logger().Errorf("Failed to get stats fragment: %v", err)
		return c.HTML(http.StatusInternalServerError, "<div class='loading'>Error loading data</div>")
	}

	realtime, _ := h.store.GetRealtimeVisitors()

	// Convert to view model
	statsVM := convertStatsToViewModel(stats)

	// Return only the stats content, not the period selector (to avoid duplication)
	component := templates.StatsFragmentOnly(statsVM, realtime, days, hourly, monthly)
	return component.Render(c.Request().Context(), c.Response())
}

// BotStatsResponse is the JSON response for bot stats endpoint.
type BotStatsResponse struct {
	Stats      *BotStats `json:"stats"`
	PeriodDays int       `json:"period_days"`
	Hourly     bool      `json:"hourly"`
	Monthly    bool      `json:"monthly"`
}

// GetBotStats returns bot analytics statistics as JSON.
func (h *Handler) GetBotStats(c echo.Context) error {
	_, days, hourly, monthly := parsePeriod(c.QueryParam("period"))

	from := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)
	to := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)

	stats, err := h.store.GetBotStats(from, to, hourly, monthly)
	if err != nil {
		c.Logger().Errorf("Failed to get bot stats: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
	}

	return c.JSON(http.StatusOK, BotStatsResponse{
		Stats:      stats,
		PeriodDays: days,
		Hourly:     hourly,
		Monthly:    monthly,
	})
}

// GetBotStatsFragment returns HTML fragment for bot stats (htmx)
func (h *Handler) GetBotStatsFragment(c echo.Context) error {
	_, days, hourly, monthly := parsePeriod(c.QueryParam("period"))

	from := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)
	to := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)

	stats, err := h.store.GetBotStats(from, to, hourly, monthly)
	if err != nil {
		c.Logger().Errorf("Failed to get bot stats fragment: %v", err)
		return c.HTML(http.StatusInternalServerError, "<div class='loading'>Error loading data</div>")
	}

	// Convert to view model
	statsVM := convertBotStatsToViewModel(stats)

	// Return only the stats content, not the period selector (to avoid duplication)
	component := templates.BotStatsFragmentOnly(statsVM, days, hourly, monthly)
	return component.Render(c.Request().Context(), c.Response())
}

// GetSetupFragment returns HTML fragment for setup tab (htmx)
func (h *Handler) GetSetupFragment(c echo.Context) error {
	origin := c.Scheme() + "://" + c.Request().Host
	component := templates.SetupContent(origin)
	return component.Render(c.Request().Context(), c.Response())
}

// parsePeriod parses the period query parameter
func parsePeriod(period string) (string, int, bool, bool) {
	var days int
	var hourly bool
	var monthly bool

	switch period {
	case "today":
		days = 1
		hourly = true
		monthly = false
	case "week":
		days = 7
		hourly = false
		monthly = false
	case "month":
		days = 30
		hourly = false
		monthly = false
	case "year":
		days = 365
		hourly = false
		monthly = true
	default:
		days = 7
		hourly = false
		monthly = false
		period = "week"
	}

	return period, days, hourly, monthly
}

// convertStatsToViewModel converts analytics.Stats to templates.StatsViewModel
func convertStatsToViewModel(stats *Stats) *templates.StatsViewModel {
	vm := &templates.StatsViewModel{
		Period:         stats.Period,
		UniqueVisitors: stats.UniqueVisitors,
		TotalViews:     stats.TotalViews,
		AvgDuration:    stats.AvgDuration,
	}

	// Convert TopPages
	vm.TopPages = make([]templates.PageStatViewModel, len(stats.TopPages))
	for i, p := range stats.TopPages {
		vm.TopPages[i] = templates.PageStatViewModel{
			Path:  p.Path,
			Views: p.Views,
		}
	}

	// Convert LatestPages
	vm.LatestPages = make([]templates.LatestPageVisitViewModel, len(stats.LatestPages))
	for i, p := range stats.LatestPages {
		vm.LatestPages[i] = templates.LatestPageVisitViewModel{
			Path:      p.Path,
			Timestamp: p.Timestamp,
			Browser:   p.Browser,
		}
	}

	// Convert BrowserStats
	vm.BrowserStats = make([]templates.DimensionStatViewModel, len(stats.BrowserStats))
	for i, s := range stats.BrowserStats {
		vm.BrowserStats[i] = templates.DimensionStatViewModel{
			Name:  s.Name,
			Count: s.Count,
		}
	}

	// Convert OSStats
	vm.OSStats = make([]templates.DimensionStatViewModel, len(stats.OSStats))
	for i, s := range stats.OSStats {
		vm.OSStats[i] = templates.DimensionStatViewModel{
			Name:  s.Name,
			Count: s.Count,
		}
	}

	// Convert DeviceStats
	vm.DeviceStats = make([]templates.DimensionStatViewModel, len(stats.DeviceStats))
	for i, s := range stats.DeviceStats {
		vm.DeviceStats[i] = templates.DimensionStatViewModel{
			Name:  s.Name,
			Count: s.Count,
		}
	}

	// Convert ReferrerStats
	vm.ReferrerStats = make([]templates.DimensionStatViewModel, len(stats.ReferrerStats))
	for i, s := range stats.ReferrerStats {
		vm.ReferrerStats[i] = templates.DimensionStatViewModel{
			Name:  s.Name,
			Count: s.Count,
		}
	}

	// Convert DailyViews
	vm.DailyViews = make([]templates.DailyViewViewModel, len(stats.DailyViews))
	for i, v := range stats.DailyViews {
		vm.DailyViews[i] = templates.DailyViewViewModel{
			Date:  v.Date,
			Views: v.Views,
		}
	}

	return vm
}

// convertBotStatsToViewModel converts analytics.BotStats to templates.BotStatsViewModel
func convertBotStatsToViewModel(stats *BotStats) *templates.BotStatsViewModel {
	vm := &templates.BotStatsViewModel{
		Period:      stats.Period,
		TotalVisits: stats.TotalVisits,
	}

	// Convert TopBots
	vm.TopBots = make([]templates.DimensionStatViewModel, len(stats.TopBots))
	for i, b := range stats.TopBots {
		vm.TopBots[i] = templates.DimensionStatViewModel{
			Name:  b.Name,
			Count: b.Count,
		}
	}

	// Convert TopPages
	vm.TopPages = make([]templates.PageStatViewModel, len(stats.TopPages))
	for i, p := range stats.TopPages {
		vm.TopPages[i] = templates.PageStatViewModel{
			Path:  p.Path,
			Views: p.Views,
		}
	}

	// Convert DailyVisits
	vm.DailyVisits = make([]templates.DailyViewViewModel, len(stats.DailyVisits))
	for i, v := range stats.DailyVisits {
		vm.DailyVisits[i] = templates.DailyViewViewModel{
			Date:  v.Date,
			Views: v.Views,
		}
	}

	return vm
}

// generateSessionID creates a session ID derived from visitor identity and date.
// Visitors get the same session ID for the same calendar day (UTC).
func generateSessionID(visitorID string) string {
	day := time.Now().UTC().Format("2006-01-02")
	h := sha256.New()
	h.Write([]byte(visitorID + "|" + day))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// RegisterRoutes registers analytics routes with the Echo router.
func (h *Handler) RegisterRoutes(e *echo.Echo, publicGroup *echo.Group, authMiddleware echo.MiddlewareFunc) {
	// Public endpoint for collecting analytics (with CORS)
	publicGroup.POST("/api/analytics/collect", h.Collect)

	// Admin API endpoints (JSON)
	admin := e.Group("/admin/analytics")
	admin.Use(authMiddleware)
	admin.GET("/api/stats", h.GetStats)
	admin.GET("/api/bot-stats", h.GetBotStats)

	// Admin fragment endpoints (HTML for htmx)
	admin.GET("/fragments/stats", h.GetStatsFragment)
	admin.GET("/fragments/bot-stats", h.GetBotStatsFragment)
	admin.GET("/fragments/setup", h.GetSetupFragment)
}

// Dashboard renders the analytics dashboard HTML.
func (h *Handler) Dashboard(c echo.Context) error {
	return c.Redirect(http.StatusSeeOther, "/admin/analytics/")
}

// DashboardHTML serves the standalone HTML dashboard using templ.
func (h *Handler) DashboardHTML(c echo.Context) error {
	return templates.Dashboard().Render(c.Request().Context(), c.Response())
}
