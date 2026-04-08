package qobuz

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestComputeFileURLSignature(t *testing.T) {
	// Known inputs and expected output
	sig := computeFileURLSignature(
		"fileUrl",
		"33967376",
		"7",
		"stream",
		"1700000000",
		"mysecret",
	)

	// The signature should be a 32-char hex MD5 hash
	if len(sig) != 32 {
		t.Errorf("signature length = %d, want 32", len(sig))
	}

	// Verify deterministic: same inputs = same output
	sig2 := computeFileURLSignature(
		"fileUrl",
		"33967376",
		"7",
		"stream",
		"1700000000",
		"mysecret",
	)
	if sig != sig2 {
		t.Errorf("signature not deterministic: %q != %q", sig, sig2)
	}

	// Different inputs should produce different signature
	sig3 := computeFileURLSignature(
		"fileUrl",
		"99999999",
		"7",
		"stream",
		"1700000000",
		"mysecret",
	)
	if sig == sig3 {
		t.Error("different inputs should produce different signature")
	}
}

func TestComputeSessionSignature(t *testing.T) {
	sig := computeSessionSignature("1700000000", "mysecret")

	if len(sig) != 32 {
		t.Errorf("signature length = %d, want 32", len(sig))
	}

	// Deterministic
	sig2 := computeSessionSignature("1700000000", "mysecret")
	if sig != sig2 {
		t.Errorf("signature not deterministic: %q != %q", sig, sig2)
	}
}

func TestStreamingGetFileURL(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/file/url" {
			t.Errorf("path = %q, want /file/url", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("method = %q, want GET", r.Method)
		}

		// Verify required signed params are present
		q := r.URL.Query()
		if q.Get("track_id") != "33967376" {
			t.Errorf("track_id = %q, want 33967376", q.Get("track_id"))
		}
		if q.Get("format_id") != "7" {
			t.Errorf("format_id = %q, want 7 (quality 3)", q.Get("format_id"))
		}
		if q.Get("intent") != "stream" {
			t.Errorf("intent = %q, want stream", q.Get("intent"))
		}
		if q.Get("request_ts") == "" {
			t.Error("request_ts should not be empty")
		}
		if q.Get("request_sig") == "" {
			t.Error("request_sig should not be empty")
		}

		resp := map[string]any{
			"track_id":     33967376,
			"format_id":    7,
			"mime_type":    "audio/mp4",
			"sampling_rate": 96000,
			"bits_depth":   24,
			"duration":     133.29,
			"url_template":  "https://streaming.example.com/$SEGMENT$",
			"n_segments":   14,
			"key_id":       "key-123",
			"key":          "qbz-1.xxx",
			"blob":         "opaque",
			"restrictions": []any{},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	fu, err := client.Streaming.GetFileURL(context.Background(), 33967376, 3)
	if err != nil {
		t.Fatalf("GetFileURL: %v", err)
	}

	if fu.TrackID != 33967376 {
		t.Errorf("TrackID = %d, want 33967376", fu.TrackID)
	}
	if fu.FormatID != 7 {
		t.Errorf("FormatID = %d, want 7", fu.FormatID)
	}
	if fu.NSegments != 14 {
		t.Errorf("NSegments = %d, want 14", fu.NSegments)
	}
}

func TestStreamingGetFileURLNoSecret(t *testing.T) {
	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	defer server.Close()

	// Clear the secret
	client.Streaming.appSecret = ""

	_, err := client.Streaming.GetFileURL(context.Background(), 123, 3)
	if err == nil {
		t.Fatal("expected error when app_secret is empty")
	}
	if err.Error() != "app_secret is required for GetFileURL" {
		t.Errorf("error = %q, want app_secret message", err.Error())
	}
}

func TestStreamingStartSession(t *testing.T) {
	var capturedProfile string
	var capturedTS string
	var capturedSig string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/session/start" {
			t.Errorf("path = %q, want /session/start", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}

		r.ParseForm()
		capturedProfile = r.PostForm.Get("profile")
		capturedTS = r.PostForm.Get("request_ts")
		capturedSig = r.PostForm.Get("request_sig")

		resp := map[string]any{
			"session_id": "sess-abc-123",
			"profile":    "qbz-1",
			"expires_at": 1775700000,
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	session, err := client.Streaming.StartSession(context.Background())
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	if capturedProfile != "qbz-1" {
		t.Errorf("profile = %q, want qbz-1", capturedProfile)
	}
	if capturedTS == "" {
		t.Error("request_ts should not be empty")
	}
	if capturedSig == "" {
		t.Error("request_sig should not be empty")
	}
	if session.SessionID != "sess-abc-123" {
		t.Errorf("SessionID = %q, want %q", session.SessionID, "sess-abc-123")
	}
	if session.Profile != "qbz-1" {
		t.Errorf("Profile = %q, want %q", session.Profile, "qbz-1")
	}
}

func TestStreamingReportStart(t *testing.T) {
	var capturedPath string
	var capturedEvents string

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		r.ParseForm()
		capturedEvents = r.PostForm.Get("events")
		w.WriteHeader(200)
		w.Write([]byte(`{"status": "success"}`))
	})
	defer server.Close()

	err := client.Streaming.ReportStart(context.Background(), 12345, 7, 99)
	if err != nil {
		t.Fatalf("ReportStart: %v", err)
	}

	if capturedPath != "/track/reportStreamingStart" {
		t.Errorf("path = %q, want /track/reportStreamingStart", capturedPath)
	}

	// Verify events JSON contains required fields
	var events []map[string]any
	if err := json.Unmarshal([]byte(capturedEvents), &events); err != nil {
		t.Fatalf("unmarshal events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events length = %d, want 1", len(events))
	}
	if events[0]["track_id"] == nil {
		t.Error("event should contain track_id")
	}
	if events[0]["date"] == nil {
		t.Error("event should contain date")
	}
	if events[0]["format_id"] == nil {
		t.Error("event should contain format_id")
	}
	if events[0]["user_id"] == nil {
		t.Error("event should contain user_id")
	}
}

