// nanolytica - Privacy-first analytics for small websites
package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/eringen/nanolytica/analytics"
	"github.com/eringen/nanolytica/analytics/templates"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Initialize analytics store
	dbPath := getEnv("NANOLYTICA_DB_PATH", "data/nanolytica.db")
	dbCfg := analytics.DefaultStoreConfig()
	if v := os.Getenv("NANOLYTICA_DB_MAX_OPEN_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			dbCfg.MaxOpenConns = n
		}
	}
	if v := os.Getenv("NANOLYTICA_DB_MAX_IDLE_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			dbCfg.MaxIdleConns = n
		}
	}

	registry, err := analytics.NewSiteRegistry(dbPath, dbCfg)
	if err != nil {
		log.Fatalf("Failed to initialize site registry: %v", err)
	}
	defer registry.Close()

	// Start daily cleanup of data older than 365 days
	stopCleanup := registry.StartCleanupSchedulers(365, 24*time.Hour)
	defer stopCleanup()

	// Start periodic cleanup of expired auth failure entries
	stopAuthCleanup := startAuthCleanup()
	defer stopAuthCleanup()

	// Cache tracking script in memory
	loadTrackingScript()

	// Create handler
	handler := analytics.NewHandler(registry)

	// Setup Echo
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Recover())

	// Skip heavy middleware on the collect endpoint — it returns 204 No Content
	// and doesn't need logging, compression, security headers, or body limiting.
	// This reduces per-request overhead by ~30-50% on the hottest path.
	collectSkipper := func(c echo.Context) bool {
		return c.Path() == "/api/analytics/collect"
	}

	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Skipper: collectSkipper,
		Format:  "${time_rfc3339} ${method} ${uri} ${status} ${latency_human}\n",
	}))

	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Skipper: collectSkipper,
	}))

	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		Skipper:               collectSkipper,
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'",
	}))

	// Limit request body size to prevent DoS
	e.Use(middleware.BodyLimitWithConfig(middleware.BodyLimitConfig{
		Skipper: collectSkipper,
		Limit:   "10K",
	}))

	// CORS scoped to public endpoints only (tracking script + collect + health)
	// NANOLYTICA_CORS_ORIGINS restricts which sites can send analytics.
	// Default "*" allows any origin (useful for multi-site tracking).
	corsOrigins := getEnv("NANOLYTICA_CORS_ORIGINS", "*")
	var allowedOrigins []string
	if corsOrigins == "*" {
		allowedOrigins = []string{"*"}
	} else {
		for _, o := range strings.Split(corsOrigins, ",") {
			if o = strings.TrimSpace(o); o != "" {
				allowedOrigins = append(allowedOrigins, o)
			}
		}
	}
	publicCORS := middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderContentType},
	})
	publicGroup := e.Group("", publicCORS)

	// Static files (embedded or filesystem based on build tags)
	serveEmbeddedStatic(e)

	// Tracking script (public, needs CORS)
	publicGroup.GET("/nanolytica.js", serveTrackingScript)
	publicGroup.GET("/static/js/analytics.min.js", serveTrackingScript)

	// Health check (public, needs CORS)
	publicGroup.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Initialize auth credentials and session secret
	initAuth()

	// Register analytics routes
	authMiddleware := createSessionAuthMiddleware()
	handler.RegisterRoutes(e, publicGroup, authMiddleware)

	// Login/logout routes (public)
	e.GET("/admin/login", handleLoginPage)
	e.POST("/admin/login", handleLogin)
	e.POST("/admin/logout", handleLogout)

	// Dashboard routes (protected)
	// Redirect /admin to /admin/analytics/
	e.GET("/admin", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/admin/analytics/")
	})

	// Redirect /admin/analytics (no trailing slash) to /admin/analytics/
	e.GET("/admin/analytics", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/admin/analytics/")
	})

	// Main dashboard page
	dashboardGroup := e.Group("/admin/analytics")
	dashboardGroup.Use(authMiddleware)
	dashboardGroup.GET("/", handler.DashboardHTML)

	// Start server with graceful shutdown
	port := getEnv("PORT", "8080")
	log.Printf("Nanolytica starting on http://localhost:%s", port)
	log.Printf("Dashboard: http://localhost:%s/admin/analytics/", port)
	log.Printf("Tracking script: http://localhost:%s/nanolytica.js", port)

	go func() {
		if err := e.Start(":" + port); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Wait for interrupt signal, then gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("Server stopped")
}

// cachedTrackingScript holds the tracking script content loaded once at startup.
var cachedTrackingScript string

// loadTrackingScript reads the minified script from disk at startup.
func loadTrackingScript() {
	data, err := os.ReadFile("static/js/analytics.min.js")
	if err != nil {
		log.Println("Warning: static/js/analytics.min.js not found. Run 'make assets' to build.")
		return
	}
	cachedTrackingScript = string(data)
}

// serveTrackingScript serves the cached analytics tracking script.
func serveTrackingScript(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/javascript")
	return c.String(http.StatusOK, cachedTrackingScript)
}

// authFailures tracks failed login attempts per IP for rate limiting.
var authFailures = struct {
	sync.Mutex
	attempts map[string][]time.Time
}{attempts: make(map[string][]time.Time)}

const (
	maxAuthAttempts    = 5
	authWindowSec      = 300 // 5 minutes
	authCleanupInterval = 10 * time.Minute
)

