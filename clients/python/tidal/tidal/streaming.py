"""Streaming API — track playback URLs and manifest decoding.

This module wraps the legacy Tidal v1 ``playbackinfopostpaywall`` endpoint,
which returns a base64-encoded JSON manifest containing the actual file
URL, codec, and (for HiRes/MQA) the AES-CTR encryption key. The downloader
consumes :class:`StreamManifest` objects produced here.
"""

from __future__ import annotations

import base64
import binascii
import json
from json import JSONDecodeError
from typing import Any

from ._http import HttpTransport
from .errors import NonStreamableError
from .types import QUALITY_MAP, StreamManifest


class StreamingAPI:
    """Resolves Tidal track IDs to playable / downloadable manifests."""

    def __init__(self, transport: HttpTransport) -> None:
        self._t = transport

    async def get_manifest(
        self,
        track_id: int | str,
        *,
        quality: int = 3,
    ) -> StreamManifest:
        """Fetch a stream manifest for a track at the requested quality.

        Falls back through lower quality tiers (HI_RES → LOSSLESS → HIGH →
        LOW) if the track is not available at the requested quality. Raises
        :class:`NonStreamableError` if every tier fails.
        """
        return await self._fetch_with_fallback(int(track_id), quality)

    async def _fetch_with_fallback(
        self, track_id: int, quality: int
    ) -> StreamManifest:
        if quality not in QUALITY_MAP:
            quality = 3 if quality > 3 else 0

        params: dict[str, Any] = {
            "audioquality": QUALITY_MAP[quality],
            "playbackmode": "STREAM",
            "assetpresentation": "FULL",
        }
        _, resp = await self._t.get(
            f"tracks/{track_id}/playbackinfopostpaywall", params
        )

        if "manifest" not in resp:
            user_msg = resp.get("userMessage") or "no manifest in response"
            raise NonStreamableError(user_msg)

        try:
            manifest = json.loads(base64.b64decode(resp["manifest"]).decode("utf-8"))
        except (JSONDecodeError, binascii.Error, UnicodeDecodeError, ValueError):
            if quality <= 0:
                raise NonStreamableError(
                    f"Track {track_id} is not available at any quality"
                )
            return await self._fetch_with_fallback(track_id, quality - 1)

        codecs = manifest.get("codecs") or ""
        urls = manifest.get("urls") or []
        if not urls:
            restrictions = manifest.get("restrictions") or []
            msg = (
                restrictions[0].get("code", "Restricted")
                if restrictions and isinstance(restrictions[0], dict)
                else "no urls in manifest"
            )
            raise NonStreamableError(msg)

        encryption_type = manifest.get("encryptionType", "NONE")
        encryption_key = manifest.get("keyId")
        if encryption_type == "NONE":
            encryption_key = None

        return StreamManifest(
            track_id=track_id,
            audio_quality=QUALITY_MAP[quality],
            codec=codecs,
            url=urls[0],
            encryption_type=encryption_type,
            encryption_key=encryption_key,
            restrictions=manifest.get("restrictions") or [],
        )
