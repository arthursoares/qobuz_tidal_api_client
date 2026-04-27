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
import logging
import xml.etree.ElementTree as ET
from json import JSONDecodeError
from typing import Any

from ._http import HttpTransport
from .errors import NonStreamableError
from .types import QUALITY_MAP, StreamManifest

logger = logging.getLogger(__name__)

# Tidal's DASH manifests use the standard MPEG-DASH namespace.
_DASH_NS = {"mpd": "urn:mpeg:dash:schema:mpd:2011"}


def _parse_dash_manifest(xml_text: str) -> tuple[str, list[str]]:
    """Parse a Tidal DASH MPD into ``(codec, [init_url, *segment_urls])``.

    Tidal's MPDs are minimal — single Period, single AdaptationSet,
    single Representation, ``SegmentTemplate`` + ``SegmentTimeline``.
    Walks the timeline to compute the segment count, then expands
    ``$Number$`` against ``startNumber``. The init segment is downloaded
    verbatim (its URL is fixed, no template substitution).
    """
    root = ET.fromstring(xml_text)
    rep = root.find(".//mpd:Representation", _DASH_NS)
    if rep is None:
        raise NonStreamableError("DASH manifest has no Representation")
    codec = (rep.get("codecs") or "").lower()

    tmpl = rep.find("mpd:SegmentTemplate", _DASH_NS)
    if tmpl is None:
        raise NonStreamableError("DASH manifest has no SegmentTemplate")
    init_url = tmpl.get("initialization")
    media_pattern = tmpl.get("media")
    start_number = int(tmpl.get("startNumber", "1"))
    if not init_url or not media_pattern:
        raise NonStreamableError("DASH SegmentTemplate missing initialization/media")

    timeline = tmpl.find("mpd:SegmentTimeline", _DASH_NS)
    if timeline is None:
        raise NonStreamableError("DASH manifest has no SegmentTimeline")
    # Each <S> has duration `d` and an optional `r` repeat count where
    # r="N" means "this S plus N additional repeats" (i.e. N+1 segments).
    segment_count = 0
    for s in timeline.findall("mpd:S", _DASH_NS):
        segment_count += 1 + int(s.get("r", "0"))
    if segment_count <= 0:
        raise NonStreamableError("DASH SegmentTimeline produced no segments")

    media_urls = [
        media_pattern.replace("$Number$", str(start_number + i))
        for i in range(segment_count)
    ]
    return codec, [init_url, *media_urls]


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
            quality = max(0, min(quality, 4))

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

        # ``manifestMimeType`` selects the wire format:
        #   application/vnd.tidal.bts → base64-JSON, single-URL (legacy MQA/AAC)
        #   application/dash+xml      → base64-MPD, multi-segment (FLAC/HiRes)
        mime = resp.get("manifestMimeType") or "?"
        encryption_type = resp.get("encryptionType") or "NONE"
        logger.debug(
            "tidal manifest track=%s requested_quality=%s mime=%s len=%d enc=%s",
            track_id, QUALITY_MAP.get(quality, quality), mime,
            len(resp.get("manifest") or ""), encryption_type,
        )

        decoded_manifest = base64.b64decode(resp["manifest"])

        if mime == "application/dash+xml":
            # Tidal hands DASH manifests to PKCE-issued tokens. Even encrypted
            # tracks would land here — but Tidal currently sets encryptionType
            # to "None" on PKCE responses (cf. tidal-tooling community).
            if encryption_type and encryption_type.upper() not in ("NONE", "OFFLINEONLY"):
                raise NotImplementedError(
                    f"DASH manifest with {encryption_type!r} encryption "
                    f"isn't supported yet (track {track_id})"
                )
            try:
                codec, segment_urls = _parse_dash_manifest(
                    decoded_manifest.decode("utf-8")
                )
            except ET.ParseError as exc:
                raise NonStreamableError(
                    f"Failed to parse Tidal DASH manifest: {exc}"
                ) from exc
            return StreamManifest(
                track_id=track_id,
                audio_quality=QUALITY_MAP[quality],
                codec=codec,
                url=segment_urls[0],
                urls=segment_urls,
                is_dash=True,
                encryption_type="NONE",
                encryption_key=None,
            )

        # Legacy BTS (single-URL JSON) path
        try:
            manifest = json.loads(decoded_manifest.decode("utf-8"))
        except (JSONDecodeError, binascii.Error, UnicodeDecodeError, ValueError) as exc:
            if quality <= 0:
                raise NonStreamableError(
                    f"Track {track_id} is not available at any quality"
                )
            logger.warning(
                "Tidal BTS manifest decode failed for track %s at quality=%s "
                "(mime=%s, %s). Falling back to quality=%s.",
                track_id, QUALITY_MAP.get(quality, quality), mime, exc,
                QUALITY_MAP.get(quality - 1, quality - 1),
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

        bts_encryption_type = manifest.get("encryptionType", "NONE")
        encryption_key = manifest.get("keyId")
        if bts_encryption_type == "NONE":
            encryption_key = None

        return StreamManifest(
            track_id=track_id,
            audio_quality=QUALITY_MAP[quality],
            codec=codecs,
            url=urls[0],
            urls=list(urls),
            is_dash=False,
            encryption_type=bts_encryption_type,
            encryption_key=encryption_key,
            restrictions=manifest.get("restrictions") or [],
        )
