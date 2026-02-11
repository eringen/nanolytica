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
