package analytics

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var siteNameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]{0,62}[a-zA-Z0-9])?$`)

// SiteRegistry manages per-site Store instances.
// Each site has its own SQLite database and salt.
type SiteRegistry struct {
	mu        sync.RWMutex
	stores    map[string]*Store
	sites     []string
	dataDir   string
	defaultDB string
	cfg       StoreConfig
}

// NewSiteRegistry creates a registry, opens the default store, and auto-discovers
// additional sites from existing .db files in the data directory.
// defaultDBPath is the full path to the default site's database (e.g., "data/nanolytica.db").
func NewSiteRegistry(defaultDBPath string, cfg StoreConfig) (*SiteRegistry, error) {
	dataDir := filepath.Dir(defaultDBPath)
	defaultBase := filepath.Base(defaultDBPath)
	r := &SiteRegistry{
		stores:    make(map[string]*Store),
		dataDir:   dataDir,
		defaultDB: defaultDBPath,
		cfg:       cfg,
	}

	// Always open the default store
	store, err := NewStoreWithConfig(defaultDBPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("open default store: %w", err)
	}
	r.stores["default"] = store
	r.sites = []string{"default"}

	// Auto-discover additional sites from existing .db files
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf("read data directory: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".db") {
			continue
		}
		// Skip the default db, WAL files, and journal files
		if name == defaultBase || strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-shm") || strings.HasSuffix(name, "-journal") {
			continue
		}
		site := strings.TrimSuffix(name, ".db")
		if !ValidateSiteName(site) {
			continue
		}
		dbPath := filepath.Join(dataDir, name)
		s, err := NewStoreWithConfig(dbPath, cfg)
		if err != nil {
			r.Close()
			return nil, fmt.Errorf("open store for site %q: %w", site, err)
		}
		r.stores[site] = s
		r.sites = append(r.sites, site)
	}

	sort.Strings(r.sites)
	return r, nil
}

// ValidateSiteName checks if a site name is valid.
// Allows alphanumeric, dots, hyphens, underscores. Max 64 chars.
func ValidateSiteName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	return siteNameRegex.MatchString(name)
}

// GetStore returns the Store for the given site name. Returns nil if site is unknown.
func (r *SiteRegistry) GetStore(site string) *Store {
	if site == "" {
		site = "default"
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stores[site]
}

// DefaultStore returns the default site's Store.
func (r *SiteRegistry) DefaultStore() *Store {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stores["default"]
}

// ListSites returns the sorted list of site names.
func (r *SiteRegistry) ListSites() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, len(r.sites))
	copy(result, r.sites)
	return result
}

// AddSite creates a new site store at runtime. Returns error if name is invalid or already exists.
func (r *SiteRegistry) AddSite(name string) error {
	if !ValidateSiteName(name) {
		return fmt.Errorf("invalid site name: %q", name)
	}
	if name == "default" {
		return fmt.Errorf("cannot add reserved site name %q", name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.stores[name]; exists {
		return fmt.Errorf("site %q already exists", name)
	}

	dbPath := filepath.Join(r.dataDir, name+".db")
	s, err := NewStoreWithConfig(dbPath, r.cfg)
	if err != nil {
		return fmt.Errorf("open store for site %q: %w", name, err)
	}
	r.stores[name] = s
	r.sites = append(r.sites, name)
	sort.Strings(r.sites)

	// Start cleanup scheduler for the new site
	s.StartCleanupScheduler(365, 24*time.Hour)

	return nil
}

// DeleteSite closes and removes a site's store and deletes its database files.
func (r *SiteRegistry) DeleteSite(name string) error {
	if name == "" || name == "default" {
		return fmt.Errorf("cannot delete the default site")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	s, exists := r.stores[name]
	if !exists {
		return fmt.Errorf("site %q not found", name)
	}

	s.Close()
	delete(r.stores, name)

	// Rebuild sites slice
	sites := make([]string, 0, len(r.stores))
	for k := range r.stores {
		sites = append(sites, k)
	}
	sort.Strings(sites)
	r.sites = sites

	// Delete database files
	dbPath := filepath.Join(r.dataDir, name+".db")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		os.Remove(dbPath + suffix)
	}

	return nil
}

// Close closes all stores.
func (r *SiteRegistry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.stores {
		s.Close()
	}
}

// StartCleanupSchedulers starts cleanup schedulers for all stores.
// Returns a stop function that stops all schedulers.
func (r *SiteRegistry) StartCleanupSchedulers(retentionDays int, interval time.Duration) func() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var stops []func()
	for _, s := range r.stores {
		stops = append(stops, s.StartCleanupScheduler(retentionDays, interval))
	}
	return func() {
		for _, stop := range stops {
			stop()
		}
	}
}
