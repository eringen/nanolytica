package analytics

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/eringen/nanolytica/analytics/templates"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// screenSizeRegex validates screen size format (e.g., "1920x1080").
var screenSizeRegex = regexp.MustCompile(`^\d{1,5}x\d{1,5}$`)

// pathRegex validates URL path content. Allows printable ASCII and UTF-8,
// rejects control characters and null bytes.
var pathRegex = regexp.MustCompile(`^[^\x00-\x1f\x7f]*$`)

// Handler handles analytics HTTP requests.
type Handler struct {
	registry *SiteRegistry
}

// NewHandler creates a new analytics handler.
func NewHandler(registry *SiteRegistry) *Handler {
	return &Handler{registry: registry}
}

// getStore returns the Store for the site specified in the query param, falling back to default.
func (h *Handler) getStore(c echo.Context) *Store {
	site := c.QueryParam("site")
	if site == "" {
		site = "default"
	}
	store := h.registry.GetStore(site)
	if store == nil {
		return h.registry.DefaultStore()
	}
	return store
}

// CollectRequest is the expected request body for the collect endpoint.
type CollectRequest struct {
	Path        string `json:"path"`
	Referrer    string `json:"referrer"`
	ScreenSize  string `json:"screen_size"`
	UserAgent   string `json:"user_agent"`
	DurationSec int    `json:"duration_sec"`
	ScrollDepth int    `json:"scroll_depth"`
	Site        string `json:"site"`
}

// Input validation limits for the collect endpoint.
const (
	maxPathLen       = 2048
	maxReferrerLen   = 2048
	maxScreenSizeLen = 32
	maxUserAgentLen  = 512
	maxDurationSec   = 86400 // 24 hours
	maxScrollDepth   = 100
	maxSiteLen       = 64
)

