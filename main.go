// nanolytica - Privacy-first analytics for small websites
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/eringen/nanolytica/analytics"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Initialize analytics store
	dbPath := getEnv("NANOLYTICA_DB_PATH", "data/nanolytica.db")
	store, err := analytics.NewStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize analytics store: %v", err)
	}
	defer store.Close()

	// Start daily cleanup of data older than 365 days
	stopCleanup := store.StartCleanupScheduler(365, 24*time.Hour)
	defer stopCleanup()

	// Initialize hash salt (persisted in DB)
	if err := analytics.InitSalt(store); err != nil {
		log.Fatalf("Failed to initialize hash salt: %v", err)
	}

	// Cache tracking script in memory
	loadTrackingScript()

	// Create handler
	handler := analytics.NewHandler(store)

	// Setup Echo
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Recover())
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "${time_rfc3339} ${method} ${uri} ${status} ${latency_human}\n",
	}))

	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'",
	}))

	// Limit request body size to prevent DoS
	e.Use(middleware.BodyLimit("10K"))

	// CORS scoped to public endpoints only (tracking script + collect + health)
	publicCORS := middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
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

	// Register analytics routes
	authMiddleware := createAuthMiddleware()
	handler.RegisterRoutes(e, publicGroup, authMiddleware)

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
	maxAuthAttempts = 5
	authWindowSec   = 300 // 5 minutes
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
	authFailures.attempts[ip] = recent
	return len(recent) >= maxAuthAttempts
}

// recordAuthFailure records a failed auth attempt for rate limiting.
func recordAuthFailure(ip string) {
	authFailures.Lock()
	defer authFailures.Unlock()
	authFailures.attempts[ip] = append(authFailures.attempts[ip], time.Now())
}

// createAuthMiddleware creates basic auth middleware for dashboard.
// If no credentials are configured, a random password is generated and logged.
func createAuthMiddleware() echo.MiddlewareFunc {
	username := os.Getenv("NANOLYTICA_USERNAME")
	password := os.Getenv("NANOLYTICA_PASSWORD")

	if username == "" || password == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			log.Fatalf("Failed to generate random password: %v", err)
		}
		username = "admin"
		password = hex.EncodeToString(b)
		log.Printf("WARNING: No NANOLYTICA_USERNAME/NANOLYTICA_PASSWORD set. Generated credentials: %s / %s", username, password)
	}

	return middleware.BasicAuth(func(user, pass string, c echo.Context) (bool, error) {
		ip := c.RealIP()
		if checkRateLimit(ip) {
			return false, nil
		}
		userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(username))
		passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(password))
		if userMatch == 1 && passMatch == 1 {
			return true, nil
		}
		recordAuthFailure(ip)
		return false, nil
	})
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

