"""Tests for the Tidal streaming API (manifest decoding + quality fallback)."""

import base64
import json

import pytest
from unittest.mock import AsyncMock

from tidal.errors import NonStreamableError
from tidal.streaming import StreamingAPI
from tidal.types import StreamManifest


def _encode_manifest(payload: dict) -> str:
    return base64.b64encode(json.dumps(payload).encode("utf-8")).decode("ascii")


@pytest.fixture
def streaming(mock_transport: AsyncMock) -> StreamingAPI:
    return StreamingAPI(mock_transport)


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------


async def test_get_manifest_unencrypted_flac(
    streaming: StreamingAPI, mock_transport: AsyncMock
):
    manifest_payload = {
        "urls": ["https://cdn.tidal.com/audio.flac"],
        "codecs": "flac",
        "encryptionType": "NONE",
    }
    mock_transport.get.return_value = (
        200,
        {"manifest": _encode_manifest(manifest_payload)},
    )

    result = await streaming.get_manifest(67890, quality=2)

    assert isinstance(result, StreamManifest)
    assert result.track_id == 67890
    assert result.audio_quality == "LOSSLESS"
    assert result.codec == "flac"
    assert result.url == "https://cdn.tidal.com/audio.flac"
    assert result.is_encrypted is False
    assert result.file_extension == "flac"

    mock_transport.get.assert_awaited_once_with(
        "tracks/67890/playbackinfopostpaywall",
        {
            "audioquality": "LOSSLESS",
            "playbackmode": "STREAM",
            "assetpresentation": "FULL",
        },
    )


async def test_get_manifest_encrypted_mqa(
    streaming: StreamingAPI, mock_transport: AsyncMock
):
    manifest_payload = {
        "urls": ["https://cdn.tidal.com/audio.mqa"],
        "codecs": "mqa",
        "encryptionType": "OLD_AES",
        "keyId": "fakebase64key==",
    }
    mock_transport.get.return_value = (
        200,
        {"manifest": _encode_manifest(manifest_payload)},
    )

    result = await streaming.get_manifest(67890, quality=3)
    assert result.is_encrypted is True
    assert result.encryption_key == "fakebase64key=="
    assert result.file_extension == "flac"


async def test_get_manifest_aac(
    streaming: StreamingAPI, mock_transport: AsyncMock
):
    manifest_payload = {
        "urls": ["https://cdn.tidal.com/audio.aac"],
        "codecs": "aac",
        "encryptionType": "NONE",
    }
    mock_transport.get.return_value = (
        200,
        {"manifest": _encode_manifest(manifest_payload)},
    )

    result = await streaming.get_manifest(67890, quality=1)
    assert result.audio_quality == "HIGH"
    assert result.file_extension == "m4a"


# ---------------------------------------------------------------------------
# Quality fallback
# ---------------------------------------------------------------------------


async def test_falls_back_to_lower_quality_on_decode_error(
    streaming: StreamingAPI, mock_transport: AsyncMock
):
    """If a HI_RES request returns garbage, the SDK retries at LOSSLESS."""
    bad_resp = (200, {"manifest": "not-base64-or-not-json"})
    good_payload = {
        "urls": ["https://cdn.tidal.com/audio.flac"],
        "codecs": "flac",
        "encryptionType": "NONE",
    }
    good_resp = (200, {"manifest": _encode_manifest(good_payload)})
    mock_transport.get.side_effect = [bad_resp, good_resp]

    result = await streaming.get_manifest(67890, quality=3)

    assert mock_transport.get.await_count == 2
    # Second call uses LOSSLESS
    args = mock_transport.get.await_args_list[1]
    assert args[0][1]["audioquality"] == "LOSSLESS"
    assert result.audio_quality == "LOSSLESS"


async def test_raises_after_exhausting_qualities(
    streaming: StreamingAPI, mock_transport: AsyncMock
):
    bad = (200, {"manifest": "garbage"})
    mock_transport.get.return_value = bad

    with pytest.raises(NonStreamableError) as exc:
        await streaming.get_manifest(67890, quality=3)
    assert "not available" in str(exc.value)


# ---------------------------------------------------------------------------
# Error paths
# ---------------------------------------------------------------------------


async def test_raises_when_manifest_missing(
    streaming: StreamingAPI, mock_transport: AsyncMock
):
    mock_transport.get.return_value = (
        200,
        {"userMessage": "Not available in your region"},
    )

    with pytest.raises(NonStreamableError) as exc:
        await streaming.get_manifest(67890, quality=3)
    assert "region" in str(exc.value)


async def test_raises_when_no_urls(
    streaming: StreamingAPI, mock_transport: AsyncMock
):
    manifest_payload = {
        "urls": [],
        "codecs": "flac",
        "encryptionType": "NONE",
        "restrictions": [{"code": "RegionRestricted"}],
    }
    mock_transport.get.return_value = (
        200,
        {"manifest": _encode_manifest(manifest_payload)},
    )

    with pytest.raises(NonStreamableError) as exc:
        await streaming.get_manifest(67890, quality=3)
    assert "RegionRestricted" in str(exc.value)


async def test_clamps_invalid_quality_above_range(
    streaming: StreamingAPI, mock_transport: AsyncMock
):
    manifest_payload = {
        "urls": ["http://x"],
        "codecs": "flac",
        "encryptionType": "NONE",
    }
    mock_transport.get.return_value = (
        200,
        {"manifest": _encode_manifest(manifest_payload)},
    )

    result = await streaming.get_manifest(67890, quality=99)
    # Clamped to 3 (HI_RES)
    assert result.audio_quality == "HI_RES"
