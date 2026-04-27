"""Album downloader — fetches tracks, decrypts MQA, tags, and writes files."""

from __future__ import annotations

import asyncio
import base64
import json
import logging
import os
import re
import time
from dataclasses import dataclass, field
from typing import Any, Callable

import aiofiles
from Cryptodome.Cipher import AES
from Cryptodome.Util import Counter

from .catalog import CatalogAPI
from .client import TidalClient
from .errors import NonStreamableError
from .streaming import StreamingAPI
from .types import Album, StreamManifest, Track

logger = logging.getLogger("tidal.downloader")

# AES master key for MQA / HI_RES manifest decryption. Public knowledge,
# baked into the Tidal mobile app and used by every Tidal-tooling project.
_MQA_MASTER_KEY_B64 = "UIlTTEMmmLfGowo/UC60x2H45W6MdGgTRfo/umg4754="

ProgressCallback = Callable[[int], None]
TrackStartCallback = Callable[[int, str], None]
TrackProgressCallback = Callable[[int, int, int], None]  # num, bytes_done, bytes_total
TrackCompleteCallback = Callable[[int, str, bool], None]  # num, title, success


@dataclass
class DownloadConfig:
    """Knobs that control the download pipeline.

    Mirrors qobuz.DownloadConfig so the streamrip backend can pass nearly
    identical settings to either SDK.
    """

    output_dir: str
    quality: int = 3  # 0=LOW, 1=HIGH, 2=LOSSLESS, 3=HI_RES (MQA / FLAC HiRes)
    folder_format: str = (
        "{albumartist}/({year}) {title} [{container}-{bit_depth}-{sampling_rate}]"
    )
    track_format: str = "{tracknumber:02}. {artist} - {title}{explicit}"
    max_connections: int = 6
    embed_cover: bool = True
    cover_size: str = "large"  # "small", "medium", "large", "640x640", etc.
    source_subdirectories: bool = False
    disc_subdirectories: bool = True
    skip_downloaded: bool = True
    downloads_db_path: str | None = None
    tag_files: bool = True
    write_metadata_file: bool = True


@dataclass
class TrackResult:
    track_id: int
    title: str
    success: bool
    error: str | None = None
    file_path: str | None = None


@dataclass
class AlbumResult:
    album_id: int
    title: str
    artist: str
    total: int
    successful: int
    tracks: list[TrackResult] = field(default_factory=list)

    @property
    def success_rate(self) -> float:
        return self.successful / self.total if self.total else 0.0


# ---------------------------------------------------------------------------
# Helpers — kept module-level so they can be unit-tested without instantiating
# the whole downloader.
# ---------------------------------------------------------------------------


async def _remux_mp4_to_flac(path: str) -> bool:
    """Remux an MP4-with-FLAC file in place into a native ``.flac`` file.

    DASH-delivered Tidal lossless lands as fragmented MP4 with FLAC frames
    inside. ffmpeg's ``-c:a copy`` extracts the FLAC stream into a real
    native FLAC container — no re-encoding, lossless, fast.

    Returns True if the file was remuxed (now a real FLAC), False if
    ffmpeg wasn't on PATH (file left as MP4-wrapped FLAC). Raises on
    ffmpeg execution failure. The boolean lets callers skip the FLAC
    tagging path when the file isn't actually FLAC yet — mutagen.flac
    raises FLACNoHeaderError on MP4-wrapped data.
    """
    import shutil

    if shutil.which("ffmpeg") is None:
        logger.warning(
            "ffmpeg not on PATH — leaving %s as MP4-wrapped FLAC. "
            "Install ffmpeg to get a native .flac that strict scanners "
            "and the mutagen tag pipeline accept.",
            os.path.basename(path),
        )
        return False

    tmp_path = path + ".remux.flac"
    proc = await asyncio.create_subprocess_exec(
        "ffmpeg",
        "-loglevel", "error",
        "-y",                 # overwrite
        "-i", path,
        "-c:a", "copy",       # stream copy, no re-encode
        "-vn",                # drop any embedded artwork (we tag it later)
        "-f", "flac",
        tmp_path,
        stdout=asyncio.subprocess.DEVNULL,
        stderr=asyncio.subprocess.PIPE,
    )
    _, stderr = await proc.communicate()
    if proc.returncode != 0:
        if os.path.exists(tmp_path):
            try:
                os.remove(tmp_path)
            except OSError:
                pass
        raise RuntimeError(
            f"ffmpeg remux failed for {os.path.basename(path)}: "
            f"{stderr.decode('utf-8', errors='replace')[:300]}"
        )
    os.replace(tmp_path, path)
    return True