func TestStreamingReportEnd(t *testing.T) {
	var capturedBody map[string]any

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/track/reportStreamingEndJson" {
			t.Errorf("path = %q, want /track/reportStreamingEndJson", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"status": "success"}`))
	})
	defer server.Close()

	events := []map[string]any{
		{"track_id": 123, "duration": 100},
	}
	err := client.Streaming.ReportEnd(context.Background(), events)
	if err != nil {
		t.Fatalf("ReportEnd: %v", err)
	}

	if capturedBody["renderer_context"] == nil {
		t.Error("body should contain renderer_context")
	}
	if capturedBody["events"] == nil {
		t.Error("body should contain events")
	}
}

func TestStreamingReportContext(t *testing.T) {
	var capturedBody map[string]any

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/event/reportTrackContext" {
			t.Errorf("path = %q, want /event/reportTrackContext", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.WriteHeader(200)
		w.Write([]byte(`{"status": "success"}`))
	})
	defer server.Close()

	err := client.Streaming.ReportContext(context.Background(), "uuid-123", map[string]any{
		"contentGroupType": "album",
		"contentGroupId":   "abc",
	})
	if err != nil {
		t.Fatalf("ReportContext: %v", err)
	}

	if capturedBody["version"] != "01.00" {
		t.Errorf("version = %v, want 01.00", capturedBody["version"])
	}
	if capturedBody["events"] == nil {
		t.Error("body should contain events array")
	}
}

func TestStreamingDynamicSuggest(t *testing.T) {
	var capturedBody map[string]any

	server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dynamic/suggest" {
			t.Errorf("path = %q, want /dynamic/suggest", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)

		resp := map[string]any{
			"algorithm": "collaborative",
			"tracks":    map[string]any{"limit": 10, "items": []any{}},
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	result, err := client.Streaming.DynamicSuggest(context.Background(), []int{100, 200, 300}, 10)
	if err != nil {
		t.Fatalf("DynamicSuggest: %v", err)
	}

	// Verify param name is listened_tracks_ids (plural)
	if capturedBody["listened_tracks_ids"] == nil {
		t.Error("body should contain listened_tracks_ids (plural)")
	}
	if capturedBody["limit"] == nil {
		t.Error("body should contain limit")
	}
	if result == nil {
		t.Error("result should not be nil")
	}
}

func TestStreamingQualityMapping(t *testing.T) {
	tests := []struct {
		quality  int
		formatID int
	}{
		{1, 5},
		{2, 6},
		{3, 7},
		{4, 27},
	}

	for _, tt := range tests {
		var capturedFormatID string

		server, client := testServerAndClient(func(w http.ResponseWriter, r *http.Request) {
			capturedFormatID = r.URL.Query().Get("format_id")
			resp := map[string]any{
				"track_id":     100,
				"format_id":    tt.formatID,
				"url_template": "https://example.com/$SEGMENT$",
				"n_segments":   1,
			}
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(resp)
		})

		_, err := client.Streaming.GetFileURL(context.Background(), 100, tt.quality)
		if err != nil {
			t.Fatalf("GetFileURL quality %d: %v", tt.quality, err)
		}

		expected := ""
		switch tt.formatID {
		case 5:
			expected = "5"
		case 6:
			expected = "6"
		case 7:
			expected = "7"
		case 27:
			expected = "27"
		}
		if capturedFormatID != expected {
			t.Errorf("quality %d: format_id = %q, want %q", tt.quality, capturedFormatID, expected)
		}

		server.Close()
	}
}
