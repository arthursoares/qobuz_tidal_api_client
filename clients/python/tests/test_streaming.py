"""Tests for streaming API — file URLs, sessions, and playback reporting."""

import hashlib
import json
import time

import pytest
from unittest.mock import AsyncMock

from qobuz.streaming import StreamingAPI, QUALITY_MAP, _compute_signature
from qobuz.types import FileUrl, Session


# ---------------------------------------------------------------------------
# Sample API responses
# ---------------------------------------------------------------------------

SAMPLE_FILE_URL = {
    "track_id": 33967376,
    "format_id": 7,
    "mime_type": "audio/flac",
    "sampling_rate": 44100,
    "bits_depth": 16,
    "duration": 133.0,
    "url_template": "https://streaming.qobuz.com/file?uid=...",
    "n_segments": 1,
    "restrictions": [],
}

SAMPLE_SESSION = {
    "session_id": "abc123-sess",
    "profile": "qbz-1",
    "expires_at": 1775700000,
}


# ---------------------------------------------------------------------------
# _compute_signature
# ---------------------------------------------------------------------------

class TestComputeSignature:
    def test_produces_correct_md5(self):
        sig = _compute_signature(
            endpoint="fileUrl",
            track_id="33967376",
            format_id="7",
            intent="stream",
            timestamp="1700000000",
            app_secret="mysecret",
        )
        raw = "fileUrlformat_id7intentstreamtrack_id339673761700000000mysecret"
        expected = hashlib.md5(raw.encode()).hexdigest()
        assert sig == expected

    def test_different_inputs_produce_different_hashes(self):
        sig1 = _compute_signature("fileUrl", "1", "7", "stream", "100", "secret")
        sig2 = _compute_signature("fileUrl", "2", "7", "stream", "100", "secret")
        assert sig1 != sig2


# ---------------------------------------------------------------------------
# get_file_url
# ---------------------------------------------------------------------------

class TestGetFileUrl:
    async def test_calls_transport_with_correct_endpoint(self):
        transport = AsyncMock()
        transport.get.return_value = (200, SAMPLE_FILE_URL)

        api = StreamingAPI(transport, app_secret="test-secret")
        result = await api.get_file_url(track_id=33967376, quality=3, intent="stream")

        assert isinstance(result, FileUrl)
        assert result.track_id == 33967376
        assert result.format_id == 7

        transport.get.assert_called_once()
        call_args = transport.get.call_args
        assert call_args[0][0] == "file/url"

        params = call_args[0][1]
        assert params["track_id"] == 33967376
        assert params["format_id"] == 7
        assert params["intent"] == "stream"
        assert "request_ts" in params
        assert "request_sig" in params

    async def test_raises_value_error_without_app_secret(self):
        transport = AsyncMock()
        api = StreamingAPI(transport, app_secret=None)

        with pytest.raises(ValueError, match="app_secret"):
            await api.get_file_url(track_id=123)

    async def test_quality_mapping(self):
        """Quality 1-4 maps to format_id 5, 6, 7, 27."""
        transport = AsyncMock()
        transport.get.return_value = (200, SAMPLE_FILE_URL)

        api = StreamingAPI(transport, app_secret="secret")

        for quality, expected_format in QUALITY_MAP.items():
            await api.get_file_url(track_id=1, quality=quality)
            params = transport.get.call_args[0][1]
            assert params["format_id"] == expected_format, (
                f"quality={quality} should map to format_id={expected_format}"
            )


# ---------------------------------------------------------------------------
# start_session
# ---------------------------------------------------------------------------