def _safe_value(s: str) -> str:
    """Sanitize a metadata value for use as a path component.

    Replaces filesystem-reserved characters and turns ``/`` into `` - ``
    so titles like "AC/DC" don't create nested directories.
    """
    s = s.replace("/", " - ")
    s = re.sub(r'[<>:"\\|?*]', "", s)
    s = s.replace("\n", " ").replace("\r", "").strip()
    return s[:200]


def _safe_filename(s: str) -> str:
    s = _safe_value(s)
    return s.rstrip(". ")  # Windows doesn't allow trailing dots/spaces


def _zero_pad(n: int, width: int = 2) -> str:
    return f"{n:0{width}d}"


def _build_albumartist(album: Album) -> str:
    return album.artist.name or "Unknown"


_TIDAL_TIER_RANK = {"LOW": 0, "HIGH": 1, "LOSSLESS": 2, "HI_RES": 3, "HI_RES_LOSSLESS": 4}
_TIDAL_TIER_NAME = {0: "LOW", 1: "HIGH", 2: "LOSSLESS", 3: "HI_RES", 4: "HI_RES_LOSSLESS"}


def _tidal_quality_fields(
    album_audio_quality: str | None,
    config_quality: int,
) -> tuple[str, int, str]:
    """Return ``(container, bit_depth, sampling_rate)`` for folder formatting.

    The actual download tier is the *minimum* of (a) the highest tier the
    album is available in, and (b) what the user asked for. Using the
    album's max alone (the previous behaviour) labels folders as FLAC even
    when the user has tidal_quality=HIGH and the actual files are AAC m4a.

    Tier → format:

    - ``LOW``             — AAC ~96 kbps
    - ``HIGH``            — AAC ~320 kbps
    - ``LOSSLESS``        — FLAC 16-bit, 44.1 kHz (CD quality)
    - ``HI_RES``          — FLAC 16/44.1 with MQA folded in (legacy MQA tier)
    - ``HI_RES_LOSSLESS`` — true 24-bit FLAC, up to 192 kHz
    """
    cfg = max(0, min(int(config_quality), 4))
    album_rank = _TIDAL_TIER_RANK.get(
        (album_audio_quality or "").upper(),
        cfg,  # absent ⇒ no album-side cap, trust the user's config
    )
    quality = _TIDAL_TIER_NAME[min(cfg, album_rank)]

    if quality in ("LOW", "HIGH"):
        return "AAC", 16, "44.1"
    if quality == "LOSSLESS":
        return "FLAC", 16, "44.1"
    if quality == "HI_RES":
        return "FLAC", 16, "44.1"  # MQA-encoded; container is 16/44.1
    if quality == "HI_RES_LOSSLESS":
        return "FLAC", 24, "192"
    return "FLAC", 16, "44.1"


def _decrypt_mqa(input_path: str, output_path: str, encryption_key_b64: str) -> None:
    """Decrypt a Tidal MQA / HI_RES file in place.

    The manifest's ``keyId`` is a base64 blob containing a 16-byte IV
    followed by an AES-CBC ciphertext that, once decrypted with the
    public master key, yields the AES-CTR key + nonce for the audio
    payload. The audio file itself is then decrypted in CTR mode and
    written to *output_path*.
    """
    master_key = base64.b64decode(_MQA_MASTER_KEY_B64)
    security_token = base64.b64decode(encryption_key_b64)

    iv = security_token[:16]
    encrypted_st = security_token[16:]

    decryptor = AES.new(master_key, AES.MODE_CBC, iv)
    decrypted_st = decryptor.decrypt(encrypted_st)

    key = decrypted_st[:16]
    nonce = decrypted_st[16:24]

    counter = Counter.new(64, prefix=nonce, initial_value=0)
    cipher = AES.new(key, AES.MODE_CTR, counter=counter)

    with open(input_path, "rb") as enc_file:
        enc_bytes = enc_file.read()
    dec_bytes = cipher.decrypt(enc_bytes)
    with open(output_path, "wb") as out_file:
        out_file.write(dec_bytes)


