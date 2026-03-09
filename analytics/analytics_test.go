package analytics

import (
	"os"
	"testing"
)

const testSalt = "test-salt-for-unit-tests"

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// --- ParseUserAgent tests ---

func TestParseUserAgent_Chrome(t *testing.T) {
	browser, os, device := ParseUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if browser != "Chrome" {
		t.Errorf("expected Chrome, got %s", browser)
	}
	if os != "Windows" {
		t.Errorf("expected Windows, got %s", os)
	}
	if device != "Desktop" {
		t.Errorf("expected Desktop, got %s", device)
	}
}

func TestParseUserAgent_Firefox(t *testing.T) {
	browser, _, _ := ParseUserAgent("Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/121.0")
	if browser != "Firefox" {
		t.Errorf("expected Firefox, got %s", browser)
	}
}

func TestParseUserAgent_Safari(t *testing.T) {
	browser, os, _ := ParseUserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15")
	if browser != "Safari" {
		t.Errorf("expected Safari, got %s", browser)
	}
	if os != "macOS" {
		t.Errorf("expected macOS, got %s", os)
	}
}

func TestParseUserAgent_Edge(t *testing.T) {
	browser, _, _ := ParseUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0")
	if browser != "Edge" {
		t.Errorf("expected Edge, got %s", browser)
	}
}

func TestParseUserAgent_Opera(t *testing.T) {
	browser, _, _ := ParseUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OPR/106.0.0.0")
	if browser != "Opera" {
		t.Errorf("expected Opera, got %s", browser)
	}
}

func TestParseUserAgent_Android(t *testing.T) {
	_, os, device := ParseUserAgent("Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36")
	if os != "Android" {
		t.Errorf("expected Android, got %s", os)
	}
	if device != "Mobile" {
		t.Errorf("expected Mobile, got %s", device)
	}
}

func TestParseUserAgent_iOS(t *testing.T) {
	_, os, device := ParseUserAgent("Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1")
	if os != "iOS" {
		t.Errorf("expected iOS, got %s", os)
	}
	if device != "Mobile" {
		t.Errorf("expected Mobile, got %s", device)
	}
}

func TestParseUserAgent_iPad(t *testing.T) {
	_, os, device := ParseUserAgent("Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1")
	if os != "iOS" {
		t.Errorf("expected iOS, got %s", os)
	}
	if device != "Tablet" {
		t.Errorf("expected Tablet, got %s", device)
	}
}

func TestParseUserAgent_Linux(t *testing.T) {
	_, os, _ := ParseUserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if os != "Linux" {
		t.Errorf("expected Linux, got %s", os)
	}
}

// --- IsBot tests ---

func TestIsBot_Googlebot(t *testing.T) {
	if !IsBot("Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)") {
		t.Error("expected Googlebot to be detected as bot")
	}
}

func TestIsBot_Bingbot(t *testing.T) {
	if !IsBot("Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)") {
		t.Error("expected Bingbot to be detected as bot")
	}
}

func TestIsBot_NotABot(t *testing.T) {
	if IsBot("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36") {
		t.Error("expected normal Chrome UA to not be a bot")
	}
}

func TestIsBot_HTTPClients(t *testing.T) {
	clients := []string{
		"curl/7.88.1",
		"Wget/1.21",
		"python-requests/2.31.0",
		"Python-urllib/3.11",
		"Go-http-client/2.0",
		"node-fetch/3.3.0",
		"axios/1.6.0",
		"Java/17.0.1",
		"okhttp/4.12.0",
		"libwww-perl/6.67",
		"PostmanRuntime/7.36",
	}
	for _, ua := range clients {
		if !IsBot(ua) {
			t.Errorf("expected %q to be detected as bot", ua)
		}
	}
}

func TestIsBot_HeadlessBrowsers(t *testing.T) {
	headless := []string{
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Unknown; Linux x86_64) AppleWebKit/538.1 (KHTML, like Gecko) PhantomJS/2.1.1 Safari/538.1",
	}
	for _, ua := range headless {
		if !IsBot(ua) {
			t.Errorf("expected %q to be detected as bot", ua)
		}
	}
}

func TestIsBot_AICrawlers(t *testing.T) {
	aiCrawlers := []string{
		"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; GPTBot/1.0; +https://openai.com/gptbot)",
		"ChatGPT-User/1.0",
		"ClaudeBot/1.0",
		"CCBot/2.0 (https://commoncrawl.org/faq/)",
		"Mozilla/5.0 (compatible; Bytespider; spider-feedback@bytedance.com)",
	}
	for _, ua := range aiCrawlers {
		if !IsBot(ua) {
			t.Errorf("expected %q to be detected as bot", ua)
		}
	}
}

func TestIsBot_Monitoring(t *testing.T) {
	monitors := []string{
		"Mozilla/5.0 (compatible; UptimeRobot/2.0; http://www.uptimerobot.com/)",
		"Pingdom.com_bot_version_1.4_(http://www.pingdom.com/)",
	}
	for _, ua := range monitors {
		if !IsBot(ua) {
			t.Errorf("expected %q to be detected as bot", ua)
		}
	}
}

func TestIsBot_RealBrowsersNotFlagged(t *testing.T) {
	realBrowsers := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
	}
	for _, ua := range realBrowsers {
		if IsBot(ua) {
			t.Errorf("expected %q to NOT be detected as bot", ua)
		}
	}
}