class TestStartSession:
    async def test_returns_session(self):
        transport = AsyncMock()
        transport.post_form.return_value = (200, SAMPLE_SESSION)

        api = StreamingAPI(transport, app_secret="test-secret")
        result = await api.start_session()

        assert isinstance(result, Session)
        assert result.session_id == "abc123-sess"
        assert result.profile == "qbz-1"
        assert result.expires_at == 1775700000

    async def test_includes_profile_in_body(self):
        transport = AsyncMock()
        transport.post_form.return_value = (200, SAMPLE_SESSION)

        api = StreamingAPI(transport, app_secret="test-secret")
        await api.start_session()

        data = transport.post_form.call_args[0][1]
        assert data["profile"] == "qbz-1"
        assert "request_ts" in data
        assert "request_sig" in data

    async def test_session_signature_format(self):
        """Session signature is MD5(f'qbz-1{timestamp}{app_secret}')."""
        transport = AsyncMock()
        transport.post_form.return_value = (200, SAMPLE_SESSION)

        api = StreamingAPI(transport, app_secret="test-secret")
        await api.start_session()

        data = transport.post_form.call_args[0][1]
        ts = data["request_ts"]
        expected_sig = hashlib.md5(f"qbz-1{ts}test-secret".encode()).hexdigest()
        assert data["request_sig"] == expected_sig

    async def test_raises_value_error_without_app_secret(self):
        transport = AsyncMock()
        api = StreamingAPI(transport, app_secret=None)

        with pytest.raises(ValueError, match="app_secret"):
            await api.start_session()


# ---------------------------------------------------------------------------
# report_start
# ---------------------------------------------------------------------------

class TestReportStart:
    async def test_calls_post_form_with_date(self):
        transport = AsyncMock()
        transport.post_form.return_value = (200, {"status": "ok"})

        api = StreamingAPI(transport)
        result = await api.report_start(
            track_id=33967376, format_id=7, user_id=2113276
        )

        assert result == {"status": "ok"}
        transport.post_form.assert_called_once()
        call_args = transport.post_form.call_args
        assert call_args[0][0] == "track/reportStreamingStart"

        data = call_args[0][1]
        events_decoded = json.loads(data["events"])
        assert len(events_decoded) == 1
        event = events_decoded[0]
        assert event["track_id"] == 33967376
        assert event["format_id"] == 7
        assert event["user_id"] == 2113276
        assert "date" in event
        assert isinstance(event["date"], int)


# ---------------------------------------------------------------------------
# report_end
# ---------------------------------------------------------------------------

class TestReportEnd:
    async def test_includes_renderer_context(self):
        transport = AsyncMock()
        transport.post_json.return_value = (200, {"status": "ok"})

        events = [{"track_id": 33967376, "duration": 120}]
        api = StreamingAPI(transport)
        result = await api.report_end(events=events)

        assert result == {"status": "ok"}
        transport.post_json.assert_called_once()
        call_args = transport.post_json.call_args
        assert call_args[0][0] == "track/reportStreamingEndJson"

        body = call_args[0][1]
        assert body["events"] == events
        assert "renderer_context" in body
        assert "software_version" in body["renderer_context"]


# ---------------------------------------------------------------------------
# report_context
# ---------------------------------------------------------------------------

class TestReportContext:
    async def test_wraps_in_version_and_events_array(self):
        transport = AsyncMock()
        transport.post_json.return_value = (200, {"status": "ok"})

        api = StreamingAPI(transport)
        result = await api.report_context(
            track_context_uuid="uuid-123",
            data={"source": "album", "album_id": "abc"},
        )

        assert result == {"status": "ok"}
        call_args = transport.post_json.call_args
        assert call_args[0][0] == "event/reportTrackContext"

        body = call_args[0][1]
        assert body["version"] == "01.00"
        assert isinstance(body["events"], list)
        assert len(body["events"]) == 1
        assert body["events"][0]["track_context_uuid"] == "uuid-123"
        assert body["events"][0]["data"] == {"source": "album", "album_id": "abc"}


# ---------------------------------------------------------------------------
# dynamic_suggest
# ---------------------------------------------------------------------------

class TestDynamicSuggest:
    async def test_uses_correct_param_name(self):
        transport = AsyncMock()
        transport.post_json.return_value = (200, {"tracks": []})

        api = StreamingAPI(transport)
        result = await api.dynamic_suggest(listened_tracks_ids=[1, 2, 3])

        assert result == {"tracks": []}
        body = transport.post_json.call_args[0][1]
        assert "listened_tracks_ids" in body  # plural "tracks"
        assert body["listened_tracks_ids"] == [1, 2, 3]
        assert body["limit"] == 50

    async def test_custom_limit(self):
        transport = AsyncMock()
        transport.post_json.return_value = (200, {"tracks": []})

        api = StreamingAPI(transport)
        await api.dynamic_suggest(listened_tracks_ids=[1], limit=10)

        body = transport.post_json.call_args[0][1]
        assert body["limit"] == 10
