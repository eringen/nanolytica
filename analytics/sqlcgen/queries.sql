-- Settings

-- name: GetSetting :one
SELECT value FROM settings WHERE key = ?;

-- name: UpsertSetting :exec
INSERT INTO settings (key, value) VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;

-- Inserts

-- name: InsertVisit :exec
INSERT INTO visits (visitor_id, session_id, ip_hash, browser, os, device, path, referrer, screen_size, timestamp, duration_sec)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertBotVisit :exec
INSERT INTO bot_visits (bot_name, ip_hash, user_agent, path, timestamp)
VALUES (?, ?, ?, ?, ?);

-- Visitor aggregations

-- name: CountVisits :one
SELECT COUNT(*) FROM visits WHERE timestamp >= ? AND timestamp < ?;

-- name: CountUniqueVisitors :one
SELECT COUNT(DISTINCT visitor_id) FROM visits WHERE timestamp >= ? AND timestamp < ?;

-- name: AvgDuration :one
SELECT AVG(duration_sec) FROM visits WHERE timestamp >= ? AND timestamp < ? AND duration_sec > 0;

-- name: TopPages :many
SELECT path, COUNT(*) AS views
FROM visits
WHERE timestamp >= ? AND timestamp < ?
GROUP BY path
ORDER BY views DESC
LIMIT 10;

-- name: LatestPages :many
SELECT path, timestamp, browser
FROM visits
WHERE timestamp >= ? AND timestamp < ?
ORDER BY timestamp DESC
LIMIT 5;

-- name: BrowserStats :many
SELECT browser AS name, COUNT(*) AS count
FROM visits
WHERE timestamp >= ? AND timestamp < ?
GROUP BY browser
ORDER BY count DESC;

-- name: OSStats :many
SELECT os AS name, COUNT(*) AS count
FROM visits
WHERE timestamp >= ? AND timestamp < ?
GROUP BY os
ORDER BY count DESC;

-- name: DeviceStats :many
SELECT device AS name, COUNT(*) AS count
FROM visits
WHERE timestamp >= ? AND timestamp < ?
GROUP BY device
ORDER BY count DESC;

-- name: ReferrerStats :many
SELECT
    CASE
        WHEN referrer = '' OR referrer IS NULL THEN 'Direct'
        WHEN referrer LIKE '%google.%' THEN 'Google'
        WHEN referrer LIKE '%bing.%' THEN 'Bing'
        WHEN referrer LIKE '%duckduckgo.%' THEN 'DuckDuckGo'
        WHEN referrer LIKE '%yahoo.%' THEN 'Yahoo'
        WHEN referrer LIKE '%github.%' THEN 'GitHub'
        ELSE 'Other'
    END AS name,
    COUNT(*) AS count
FROM visits
WHERE timestamp >= ? AND timestamp < ?
GROUP BY 1
ORDER BY count DESC;

-- name: DailyViews :many
SELECT CAST(substr(timestamp, 1, 10) AS TEXT) AS date, COUNT(*) AS views
FROM visits
WHERE timestamp >= ? AND timestamp < ?
GROUP BY 1
ORDER BY date;

-- name: HourlyViews :many
SELECT CAST(substr(timestamp, 12, 2) || ':00' AS TEXT) AS date, COUNT(*) AS views
FROM visits
WHERE timestamp >= ? AND timestamp < ?
GROUP BY 1
ORDER BY date;

-- name: MonthlyViews :many
SELECT CAST(substr(timestamp, 1, 7) AS TEXT) AS date, COUNT(*) AS views
FROM visits
WHERE timestamp >= ? AND timestamp < ?
GROUP BY 1
ORDER BY date;

-- Bot aggregations

-- name: CountBotVisits :one
SELECT COUNT(*) FROM bot_visits WHERE timestamp >= ? AND timestamp < ?;

-- name: TopBots :many
SELECT bot_name AS name, COUNT(*) AS count
FROM bot_visits
WHERE timestamp >= ? AND timestamp < ?
GROUP BY bot_name
ORDER BY count DESC
LIMIT 10;

-- name: TopBotPages :many
SELECT path, COUNT(*) AS views
FROM bot_visits
WHERE timestamp >= ? AND timestamp < ?
GROUP BY path
ORDER BY views DESC
LIMIT 10;

-- name: DailyBotVisits :many
SELECT CAST(substr(timestamp, 1, 10) AS TEXT) AS date, COUNT(*) AS views
FROM bot_visits
WHERE timestamp >= ? AND timestamp < ?
GROUP BY 1
ORDER BY date;

-- name: HourlyBotVisits :many
SELECT CAST(substr(timestamp, 12, 2) || ':00' AS TEXT) AS date, COUNT(*) AS views
FROM bot_visits
WHERE timestamp >= ? AND timestamp < ?
GROUP BY 1
ORDER BY date;

-- name: MonthlyBotVisits :many
SELECT CAST(substr(timestamp, 1, 7) AS TEXT) AS date, COUNT(*) AS views
FROM bot_visits
WHERE timestamp >= ? AND timestamp < ?
GROUP BY 1
ORDER BY date;

-- Cleanup

-- name: DeleteOldVisits :exec
DELETE FROM visits WHERE timestamp < ?;

-- name: DeleteOldBotVisits :exec
DELETE FROM bot_visits WHERE timestamp < ?;

-- Realtime

-- name: CountRealtimeVisitors :one
SELECT COUNT(DISTINCT visitor_id) FROM visits WHERE timestamp >= ?;