// checkRateLimit returns true if the IP has exceeded the max failed auth attempts.
func checkRateLimit(ip string) bool {
	authFailures.Lock()
	defer authFailures.Unlock()

	cutoff := time.Now().Add(-time.Duration(authWindowSec) * time.Second)
	recent := authFailures.attempts[ip][:0]
	for _, t := range authFailures.attempts[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) == 0 {
		delete(authFailures.attempts, ip)
		return false
	}
	authFailures.attempts[ip] = recent
	return len(recent) >= maxAuthAttempts
}

// recordAuthFailure records a failed auth attempt for rate limiting.
func recordAuthFailure(ip string) {
	authFailures.Lock()
	defer authFailures.Unlock()
	authFailures.attempts[ip] = append(authFailures.attempts[ip], time.Now())
}

// startAuthCleanup periodically removes expired entries from the authFailures map.
// Returns a stop function.
func startAuthCleanup() func() {
	ticker := time.NewTicker(authCleanupInterval)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				cleanupAuthFailures()
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	return func() { close(done) }
}

// cleanupAuthFailures removes all entries with only expired timestamps.
func cleanupAuthFailures() {
	authFailures.Lock()
	defer authFailures.Unlock()

	cutoff := time.Now().Add(-time.Duration(authWindowSec) * time.Second)
	for ip, times := range authFailures.attempts {
		recent := times[:0]
		for _, t := range times {
			if t.After(cutoff) {
				recent = append(recent, t)
			}
		}
		if len(recent) == 0 {
			delete(authFailures.attempts, ip)
		} else {
			authFailures.attempts[ip] = recent
		}
	}
}

// sessionCookieName is the name of the auth session cookie.
const sessionCookieName = "nanolytica_session"

// sessionSecret holds the HMAC key for signing session cookies.
var sessionSecret []byte

// adminUsername and adminPassword hold the configured credentials.
var adminUsername, adminPassword string

// initAuth sets up admin credentials and session secret.
func initAuth() {
	adminUsername = os.Getenv("NANOLYTICA_USERNAME")
	adminPassword = os.Getenv("NANOLYTICA_PASSWORD")

	if adminUsername == "" || adminPassword == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			log.Fatalf("Failed to generate random password: %v", err)
		}
		adminUsername = "admin"
		adminPassword = hex.EncodeToString(b)
		log.Printf("WARNING: No NANOLYTICA_USERNAME/NANOLYTICA_PASSWORD set. Generated credentials: %s / %s", adminUsername, adminPassword)
	}

	// Generate a random session secret at startup.
	// Sessions are invalidated on restart, which is acceptable for this use case.
	sessionSecret = make([]byte, 32)
	if _, err := rand.Read(sessionSecret); err != nil {
		log.Fatalf("Failed to generate session secret: %v", err)
	}
}

// signSession creates a signed session token: "username|timestamp|hmac".
func signSession(username string) string {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	payload := username + "|" + ts
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payload + "|" + sig
}

// verifySession checks a signed session token. Returns true if valid and not expired (30 days).
func verifySession(token string) bool {
	parts := strings.SplitN(token, "|", 3)
	if len(parts) != 3 {
		return false
	}
	payload := parts[0] + "|" + parts[1]
	sig, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(payload))
	if !hmac.Equal(mac.Sum(nil), sig) {
		return false
	}
	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return false
	}
	// Expire after 30 days
	if time.Now().Unix()-ts > 30*24*60*60 {
		return false
	}
	return true
}

// createSessionAuthMiddleware returns middleware that redirects to login if not authenticated.
func createSessionAuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Cookie(sessionCookieName)
			if err != nil || !verifySession(cookie.Value) {
				return c.Redirect(http.StatusSeeOther, "/admin/login")
			}
			return next(c)
		}
	}
}

// handleLoginPage renders the login form.
func handleLoginPage(c echo.Context) error {
	// If already logged in, redirect to dashboard
	cookie, err := c.Cookie(sessionCookieName)
	if err == nil && verifySession(cookie.Value) {
		return c.Redirect(http.StatusSeeOther, "/admin/analytics/")
	}
	return templates.LoginPage("").Render(c.Request().Context(), c.Response())
}

// handleLogin processes the login form submission.
func handleLogin(c echo.Context) error {
	ip := c.RealIP()
	if checkRateLimit(ip) {
		return templates.LoginPage("Too many failed attempts. Please try again later.").Render(c.Request().Context(), c.Response())
	}

	user := c.FormValue("username")
	pass := c.FormValue("password")

	userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(adminUsername))
	passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(adminPassword))

	if userMatch != 1 || passMatch != 1 {
		recordAuthFailure(ip)
		c.Response().Status = http.StatusUnauthorized
		return templates.LoginPage("Invalid username or password.").Render(c.Request().Context(), c.Response())
	}

	// Set session cookie
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    signSession(user),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 60 * 60, // 30 days
	}
	if os.Getenv("COOKIE_SECURE") == "true" {
		cookie.Secure = true
	}
	c.SetCookie(cookie)

	return c.Redirect(http.StatusSeeOther, "/admin/analytics/")
}

// handleLogout clears the session cookie.
func handleLogout(c echo.Context) error {
	c.SetCookie(&http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	return c.Redirect(http.StatusSeeOther, "/admin/login")
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

