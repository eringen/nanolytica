package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/eringen/nanolytica/analytics/sqlcgen"
	_ "modernc.org/sqlite"
)

// Store provides database operations for analytics.
type Store struct {
	db *sql.DB
	q  *sqlcgen.Queries
}

// NewStore creates a new analytics store.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open analytics db: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	s := &Store{
		db: db,
		q:  sqlcgen.New(db),
	}
	if err := s.ensureSchema(); err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// ensureSchema creates the necessary tables if they don't exist.
func (s *Store) ensureSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS visits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			visitor_id TEXT NOT NULL,
			session_id TEXT NOT NULL,
			ip_hash TEXT NOT NULL,
			browser TEXT NOT NULL,
			os TEXT NOT NULL,
			device TEXT NOT NULL,
			path TEXT NOT NULL,
			referrer TEXT,
			screen_size TEXT,
			timestamp DATETIME NOT NULL,
			duration_sec INTEGER DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS bot_visits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bot_name TEXT NOT NULL,
			ip_hash TEXT NOT NULL,
			user_agent TEXT NOT NULL,
			path TEXT NOT NULL,
			timestamp DATETIME NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_visits_timestamp ON visits(timestamp);
		CREATE INDEX IF NOT EXISTS idx_visits_visitor_id ON visits(visitor_id);
		CREATE INDEX IF NOT EXISTS idx_visits_path ON visits(path);
		CREATE INDEX IF NOT EXISTS idx_visits_browser ON visits(browser);
		CREATE INDEX IF NOT EXISTS idx_visits_os ON visits(os);
		CREATE INDEX IF NOT EXISTS idx_visits_device ON visits(device);

		CREATE INDEX IF NOT EXISTS idx_bot_visits_timestamp ON bot_visits(timestamp);
		CREATE INDEX IF NOT EXISTS idx_bot_visits_name ON bot_visits(bot_name);

		CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`)
	return err
}

// currentSchemaVersion is the latest schema version. Increment when adding migrations.
const currentSchemaVersion = 1

// migrate applies incremental schema migrations based on a version stored in the settings table.
func (s *Store) migrate() error {
	verStr, err := s.GetSetting("schema_version")
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	version := 0
	if verStr != "" {
		version, err = strconv.Atoi(verStr)
		if err != nil {
			return fmt.Errorf("parse schema version %q: %w", verStr, err)
		}
	}

	if version < 1 {
		version = 1
	}

	return s.SetSetting("schema_version", strconv.Itoa(version))
}

// GetSetting retrieves a setting value by key. Returns empty string if not found.
func (s *Store) GetSetting(key string) (string, error) {
	val, err := s.q.GetSetting(context.Background(), key)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// SetSetting stores a setting value by key (upsert).
func (s *Store) SetSetting(key, value string) error {
	return s.q.UpsertSetting(context.Background(), key, value)
}

// SaveVisit stores a new visit in the database.
func (s *Store) SaveVisit(v *Visit) error {
	return s.q.InsertVisit(context.Background(), sqlcgen.InsertVisitParams{
		VisitorID:   v.VisitorID,
		SessionID:   v.SessionID,
		IpHash:      v.IPHash,
		Browser:     v.Browser,
		Os:          v.OS,
		Device:      v.Device,
		Path:        v.Path,
		Referrer:    sql.NullString{String: v.Referrer, Valid: true},
		ScreenSize:  sql.NullString{String: v.ScreenSize, Valid: true},
		Timestamp:   v.Timestamp.UTC(),
		DurationSec: sql.NullInt64{Int64: int64(v.DurationSec), Valid: true},
	})
}

// SaveBotVisit stores a new bot visit in the database.
func (s *Store) SaveBotVisit(bv *BotVisit) error {
	return s.q.InsertBotVisit(context.Background(), sqlcgen.InsertBotVisitParams{
		BotName:   bv.BotName,
		IpHash:    bv.IPHash,
		UserAgent: bv.UserAgent,
		Path:      bv.Path,
		Timestamp: bv.Timestamp.UTC(),
	})
}

// GetStats returns aggregated statistics for the given time period.
func (s *Store) GetStats(from, to time.Time, hourly, monthly bool) (*Stats, error) {
	ctx := context.Background()
	stats := &Stats{
		Period:        from.Format("2006-01-02") + " to " + to.Format("2006-01-02"),
		TopPages:      []PageStat{},
		LatestPages:   []LatestPageVisit{},
		BrowserStats:  []DimensionStat{},
		OSStats:       []DimensionStat{},
		DeviceStats:   []DimensionStat{},
		ReferrerStats: []DimensionStat{},
		DailyViews:    []DailyView{},
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error

	setErr := func(err error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
	}

	// Total views
	wg.Add(1)
	go func() {
		defer wg.Done()
		count, err := s.q.CountVisits(ctx, from, to)
		if err != nil {
			setErr(fmt.Errorf("count views: %w", err))
			return
		}
		mu.Lock()
		stats.TotalViews = int(count)
		mu.Unlock()
	}()

	// Unique visitors
	wg.Add(1)
	go func() {
		defer wg.Done()
		count, err := s.q.CountUniqueVisitors(ctx, from, to)
		if err != nil {
			setErr(fmt.Errorf("count unique visitors: %w", err))
			return
		}
		mu.Lock()
		stats.UniqueVisitors = int(count)
		mu.Unlock()
	}()

	// Average duration
	wg.Add(1)
	go func() {
		defer wg.Done()
		avg, err := s.q.AvgDuration(ctx, from, to)
		if err != nil {
			setErr(fmt.Errorf("avg duration: %w", err))
			return
		}
		if avg.Valid {
			mu.Lock()
			stats.AvgDuration = int(avg.Float64)
			mu.Unlock()
		}
	}()

	// Top pages
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := s.q.TopPages(ctx, from, to)
		if err != nil {
			setErr(fmt.Errorf("top pages: %w", err))
			return
		}
		pages := make([]PageStat, len(rows))
		for i, r := range rows {
			pages[i] = PageStat{Path: r.Path, Views: int(r.Views)}
		}
		mu.Lock()
		stats.TopPages = pages
		mu.Unlock()
	}()

	// Latest pages
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := s.q.LatestPages(ctx, from, to)
		if err != nil {
			setErr(fmt.Errorf("latest pages: %w", err))
			return
		}
		latest := make([]LatestPageVisit, len(rows))
		for i, r := range rows {
			latest[i] = LatestPageVisit{
				Path:      r.Path,
				Timestamp: r.Timestamp.Format("2006-01-02 15:04:05"),
				Browser:   r.Browser,
			}
		}
		mu.Lock()
		stats.LatestPages = latest
		mu.Unlock()
	}()

	// Browser stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := s.q.BrowserStats(ctx, from, to)
		if err != nil {
			setErr(fmt.Errorf("browser stats: %w", err))
			return
		}
		result := make([]DimensionStat, len(rows))
		for i, r := range rows {
			result[i] = DimensionStat{Name: r.Name, Count: int(r.Count)}
		}
		mu.Lock()
		stats.BrowserStats = result
		mu.Unlock()
	}()

	// OS stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := s.q.OSStats(ctx, from, to)
		if err != nil {
			setErr(fmt.Errorf("os stats: %w", err))
			return
		}
		result := make([]DimensionStat, len(rows))
		for i, r := range rows {
			result[i] = DimensionStat{Name: r.Name, Count: int(r.Count)}
		}
		mu.Lock()
		stats.OSStats = result
		mu.Unlock()
	}()

	// Device stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := s.q.DeviceStats(ctx, from, to)
		if err != nil {
			setErr(fmt.Errorf("device stats: %w", err))
			return
		}
		result := make([]DimensionStat, len(rows))
		for i, r := range rows {
			result[i] = DimensionStat{Name: r.Name, Count: int(r.Count)}
		}
		mu.Lock()
		stats.DeviceStats = result
		mu.Unlock()
	}()

	// Referrer stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		rows, err := s.q.ReferrerStats(ctx, from, to)
		if err != nil {
			setErr(fmt.Errorf("referrer stats: %w", err))
			return
		}
		result := make([]DimensionStat, len(rows))
		for i, r := range rows {
			result[i] = DimensionStat{Name: r.Name, Count: int(r.Count)}
		}
		mu.Lock()
		stats.ReferrerStats = result
		mu.Unlock()
	}()

	// Daily/hourly/monthly views
	wg.Add(1)
	go func() {
		defer wg.Done()
		var result []DailyView
		if hourly {
			rows, err := s.q.HourlyViews(ctx, from, to)
			if err != nil {
				setErr(fmt.Errorf("hourly views: %w", err))
				return
			}
			result = make([]DailyView, len(rows))
			for i, r := range rows {
				result[i] = DailyView{Date: r.Date, Views: int(r.Views)}
			}
		} else if monthly {
			rows, err := s.q.MonthlyViews(ctx, from, to)
			if err != nil {
				setErr(fmt.Errorf("monthly views: %w", err))
				return
			}
			result = make([]DailyView, len(rows))
			for i, r := range rows {
				result[i] = DailyView{Date: r.Date, Views: int(r.Views)}
			}
		} else {
			rows, err := s.q.DailyViews(ctx, from, to)
			if err != nil {
				setErr(fmt.Errorf("daily views: %w", err))
				return
			}
			result = make([]DailyView, len(rows))
			for i, r := range rows {
				result[i] = DailyView{Date: r.Date, Views: int(r.Views)}
			}
		}
		mu.Lock()
		stats.DailyViews = result
		mu.Unlock()
	}()

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return stats, nil
}

// GetBotStats returns aggregated bot statistics for the given time period.
func (s *Store) GetBotStats(from, to time.Time, hourly, monthly bool) (*BotStats, error) {
	ctx := context.Background()
	stats := &BotStats{
		Period:      from.Format("2006-01-02") + " to " + to.Format("2006-01-02"),
		TopBots:     []DimensionStat{},
		TopPages:    []PageStat{},
		DailyVisits: []DailyView{},
	}

	// Total bot visits
	count, err := s.q.CountBotVisits(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("count bot visits: %w", err)
	}
	stats.TotalVisits = int(count)

	// Top bots
	topBots, err := s.q.TopBots(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("top bots: %w", err)
	}
	for _, r := range topBots {
		stats.TopBots = append(stats.TopBots, DimensionStat{Name: r.Name, Count: int(r.Count)})
	}

	// Top pages
	topPages, err := s.q.TopBotPages(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("top bot pages: %w", err)
	}
	for _, r := range topPages {
		stats.TopPages = append(stats.TopPages, PageStat{Path: r.Path, Views: int(r.Views)})
	}

	// Daily/hourly/monthly bot visits
	if hourly {
		rows, err := s.q.HourlyBotVisits(ctx, from, to)
		if err != nil {
			return nil, fmt.Errorf("bot views: %w", err)
		}
		for _, r := range rows {
			stats.DailyVisits = append(stats.DailyVisits, DailyView{Date: r.Date, Views: int(r.Views)})
		}
	} else if monthly {
		rows, err := s.q.MonthlyBotVisits(ctx, from, to)
		if err != nil {
			return nil, fmt.Errorf("bot views: %w", err)
		}
		for _, r := range rows {
			stats.DailyVisits = append(stats.DailyVisits, DailyView{Date: r.Date, Views: int(r.Views)})
		}
	} else {
		rows, err := s.q.DailyBotVisits(ctx, from, to)
		if err != nil {
			return nil, fmt.Errorf("bot views: %w", err)
		}
		for _, r := range rows {
			stats.DailyVisits = append(stats.DailyVisits, DailyView{Date: r.Date, Views: int(r.Views)})
		}
	}

	return stats, nil
}

// CleanupOldVisits removes visits and bot visits older than the retention period.
func (s *Store) CleanupOldVisits(retentionDays int) error {
	ctx := context.Background()
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	if err := s.q.DeleteOldVisits(ctx, cutoff); err != nil {
		return fmt.Errorf("cleanup visits: %w", err)
	}
	if err := s.q.DeleteOldBotVisits(ctx, cutoff); err != nil {
		return fmt.Errorf("cleanup bot_visits: %w", err)
	}
	return nil
}

// StartCleanupScheduler runs periodic cleanup of old data. Returns a stop function.
func (s *Store) StartCleanupScheduler(retentionDays int, interval time.Duration) func() {
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				if err := s.CleanupOldVisits(retentionDays); err != nil {
					fmt.Printf("cleanup error: %v\n", err)
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	return func() { close(done) }
}

// GetRealtimeVisitors returns the number of unique visitors in the last 5 minutes.
func (s *Store) GetRealtimeVisitors() (int, error) {
	cutoff := time.Now().UTC().Add(-5 * time.Minute)
	count, err := s.q.CountRealtimeVisitors(context.Background(), cutoff)
	return int(count), err
}