// validateCollectRequest checks field lengths and value ranges.
func validateCollectRequest(req *CollectRequest) error {
	if len(req.Path) > maxPathLen {
		return fmt.Errorf("path exceeds maximum length of %d", maxPathLen)
	}
	if req.Path != "" && !pathRegex.MatchString(req.Path) {
		return fmt.Errorf("path contains invalid characters")
	}
	if len(req.Referrer) > maxReferrerLen {
		return fmt.Errorf("referrer exceeds maximum length of %d", maxReferrerLen)
	}
	if len(req.ScreenSize) > maxScreenSizeLen {
		return fmt.Errorf("screen_size exceeds maximum length of %d", maxScreenSizeLen)
	}
	if req.ScreenSize != "" && !screenSizeRegex.MatchString(req.ScreenSize) {
		return fmt.Errorf("screen_size must match format WIDTHxHEIGHT (e.g., 1920x1080)")
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
	if req.ScrollDepth < 0 {
		return fmt.Errorf("scroll_depth must not be negative")
	}
	if req.ScrollDepth > maxScrollDepth {
		return fmt.Errorf("scroll_depth exceeds maximum of %d", maxScrollDepth)
	}
	if len(req.Site) > maxSiteLen {
		return fmt.Errorf("site exceeds maximum length of %d", maxSiteLen)
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

	// Get store for site
	site := req.Site
	if site == "" {
		site = "default"
	}
	store := h.registry.GetStore(site)
	if store == nil {
		// Unknown site — silently ignore
		return c.NoContent(http.StatusNoContent)
	}

	// Handle bot visits separately
	if IsBot(userAgent) {
		store.EnqueueBotVisit(&BotVisit{
			BotName:   ExtractBotName(userAgent),
			IPHash:    store.HashIP(ip),
			UserAgent: userAgent,
			Path:      req.Path,
			Timestamp: time.Now().UTC(),
		})
		return c.NoContent(http.StatusNoContent)
	}

	// Generate visitor ID
	visitorID := store.GenerateVisitorID(ip, userAgent)

	// Parse browser, OS, device
	browser, os, device := ParseUserAgent(userAgent)

	// Clean referrer
	referrer := CleanReferrer(req.Referrer)

	// Enqueue visit for async batch insert
	store.EnqueueVisit(&Visit{
		VisitorID:   visitorID,
		SessionID:   generateSessionID(visitorID),
		IPHash:      store.HashIP(ip),
		Browser:     browser,
		OS:          os,
		Device:      device,
		Path:        req.Path,
		Referrer:    referrer,
		ScreenSize:  req.ScreenSize,
		Timestamp:   time.Now().UTC(),
		DurationSec: req.DurationSec,
		ScrollDepth: req.ScrollDepth,
	})

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

	from, to := periodTimeRange(days, hourly)

	store := h.getStore(c)
	stats, err := store.GetStats(from, to, hourly, monthly)
	if err != nil {
		c.Logger().Errorf("Failed to get stats: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
	}

	realtime, _ := store.GetRealtimeVisitors()

	return c.JSON(http.StatusOK, StatsResponse{
		Stats:      stats,
		Realtime:   realtime,
		PeriodDays: days,
		Hourly:     hourly,
		Monthly:    monthly,
	})
}

// GetStatsFragment returns HTML fragment for visitor stats (talkDOM)
func (h *Handler) GetStatsFragment(c echo.Context) error {
	_, days, hourly, monthly := parsePeriod(c.QueryParam("period"))

	from, to := periodTimeRange(days, hourly)

	store := h.getStore(c)
	stats, err := store.GetStats(from, to, hourly, monthly)
	if err != nil {
		c.Logger().Errorf("Failed to get stats fragment: %v", err)
		return c.HTML(http.StatusInternalServerError, "<div class='loading'>Error loading data</div>")
	}

	realtime, _ := store.GetRealtimeVisitors()

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

	from, to := periodTimeRange(days, hourly)

	store := h.getStore(c)
	stats, err := store.GetBotStats(from, to, hourly, monthly)
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

// GetBotStatsFragment returns HTML fragment for bot stats (talkDOM)
func (h *Handler) GetBotStatsFragment(c echo.Context) error {
	_, days, hourly, monthly := parsePeriod(c.QueryParam("period"))

	from, to := periodTimeRange(days, hourly)

	store := h.getStore(c)
	stats, err := store.GetBotStats(from, to, hourly, monthly)
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

// GetSetupFragment returns HTML fragment for setup tab (talkDOM)
func (h *Handler) GetSetupFragment(c echo.Context) error {
	origin := c.Scheme() + "://" + c.Request().Host
	site := c.QueryParam("site")
	if site == "" {
		site = "default"
	}
	component := templates.SetupContent(origin, site)
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

// periodTimeRange computes the from/to time range for a given period.
// For hourly (last 24 hours), it uses a rolling 24-hour window aligned to hour boundaries.
// For other periods, it uses calendar day boundaries.
func periodTimeRange(days int, hourly bool) (time.Time, time.Time) {
	now := time.Now().UTC()
	if hourly {
		currentHour := now.Truncate(time.Hour)
		from := currentHour.Add(-23 * time.Hour)
		to := currentHour.Add(time.Hour)
		return from, to
	}
	from := now.AddDate(0, 0, -days).Truncate(24 * time.Hour)
	to := now.Add(24 * time.Hour).Truncate(24 * time.Hour)
	return from, to
}

// convertStatsToViewModel converts analytics.Stats to templates.StatsViewModel
func convertStatsToViewModel(stats *Stats) *templates.StatsViewModel {
	vm := &templates.StatsViewModel{
		Period:         stats.Period,
		UniqueVisitors: stats.UniqueVisitors,
		TotalViews:     stats.TotalViews,
		AvgDuration:    stats.AvgDuration,
		AvgScrollDepth: stats.AvgScrollDepth,
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

// ExportStatsCSV returns visitor stats as a CSV download.
func (h *Handler) ExportStatsCSV(c echo.Context) error {
	period, days, hourly, monthly := parsePeriod(c.QueryParam("period"))
	from, to := periodTimeRange(days, hourly)

	store := h.getStore(c)
	stats, err := store.GetStats(from, to, hourly, monthly)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to export stats")
	}

	c.Response().Header().Set("Content-Type", "text/csv")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=visitors_%s.csv", period))
	c.Response().WriteHeader(http.StatusOK)

	w := csv.NewWriter(c.Response())

	// Summary
	w.Write([]string{"# Summary"})
	w.Write([]string{"Period", stats.Period})
	w.Write([]string{"Unique Visitors", strconv.Itoa(stats.UniqueVisitors)})
	w.Write([]string{"Total Views", strconv.Itoa(stats.TotalViews)})
	w.Write([]string{"Avg Duration (sec)", strconv.Itoa(stats.AvgDuration)})
	w.Write([]string{"Avg Scroll Depth (%)", strconv.Itoa(stats.AvgScrollDepth)})
	w.Write([]string{})

	// Views over time
	w.Write([]string{"# Views Over Time"})
	w.Write([]string{"Date", "Views"})
	for _, v := range stats.DailyViews {
		w.Write([]string{v.Date, strconv.Itoa(v.Views)})
	}
	w.Write([]string{})

	// Top pages
	w.Write([]string{"# Top Pages"})
	w.Write([]string{"Path", "Views"})
	for _, p := range stats.TopPages {
		w.Write([]string{p.Path, strconv.Itoa(p.Views)})
	}
	w.Write([]string{})

	// Browsers
	w.Write([]string{"# Browsers"})
	w.Write([]string{"Browser", "Count"})
	for _, s := range stats.BrowserStats {
		w.Write([]string{s.Name, strconv.Itoa(s.Count)})
	}
	w.Write([]string{})

	// Operating Systems
	w.Write([]string{"# Operating Systems"})
	w.Write([]string{"OS", "Count"})
	for _, s := range stats.OSStats {
		w.Write([]string{s.Name, strconv.Itoa(s.Count)})
	}
	w.Write([]string{})

	// Devices
	w.Write([]string{"# Devices"})
	w.Write([]string{"Device", "Count"})
	for _, s := range stats.DeviceStats {
		w.Write([]string{s.Name, strconv.Itoa(s.Count)})
	}
	w.Write([]string{})

	// Referrers
	w.Write([]string{"# Referrers"})
	w.Write([]string{"Referrer", "Count"})
	for _, s := range stats.ReferrerStats {
		w.Write([]string{s.Name, strconv.Itoa(s.Count)})
	}

	w.Flush()
	return nil
}

// ExportBotStatsCSV returns bot stats as a CSV download.
func (h *Handler) ExportBotStatsCSV(c echo.Context) error {
	period, days, hourly, monthly := parsePeriod(c.QueryParam("period"))
	from, to := periodTimeRange(days, hourly)

	store := h.getStore(c)
	stats, err := store.GetBotStats(from, to, hourly, monthly)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to export bot stats")
	}

	c.Response().Header().Set("Content-Type", "text/csv")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=bots_%s.csv", period))
	c.Response().WriteHeader(http.StatusOK)

	w := csv.NewWriter(c.Response())

	// Summary
	w.Write([]string{"# Summary"})
	w.Write([]string{"Period", stats.Period})
	w.Write([]string{"Total Bot Visits", strconv.Itoa(stats.TotalVisits)})
	w.Write([]string{})

	// Bot visits over time
	w.Write([]string{"# Bot Visits Over Time"})
	w.Write([]string{"Date", "Visits"})
	for _, v := range stats.DailyVisits {
		w.Write([]string{v.Date, strconv.Itoa(v.Views)})
	}
	w.Write([]string{})

	// Top bots
	w.Write([]string{"# Top Bots"})
	w.Write([]string{"Bot", "Count"})
	for _, b := range stats.TopBots {
		w.Write([]string{b.Name, strconv.Itoa(b.Count)})
	}
	w.Write([]string{})

	// Top pages
	w.Write([]string{"# Top Pages"})
	w.Write([]string{"Path", "Views"})
	for _, p := range stats.TopPages {
		w.Write([]string{p.Path, strconv.Itoa(p.Views)})
	}

	w.Flush()
	return nil
}

// AddSite handles POST requests to add a new site.
func (h *Handler) AddSite(c echo.Context) error {
	name := strings.TrimSpace(c.FormValue("site_name"))
	if name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Site name is required"})
	}
	if err := h.registry.AddSite(name); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok", "site": name})
}

// RegisterRoutes registers analytics routes with the Echo router.
func (h *Handler) RegisterRoutes(e *echo.Echo, publicGroup *echo.Group, authMiddleware echo.MiddlewareFunc) {
	// Rate limit the collect endpoint: 5 req/s per IP, burst of 10
	collectRateLimiter := middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{Rate: 5, Burst: 10, ExpiresIn: 5 * time.Minute},
		),
		IdentifierExtractor: func(c echo.Context) (string, error) {
			return c.RealIP(), nil
		},
		DenyHandler: func(c echo.Context, identifier string, err error) error {
			return c.NoContent(http.StatusTooManyRequests)
		},
	})

	// Public endpoint for collecting analytics (with CORS + rate limit)
	publicGroup.POST("/api/analytics/collect", h.Collect, collectRateLimiter)

	// Admin API endpoints (JSON)
	admin := e.Group("/admin/analytics")
	admin.Use(authMiddleware)
	admin.GET("/api/stats", h.GetStats)
	admin.GET("/api/bot-stats", h.GetBotStats)

	// CSV export endpoints
	admin.GET("/api/export/stats", h.ExportStatsCSV)
	admin.GET("/api/export/bot-stats", h.ExportBotStatsCSV)

	// Admin fragment endpoints (HTML for talkDOM)
	admin.GET("/fragments/stats", h.GetStatsFragment)
	admin.GET("/fragments/bot-stats", h.GetBotStatsFragment)
	admin.GET("/fragments/setup", h.GetSetupFragment)

	// Site management
	admin.POST("/api/sites", h.AddSite)
}

// Dashboard renders the analytics dashboard HTML.
func (h *Handler) Dashboard(c echo.Context) error {
	return c.Redirect(http.StatusSeeOther, "/admin/analytics/")
}

// DashboardHTML serves the standalone HTML dashboard using templ.
func (h *Handler) DashboardHTML(c echo.Context) error {
	sites := h.registry.ListSites()
	currentSite := c.QueryParam("site")
	if currentSite == "" {
		currentSite = "default"
	}
	return templates.Dashboard(sites, currentSite).Render(c.Request().Context(), c.Response())
}