# ---------------------------------------------------------------------------
# AlbumDownloader
# ---------------------------------------------------------------------------


class AlbumDownloader:
    """Download a Tidal album end-to-end.

    Mirrors :class:`qobuz.AlbumDownloader`'s public surface so the
    streamrip backend can pick the right SDK by source and reuse the
    same callback wiring.
    """

    def __init__(
        self,
        client: TidalClient,
        config: DownloadConfig,
        *,
        on_track_start: TrackStartCallback | None = None,
        on_track_progress: TrackProgressCallback | None = None,
        on_track_complete: TrackCompleteCallback | None = None,
    ) -> None:
        self._client = client
        self._config = config
        self._on_track_start = on_track_start
        self._on_track_progress = on_track_progress
        self._on_track_complete = on_track_complete

        # Subset accessors for convenience
        self._catalog: CatalogAPI = client.catalog
        self._streaming: StreamingAPI = client.streaming

    # -- Public entry point --------------------------------------------------

    async def download(self, album_id: int | str) -> AlbumResult:
        album, tracks = await self._catalog.get_album_with_tracks(album_id)
        album_dir = self._build_album_folder(album)
        os.makedirs(album_dir, exist_ok=True)

        cover_path = await self._download_cover(album, album_dir)

        sem = asyncio.Semaphore(self._config.max_connections)

        async def _wrapped(track: Track) -> TrackResult:
            async with sem:
                try:
                    return await self._download_one_track(
                        track, album, album_dir, cover_path
                    )
                except Exception as exc:
                    # Catches anything _download_one_track didn't handle
                    # itself (most notably user-supplied callback exceptions
                    # that fire outside the per-track try/except). One bad
                    # track must never cancel siblings.
                    logger.exception(
                        "Unhandled error downloading track %s", track.id
                    )
                    if self._on_track_complete is not None:
                        try:
                            self._on_track_complete(
                                track.track_number or 0, track.title, False
                            )
                        except Exception:
                            logger.exception(
                                "on_track_complete callback raised for track %s",
                                track.id,
                            )
                    return TrackResult(
                        track_id=track.id,
                        title=track.title,
                        success=False,
                        error=str(exc),
                    )

        raw_results = await asyncio.gather(
            *(_wrapped(t) for t in tracks), return_exceptions=True
        )
        results: list[TrackResult] = []
        for idx, tr in enumerate(raw_results):
            if isinstance(tr, BaseException):
                # Defensive — the inner try/except should have converted
                # every real failure into a TrackResult already.
                track = tracks[idx]
                logger.exception(
                    "Track %s failed with unwrapped exception: %s", track.id, tr
                )
                results.append(
                    TrackResult(
                        track_id=track.id,
                        title=track.title,
                        success=False,
                        error=str(tr),
                    )
                )
            else:
                results.append(tr)

        successful = sum(1 for r in results if r.success)
        album_result = AlbumResult(
            album_id=album.id,
            title=album.title,
            artist=album.artist.name,
            total=len(tracks),
            successful=successful,
            tracks=results,
        )

        # Write metadata sentinel for filesystem-based dedup + scan
        # reconciliation, only if at least one track succeeded.  Same
        # filename and key shape as the Qobuz SDK so a single scanner
        # picks up albums from either source.
        if self._config.write_metadata_file and successful > 0:
            metadata_path = os.path.join(album_dir, self.METADATA_FILENAME)
            self._write_metadata_file(metadata_path, album, album_result)

        return album_result

    # -- Filesystem layout ---------------------------------------------------

    def _build_album_folder(self, album: Album) -> str:
        info = self._album_format_info(album)
        try:
            folder = self._config.folder_format.format(**info)
        except (KeyError, IndexError):
            folder = f"{info['albumartist']} - {info['title']}"
        folder = "/".join(_safe_value(part) for part in folder.split("/") if part)
        base = self._config.output_dir
        if self._config.source_subdirectories:
            base = os.path.join(base, "Tidal")
        return os.path.join(base, folder)

    def _build_track_filename(self, track: Track, album: Album) -> str:
        info = self._track_format_info(track, album)
        try:
            name = self._config.track_format.format(**info)
        except (KeyError, IndexError):
            name = f"{info['tracknumber']:02} - {info['title']}"
        return _safe_filename(name)

    def _album_format_info(self, album: Album) -> dict[str, Any]:
        year = (album.release_date or "")[:4] or "0000"
        container, bit_depth, sampling_rate = _tidal_quality_fields(
            album.audio_quality, self._config.quality
        )
        return {
            "albumartist": _safe_value(_build_albumartist(album)),
            "title": _safe_value(album.title),
            "year": year,
            "container": container,
            "bit_depth": bit_depth,
            "sampling_rate": sampling_rate,
            "id": album.id,
            "albumcomposer": _safe_value(_build_albumartist(album)),
        }

    def _track_format_info(self, track: Track, album: Album) -> dict[str, Any]:
        return {
            "tracknumber": track.track_number,
            "discnumber": track.volume_number,
            "title": _safe_value(track.title),
            "artist": _safe_value(track.artist.name),
            "albumartist": _safe_value(_build_albumartist(album)),
            "explicit": " (Explicit)" if track.explicit else "",
            "composer": "",
            "id": track.id,
        }

    # -- Cover art -----------------------------------------------------------

    async def _download_cover(self, album: Album, album_dir: str) -> str | None:
        if not self._config.embed_cover or not album.cover:
            return None
        size_map = {"small": "320x320", "medium": "640x640", "large": "1280x1280"}
        size = size_map.get(self._config.cover_size, self._config.cover_size)
        url = (
            f"https://resources.tidal.com/images/{album.cover.replace('-', '/')}/{size}.jpg"
        )
        cover_path = os.path.join(album_dir, "cover.jpg")
        try:
            session = await self._client._transport.session()
            async with session.get(url) as resp:
                if resp.status >= 400:
                    return None
                async with aiofiles.open(cover_path, "wb") as f:
                    await f.write(await resp.read())
            return cover_path
        except Exception as exc:
            logger.warning("Failed to download cover for album %s: %s", album.id, exc)
            return None

    # -- Per-track download --------------------------------------------------

    async def _download_one_track(
        self,
        track: Track,
        album: Album,
        album_dir: str,
        cover_path: str | None,
    ) -> TrackResult:
        track_num = track.track_number or 0
        title = track.title

        if self._on_track_start is not None:
            self._on_track_start(track_num, title)

        try:
            manifest = await self._streaming.get_manifest(
                track.id, quality=self._config.quality
            )
        except NonStreamableError as exc:
            if self._on_track_complete is not None:
                self._on_track_complete(track_num, title, False)
            return TrackResult(
                track_id=track.id, title=title, success=False, error=str(exc)
            )
        except Exception as exc:
            if self._on_track_complete is not None:
                self._on_track_complete(track_num, title, False)
            return TrackResult(
                track_id=track.id, title=title, success=False, error=str(exc)
            )

        target = self._track_target_path(track, album, album_dir, manifest)

        if self._config.skip_downloaded and os.path.exists(target):
            if self._on_track_complete is not None:
                self._on_track_complete(track_num, title, True)
            return TrackResult(
                track_id=track.id, title=title, success=True, file_path=target
            )

        try:
            os.makedirs(os.path.dirname(target), exist_ok=True)
            await self._download_file(manifest, target, track_num)
            # Tidal's PKCE/DASH manifests deliver FLAC inside a fragmented
            # MP4 container (``.flac`` extension but mp4 magic bytes). For
            # the file to round-trip through mutagen and look like FLAC to
            # strict scanners, remux to a real native FLAC if ffmpeg is on
            # PATH. Best-effort — if ffmpeg isn't available we keep the
            # mp4-wrapped file (still plays in mpv/VLC/foobar2000).
            tag_safe = True
            if manifest.is_dash and manifest.file_extension == "flac":
                tag_safe = await _remux_mp4_to_flac(target)
            # Skip tagging on un-remuxed DASH FLAC: mutagen.flac would
            # raise FLACNoHeaderError on the MP4 magic bytes, the outer
            # handler would delete the file, and the user would see a
            # successful download silently fail. Better to keep the file
            # untagged than to lose it.
            if self._config.tag_files and tag_safe:
                self._tag_file(target, track, album, cover_path, manifest)
            elif self._config.tag_files and not tag_safe:
                logger.warning(
                    "Skipping tag write for %s — file is MP4-wrapped FLAC "
                    "(no ffmpeg available to remux). Install ffmpeg or "
                    "tag manually.",
                    os.path.basename(target),
                )
        except Exception as exc:
            logger.exception("Download failed for track %s", track.id)
            # Clean up both the final target and the ``.enc`` temp (if any)
            # so a failed download never leaves half-written files behind.
            for leftover in (target, target + ".enc"):
                if os.path.exists(leftover):
                    try:
                        os.remove(leftover)
                    except OSError:
                        pass
            if self._on_track_complete is not None:
                self._on_track_complete(track_num, title, False)
            return TrackResult(
                track_id=track.id, title=title, success=False, error=str(exc)
            )

        if self._on_track_complete is not None:
            self._on_track_complete(track_num, title, True)
        return TrackResult(
            track_id=track.id, title=title, success=True, file_path=target
        )

    def _track_target_path(
        self, track: Track, album: Album, album_dir: str, manifest: StreamManifest
    ) -> str:
        filename = self._build_track_filename(track, album)
        ext = manifest.file_extension
        # Multi-disc albums get a per-disc subdirectory. Uses ``Disc N`` to
        # match streamrip's historical layout so upgrades from streamrip to
        # this SDK don't re-download an already-organized library.
        if self._config.disc_subdirectories and album.number_of_volumes > 1:
            disc_dir = os.path.join(album_dir, f"Disc {track.volume_number}")
            return os.path.join(disc_dir, f"{filename}.{ext}")
        return os.path.join(album_dir, f"{filename}.{ext}")

    async def _download_file(
        self, manifest: StreamManifest, target_path: str, track_num: int,
        retries: int = 5,
    ) -> None:
        """Stream the track to disk.

        Three flavours, picked by the manifest:
        - **DASH** (``manifest.is_dash``) — concatenate every segment URL into
          the target file in order. Single open file, append per segment.
        - **BTS encrypted** — single download into ``<target>.enc``, then
          AES-CTR decrypt to ``<target>``.
        - **BTS plaintext** — single download straight to ``<target>``.

        Each per-URL fetch is retried with exponential backoff (2/4/8/16s)
        on the usual Tidal CDN flakiness. Decrypt failures aren't retried
        (a bad key won't fix itself).
        """
        if manifest.is_dash:
            await self._download_dash(manifest, target_path, track_num, retries)
            return

        chunk_size = 2**17  # 128 KiB
        encrypted_path = target_path if not manifest.is_encrypted else target_path + ".enc"

        try:
            # ── Download with retry ──
            last_exc: Exception | None = None
            for attempt in range(retries):
                try:
                    session = await self._client._transport.session()
                    async with session.get(manifest.url) as resp:
                        if resp.status >= 400:
                            raise RuntimeError(
                                f"download HTTP {resp.status} for track {manifest.track_id}"
                            )
                        total = int(resp.headers.get("Content-Length") or 0)
                        bytes_done = 0
                        async with aiofiles.open(encrypted_path, "wb") as out:
                            async for chunk in resp.content.iter_chunked(chunk_size):
                                await out.write(chunk)
                                bytes_done += len(chunk)
                                if self._on_track_progress is not None:
                                    self._on_track_progress(track_num, bytes_done, total)
                    last_exc = None
                    break  # success
                except Exception as e:
                    last_exc = e
                    if attempt < retries - 1:
                        backoff = 2 * (2 ** attempt)  # 2, 4, 8, 16s
                        logger.warning(
                            "Tidal download attempt %d/%d failed for track %s, "
                            "retrying in %ds: %s",
                            attempt + 1, retries, manifest.track_id, backoff, e,
                        )
                        # Drop the partial file before retrying
                        if os.path.exists(encrypted_path):
                            try:
                                os.remove(encrypted_path)
                            except OSError:
                                pass
                        await asyncio.sleep(backoff)
            if last_exc is not None:
                logger.error(
                    "Tidal download failed for track %s after %d attempts: %s",
                    manifest.track_id, retries, last_exc,
                )
                raise last_exc

            # ── Decrypt (no retry — bad key won't fix itself) ──
            if manifest.is_encrypted and manifest.encryption_key:
                _decrypt_mqa(encrypted_path, target_path, manifest.encryption_key)
        finally:
            # Remove the temp file on every path — success (after decrypt),
            # failed decrypt, or failed download.
            if (
                manifest.is_encrypted
                and encrypted_path != target_path
                and os.path.exists(encrypted_path)
            ):
                try:
                    os.remove(encrypted_path)
                except OSError:
                    pass

    async def _download_dash(
        self,
        manifest: StreamManifest,
        target_path: str,
        track_num: int,
        retries: int,
    ) -> None:
        """Download a Tidal DASH track by concatenating its segments.

        Sequential rather than parallel: total bytes are unknown up-front
        (HEAD on a Tidal segment URL is unreliable), and concatenation
        order is load-bearing — the init segment must precede the media
        segments. Progress is approximated by segment index since we
        can't know byte totals before fetching.
        """
        chunk_size = 2**17  # 128 KiB
        urls = manifest.urls or [manifest.url]
        total_segments = len(urls)
        bytes_done = 0

        # Best-effort progress: scale segments to a "fake" byte total so
        # the UI shows a moving bar. The real byte count comes after.
        fake_total = max(total_segments, 1)

        try:
            async with aiofiles.open(target_path, "wb") as out:
                for idx, url in enumerate(urls):
                    # Snapshot the file position *before* this segment so a
                    # mid-stream failure can be rolled back on retry. Without
                    # this, a partial write followed by a successful retry
                    # appends a duplicate prefix and corrupts the output.
                    segment_start = await out.tell()
                    last_exc: Exception | None = None
                    for attempt in range(retries):
                        try:
                            session = await self._client._transport.session()
                            async with session.get(url) as resp:
                                if resp.status >= 400:
                                    raise RuntimeError(
                                        f"DASH segment HTTP {resp.status} "
                                        f"(track {manifest.track_id}, segment {idx})"
                                    )
                                async for chunk in resp.content.iter_chunked(chunk_size):
                                    await out.write(chunk)
                                    bytes_done += len(chunk)
                            last_exc = None
                            break
                        except Exception as e:
                            last_exc = e
                            if attempt < retries - 1:
                                backoff = 2 * (2**attempt)
                                logger.warning(
                                    "Tidal DASH segment %d/%d retry %d/%d for "
                                    "track %s in %ds: %s",
                                    idx, total_segments, attempt + 1, retries,
                                    manifest.track_id, backoff, e,
                                )
                                # Roll back any partial bytes from the failed
                                # attempt so the retry writes from a clean offset.
                                bytes_done -= max(0, await out.tell() - segment_start)
                                await out.seek(segment_start)
                                await out.truncate()
                                await asyncio.sleep(backoff)
                    if last_exc is not None:
                        logger.error(
                            "Tidal DASH download failed for track %s on segment %d: %s",
                            manifest.track_id, idx, last_exc,
                        )
                        raise last_exc
                    if self._on_track_progress is not None:
                        # Treat segment progress as fractional bytes-of-fake-total;
                        # the real progress callback expects (done, total) in bytes.
                        self._on_track_progress(track_num, idx + 1, fake_total)
        except Exception:
            if os.path.exists(target_path):
                try:
                    os.remove(target_path)
                except OSError:
                    pass
            raise

    # -- Tagging -------------------------------------------------------------

    def _tag_file(
        self,
        path: str,
        track: Track,
        album: Album,
        cover_path: str | None,
        manifest: StreamManifest,
    ) -> None:
        """Write metadata tags into the audio file using mutagen."""
        ext = path.rsplit(".", 1)[-1].lower()
        if ext == "flac":
            self._tag_flac(path, track, album, cover_path)
        elif ext == "m4a":
            self._tag_m4a(path, track, album, cover_path)

    def _tag_flac(
        self, path: str, track: Track, album: Album, cover_path: str | None
    ) -> None:
        from mutagen.flac import FLAC, Picture

        audio = FLAC(path)
        audio["title"] = track.title
        audio["artist"] = track.artist.name
        audio["albumartist"] = _build_albumartist(album)
        audio["album"] = album.title
        audio["tracknumber"] = _zero_pad(track.track_number)
        audio["discnumber"] = _zero_pad(track.volume_number)
        audio["totaltracks"] = str(album.number_of_tracks)
        if album.release_date:
            audio["date"] = album.release_date
            audio["year"] = album.release_date[:4]
        if track.isrc:
            audio["isrc"] = track.isrc
        if album.copyright:
            audio["copyright"] = album.copyright

        if cover_path and os.path.exists(cover_path):
            pic = Picture()
            pic.type = 3  # Cover (front)
            pic.mime = "image/jpeg"
            with open(cover_path, "rb") as cf:
                pic.data = cf.read()
            audio.clear_pictures()
            audio.add_picture(pic)

        audio.save()

    def _tag_m4a(
        self, path: str, track: Track, album: Album, cover_path: str | None
    ) -> None:
        from mutagen.mp4 import MP4, MP4Cover

        audio = MP4(path)
        audio["\xa9nam"] = track.title
        audio["\xa9ART"] = track.artist.name
        audio["aART"] = _build_albumartist(album)
        audio["\xa9alb"] = album.title
        audio["trkn"] = [(track.track_number, album.number_of_tracks)]
        audio["disk"] = [(track.volume_number, album.number_of_volumes)]
        if album.release_date:
            audio["\xa9day"] = album.release_date

        if cover_path and os.path.exists(cover_path):
            with open(cover_path, "rb") as cf:
                audio["covr"] = [MP4Cover(cf.read(), imageformat=MP4Cover.FORMAT_JPEG)]

        audio.save()

    # -- Metadata sentinel --------------------------------------------------

    METADATA_FILENAME = ".streamrip.json"
    """Sentinel filename written next to the audio files for filesystem-based
    dedup and scan reconciliation. Same name as the Qobuz SDK so a single
    scanner (e.g. ``qobuz.downloader.AlbumDownloader.scan_downloaded_albums``)
    can pick up albums from either source — they're disambiguated by the
    ``source`` field in the payload."""

    def _write_metadata_file(
        self,
        path: str,
        album: Album,
        result: AlbumResult | None = None,
    ) -> None:
        try:
            payload: dict[str, Any] = {
                "source": "tidal",
                "album_id": album.id,
                "title": album.title,
                "artist": album.artist.name,
                "release_date": album.release_date,
                # Match the Qobuz SDK key name so the shared scanner picks
                # this up.  ``number_of_tracks`` kept as a Tidal-specific
                # alias for backwards compatibility.
                "tracks_count": album.number_of_tracks,
                "number_of_tracks": album.number_of_tracks,
                "quality": self._config.quality,
                "downloaded_at": time.strftime("%Y-%m-%dT%H:%M:%S"),
            }
            if result is not None:
                payload["tracks_downloaded"] = result.successful
                payload["tracks"] = [
                    {
                        "id": t.track_id,
                        "title": t.title,
                        "success": t.success,
                        "path": (
                            os.path.basename(t.file_path) if t.file_path else None
                        ),
                    }
                    for t in result.tracks
                ]
            with open(path, "w", encoding="utf-8") as f:
                json.dump(payload, f, ensure_ascii=False, indent=2)
        except OSError as exc:
            logger.warning("Failed to write %s: %s", path, exc)
