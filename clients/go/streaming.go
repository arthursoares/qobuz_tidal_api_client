package qobuz

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// QualityMap maps quality tiers (1-4) to Qobuz format IDs (5, 6, 7, 27).
var QualityMap = map[int]int{
	1: 5,
	2: 6,
	3: 7,
	4: 27,
}

// StreamingService provides methods for streaming, sessions, and playback telemetry.
type StreamingService struct {
	t         *transport
	appSecret string
}

// computeFileURLSignature computes the MD5 request signature for file URL endpoints.
// Raw string: "{endpoint}format_id{format_id}intent{intent}track_id{track_id}{timestamp}{app_secret}"
func computeFileURLSignature(endpoint, trackID, formatID, intent, timestamp, appSecret string) string {
	raw := fmt.Sprintf("%sformat_id%sintent%strack_id%s%s%s", endpoint, formatID, intent, trackID, timestamp, appSecret)
	return fmt.Sprintf("%x", md5.Sum([]byte(raw)))
}

// computeSessionSignature computes the MD5 signature for session/start.
// Raw string: "qbz-1{timestamp}{app_secret}"
func computeSessionSignature(timestamp, appSecret string) string {
	raw := fmt.Sprintf("qbz-1%s%s", timestamp, appSecret)
	return fmt.Sprintf("%x", md5.Sum([]byte(raw)))
}

// GetFileURL gets a streamable/downloadable file URL for a track.
// quality is 1-4, mapped to Qobuz format IDs (5, 6, 7, 27).
func (s *StreamingService) GetFileURL(ctx context.Context, trackID int, quality int) (*FileUrl, error) {
	if s.appSecret == "" {
		return nil, errors.New("app_secret is required for GetFileURL")
	}

	formatID, ok := QualityMap[quality]
	if !ok {
		formatID = 7 // default to quality 3
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	sig := computeFileURLSignature(
		"fileUrl",
		fmt.Sprintf("%d", trackID),
		fmt.Sprintf("%d", formatID),
		"stream",
		timestamp,
		s.appSecret,
	)

	data, err := s.t.get(ctx, "file/url", map[string]string{
		"track_id":    fmt.Sprintf("%d", trackID),
		"format_id":   fmt.Sprintf("%d", formatID),
		"intent":      "stream",
		"request_ts":  timestamp,
		"request_sig": sig,
	})
	if err != nil {
		return nil, err
	}

	var f FileUrl
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse file url: %w", err)
	}
	return &f, nil
}

// StartSession starts a streaming session.
func (s *StreamingService) StartSession(ctx context.Context) (*Session, error) {
	if s.appSecret == "" {
		return nil, errors.New("app_secret is required for StartSession")
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	sig := computeSessionSignature(timestamp, s.appSecret)

	data, err := s.t.postForm(ctx, "session/start", map[string]string{
		"profile":     "qbz-1",
		"request_ts":  timestamp,
		"request_sig": sig,
	})
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}
	return &session, nil
}

// ReportStart reports the start of track playback.
func (s *StreamingService) ReportStart(ctx context.Context, trackID, formatID, userID int) error {
	event := map[string]any{
		"track_id":  trackID,
		"date":      int(time.Now().Unix()),
		"format_id": formatID,
		"user_id":   userID,
	}
	eventsJSON, err := json.Marshal([]map[string]any{event})
	if err != nil {
		return fmt.Errorf("marshal events: %w", err)
	}

	_, err = s.t.postForm(ctx, "track/reportStreamingStart", map[string]string{
		"events": string(eventsJSON),
	})
	return err
}

// ReportEnd reports the end of track playback.
func (s *StreamingService) ReportEnd(ctx context.Context, events []map[string]any) error {
	body := map[string]any{
		"events":           events,
		"renderer_context": map[string]any{"software_version": "qobuz-sdk-0.1.0"},
	}
	_, err := s.t.postJSON(ctx, "track/reportStreamingEndJson", body)
	return err
}

// ReportContext reports track playback context.
func (s *StreamingService) ReportContext(ctx context.Context, trackContextUUID string, data map[string]any) error {
	body := map[string]any{
		"version": "01.00",
		"events": []map[string]any{
			{
				"track_context_uuid": trackContextUUID,
				"data":               data,
			},
		},
	}
	_, err := s.t.postJSON(ctx, "event/reportTrackContext", body)
	return err
}

// DynamicSuggest gets dynamic track suggestions based on listening history.
func (s *StreamingService) DynamicSuggest(ctx context.Context, listenedTracksIDs []int, limit int) (json.RawMessage, error) {
	data, err := s.t.postJSON(ctx, "dynamic/suggest", map[string]any{
		"listened_tracks_ids": listenedTracksIDs,
		"limit":               limit,
	})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
