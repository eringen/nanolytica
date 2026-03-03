package analytics

import (
	"strings"
	"testing"
)

func TestValidateCollectRequest_Valid(t *testing.T) {
	req := &CollectRequest{
		Path:        "/blog/hello-world",
		Referrer:    "https://google.com",
		ScreenSize:  "1920x1080",
		UserAgent:   "Mozilla/5.0",
		DurationSec: 120,
	}
	if err := validateCollectRequest(req); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestValidateCollectRequest_EmptyFields(t *testing.T) {
	req := &CollectRequest{
		Path:        "",
		Referrer:    "",
		ScreenSize:  "",
		UserAgent:   "",
		DurationSec: 0,
	}
	if err := validateCollectRequest(req); err != nil {
		t.Errorf("expected nil error for empty fields, got: %v", err)
	}
}

func TestValidateCollectRequest_PathTooLong(t *testing.T) {
	req := &CollectRequest{
		Path: strings.Repeat("a", maxPathLen+1),
	}
	if err := validateCollectRequest(req); err == nil {
		t.Error("expected error for path exceeding max length")
	}
}

func TestValidateCollectRequest_PathValid(t *testing.T) {
	valid := []string{
		"", "/", "/blog/hello-world", "/path/to/page",
		"/search?q=test&page=1", "/café", "/日本語",
		"/path/with spaces", "/path%20encoded",
		"/blog/hello-world#section", "/path/with~tilde",
		"/path/with.dots/file.html",
	}
	for _, p := range valid {
		req := &CollectRequest{Path: p}
		if err := validateCollectRequest(req); err != nil {
			t.Errorf("expected path %q to be valid, got: %v", p, err)
		}
	}
}

func TestValidateCollectRequest_PathInvalid(t *testing.T) {
	invalid := []string{
		"/path\x00with-null",
		"/path\x01control",
		"/path\x1fchar",
		"/path\x7fdelete",
		"hello\ttab",
		"hello\nnewline",
		"hello\rreturn",
	}
	for _, p := range invalid {
		req := &CollectRequest{Path: p}
		if err := validateCollectRequest(req); err == nil {
			t.Errorf("expected path %q to be invalid", p)
		}
	}
}

func TestValidateCollectRequest_ReferrerTooLong(t *testing.T) {
	req := &CollectRequest{
		Path:     "/",
		Referrer: strings.Repeat("a", maxReferrerLen+1),
	}
	if err := validateCollectRequest(req); err == nil {
		t.Error("expected error for referrer exceeding max length")
	}
}

func TestValidateCollectRequest_ScreenSizeTooLong(t *testing.T) {
	req := &CollectRequest{
		Path:       "/",
		ScreenSize: strings.Repeat("x", maxScreenSizeLen+1),
	}
	if err := validateCollectRequest(req); err == nil {
		t.Error("expected error for screen_size exceeding max length")
	}
}

func TestValidateCollectRequest_UserAgentTooLong(t *testing.T) {
	req := &CollectRequest{
		Path:      "/",
		UserAgent: strings.Repeat("u", maxUserAgentLen+1),
	}
	if err := validateCollectRequest(req); err == nil {
		t.Error("expected error for user_agent exceeding max length")
	}
}

func TestValidateCollectRequest_ScreenSizeValid(t *testing.T) {
	valid := []string{"", "1920x1080", "360x640", "1x1", "99999x99999"}
	for _, ss := range valid {
		req := &CollectRequest{Path: "/", ScreenSize: ss}
		if err := validateCollectRequest(req); err != nil {
			t.Errorf("expected %q to be valid, got: %v", ss, err)
		}
	}
}

func TestValidateCollectRequest_ScreenSizeInvalid(t *testing.T) {
	invalid := []string{"garbage", "x", "1920", "1920x", "x1080", "abc x def", "1920X1080", "1920 x 1080", "-1x1080"}
	for _, ss := range invalid {
		req := &CollectRequest{Path: "/", ScreenSize: ss}
		if err := validateCollectRequest(req); err == nil {
			t.Errorf("expected %q to be invalid screen_size", ss)
		}
	}
}

func TestValidateCollectRequest_NegativeDuration(t *testing.T) {
	req := &CollectRequest{
		Path:        "/",
		DurationSec: -1,
	}
	if err := validateCollectRequest(req); err == nil {
		t.Error("expected error for negative duration_sec")
	}
}

func TestValidateCollectRequest_DurationTooLarge(t *testing.T) {
	req := &CollectRequest{
		Path:        "/",
		DurationSec: maxDurationSec + 1,
	}
	if err := validateCollectRequest(req); err == nil {
		t.Error("expected error for duration_sec exceeding maximum")
	}
}
