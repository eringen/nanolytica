// Package templates contains view model types for templating.
// These types mirror the analytics types to avoid import cycles.
package templates

// StatsViewModel represents analytics statistics for templating.
type StatsViewModel struct {
	Period         string
	UniqueVisitors int
	TotalViews     int
	AvgDuration    int
	TopPages       []PageStatViewModel
	LatestPages    []LatestPageVisitViewModel
	BrowserStats   []DimensionStatViewModel
	OSStats        []DimensionStatViewModel
	DeviceStats    []DimensionStatViewModel
	ReferrerStats  []DimensionStatViewModel
	DailyViews     []DailyViewViewModel
}

// BotStatsViewModel represents bot analytics statistics for templating.
type BotStatsViewModel struct {
	Period      string
	TotalVisits int
	TopBots     []DimensionStatViewModel
	TopPages    []PageStatViewModel
	DailyVisits []DailyViewViewModel
}

// PageStatViewModel represents page view statistics.
type PageStatViewModel struct {
	Path  string
	Views int
}

// LatestPageVisitViewModel represents a single recent page visit.
type LatestPageVisitViewModel struct {
	Path      string
	Timestamp string
	Browser   string
}

// DimensionStatViewModel represents a dimension breakdown.
type DimensionStatViewModel struct {
	Name  string
	Count int
}

// DailyViewViewModel represents views per day.
type DailyViewViewModel struct {
	Date  string
	Views int
}
