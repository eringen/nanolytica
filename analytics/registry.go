package analytics

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
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

// NewSiteRegistry creates a registry and opens stores for all configured sites.
// defaultDBPath is the full path to the default site's database (e.g., "data/nanolytica.db").
// sites is the list of additional site names. The "default" site is always available.
func NewSiteRegistry(defaultDBPath string, sites []string, cfg StoreConfig) (*SiteRegistry, error) {
	dataDir := filepath.Dir(defaultDBPath)
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

	// Open stores for additional sites
	for _, site := range sites {
		if site == "" || site == "default" {
			continue
		}
		if !ValidateSiteName(site) {
			r.Close()
			return nil, fmt.Errorf("invalid site name: %q", site)
		}
		dbPath := filepath.Join(dataDir, site+".db")
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