// --- ExtractBotName tests ---

func TestExtractBotName_Known(t *testing.T) {
	tests := map[string]string{
		"Mozilla/5.0 (compatible; Googlebot/2.1)":   "Googlebot",
		"Mozilla/5.0 (compatible; bingbot/2.0)":     "Bingbot",
		"Mozilla/5.0 (compatible; AhrefsBot/7.0)":   "Ahrefs",
		"facebookexternalhit/1.1":                    "Facebook",
		"Mozilla/5.0 (compatible; SemrushBot/7~bl)": "SEMrush",
	}
	for ua, expected := range tests {
		got := ExtractBotName(ua)
		if got != expected {
			t.Errorf("ExtractBotName(%q) = %q, want %q", ua, got, expected)
		}
	}
}

func TestExtractBotName_NewBots(t *testing.T) {
	tests := []struct {
		ua   string
		name string
	}{
		{"curl/7.88.1", "curl"},
		{"python-requests/2.31.0", "Python Requests"},
		{"Go-http-client/2.0", "Go HTTP"},
		{"Mozilla/5.0 (X11; Linux x86_64) HeadlessChrome/120.0.0.0", "Headless Chrome"},
		{"Mozilla/5.0 AppleWebKit/537.36 (compatible; GPTBot/1.0)", "GPTBot"},
		{"ClaudeBot/1.0", "ClaudeBot"},
		{"CCBot/2.0", "Common Crawl"},
		{"Mozilla/5.0 (compatible; UptimeRobot/2.0)", "UptimeRobot"},
		{"PostmanRuntime/7.36.0", "Postman"},
		{"Wget/1.21", "wget"},
	}
	for _, tt := range tests {
		got := ExtractBotName(tt.ua)
		if got != tt.name {
			t.Errorf("ExtractBotName(%q) = %q, want %q", tt.ua, got, tt.name)
		}
	}
}

func TestExtractBotName_GenericBot(t *testing.T) {
	name := ExtractBotName("SomeRandomBot/1.0")
	if name != "Other Bot" {
		t.Errorf("expected 'Other Bot', got %s", name)
	}
}

// --- CleanReferrer tests ---

func TestCleanReferrer_Empty(t *testing.T) {
	if got := CleanReferrer(""); got != "Direct" {
		t.Errorf("expected Direct, got %s", got)
	}
}

func TestCleanReferrer_Google(t *testing.T) {
	if got := CleanReferrer("https://www.google.com/search?q=test"); got != "Google" {
		t.Errorf("expected Google, got %s", got)
	}
}

func TestCleanReferrer_Domain(t *testing.T) {
	if got := CleanReferrer("https://example.com/page"); got != "example.com" {
		t.Errorf("expected example.com, got %s", got)
	}
}

func TestCleanReferrer_DomainWithWWW(t *testing.T) {
	if got := CleanReferrer("https://www.example.com/page"); got != "example.com" {
		t.Errorf("expected example.com, got %s", got)
	}
}

// --- HashIP tests ---

func TestHashIP_Deterministic(t *testing.T) {
	h1 := HashIP(testSalt, "192.168.1.1")
	h2 := HashIP(testSalt, "192.168.1.1")
	if h1 != h2 {
		t.Errorf("same IP should produce same hash, got %s and %s", h1, h2)
	}
}

func TestHashIP_DifferentIPs(t *testing.T) {
	h1 := HashIP(testSalt, "192.168.1.1")
	h2 := HashIP(testSalt, "10.0.0.1")
	if h1 == h2 {
		t.Errorf("different IPs should produce different hashes")
	}
}

func TestHashIP_DifferentSalts(t *testing.T) {
	h1 := HashIP("salt-a", "192.168.1.1")
	h2 := HashIP("salt-b", "192.168.1.1")
	if h1 == h2 {
		t.Errorf("different salts should produce different hashes")
	}
}

func TestHashIP_Length(t *testing.T) {
	h := HashIP(testSalt, "192.168.1.1")
	if len(h) != 16 {
		t.Errorf("expected hash length 16, got %d", len(h))
	}
}

// --- GenerateVisitorID tests ---

func TestGenerateVisitorID_Deterministic(t *testing.T) {
	v1 := GenerateVisitorID(testSalt, "192.168.1.1", "Chrome")
	v2 := GenerateVisitorID(testSalt, "192.168.1.1", "Chrome")
	if v1 != v2 {
		t.Errorf("same inputs should produce same visitor ID")
	}
}

func TestGenerateVisitorID_DifferentUA(t *testing.T) {
	v1 := GenerateVisitorID(testSalt, "192.168.1.1", "Chrome")
	v2 := GenerateVisitorID(testSalt, "192.168.1.1", "Firefox")
	if v1 == v2 {
		t.Errorf("different UAs should produce different visitor IDs")
	}
}

// --- ValidateSiteName tests ---

func TestValidateSiteName(t *testing.T) {
	valid := []string{"default", "mysite", "blog.example.com", "my-site_1", "a", "A1"}
	for _, name := range valid {
		if !ValidateSiteName(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalid := []string{"", "../escape", "a/b", "has spaces", string(make([]byte, 65))}
	for _, name := range invalid {
		if ValidateSiteName(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
