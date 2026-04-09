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
from pathlib import Path
from typing import Any, Callable

import aiofiles
import aiohttp

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

        # Optional: write metadata sentinel for filesystem-based dedup
        metadata_path = os.path.join(album_dir, ".tidal.json")
        if self._config.write_metadata_file:
            self._write_metadata_file(metadata_path, album)

        cover_path = await self._download_cover(album, album_dir)

        sem = asyncio.Semaphore(self._config.max_connections)
        results: list[TrackResult] = []

        async def _wrapped(track: Track) -> TrackResult:
            async with sem:
                return await self._download_one_track(
                    track, album, album_dir, cover_path
                )

        for tr in await asyncio.gather(*(_wrapped(t) for t in tracks)):
            results.append(tr)

        successful = sum(1 for r in results if r.success)
        return AlbumResult(
            album_id=album.id,
            title=album.title,
            artist=album.artist.name,
            total=len(tracks),
            successful=successful,
            tracks=results,
        )

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
        return {
            "albumartist": _safe_value(_build_albumartist(album)),
            "title": _safe_value(album.title),
            "year": year,
            "container": "FLAC",
            "bit_depth": 16 if album.audio_quality == "LOSSLESS" else 24,
            "sampling_rate": "44.1" if album.audio_quality == "LOSSLESS" else "96",
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
            if self._config.tag_files:
                self._tag_file(target, track, album, cover_path, manifest)
        except Exception as exc:
            logger.exception("Download failed for track %s", track.id)
            if os.path.exists(target):
                try:
                    os.remove(target)
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
        # Optional disc subdir for multi-disc albums
        if self._config.disc_subdirectories and album.number_of_volumes > 1:
            disc_dir = os.path.join(album_dir, f"CD{track.volume_number:02d}")
            return os.path.join(disc_dir, f"{filename}.{ext}")
        return os.path.join(album_dir, f"{filename}.{ext}")

    async def _download_file(
        self, manifest: StreamManifest, target_path: str, track_num: int
    ) -> None:
        """Stream the file to disk and decrypt if needed."""
        session = await self._client._transport.session()
        chunk_size = 2**17  # 128 KiB

        # If encrypted, write to a temp path first then decrypt over the top.
        encrypted_path = target_path if not manifest.is_encrypted else target_path + ".enc"

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

        if manifest.is_encrypted and manifest.encryption_key:
            try:
                _decrypt_mqa(encrypted_path, target_path, manifest.encryption_key)
            finally:
                if encrypted_path != target_path and os.path.exists(encrypted_path):
                    try:
                        os.remove(encrypted_path)
                    except OSError:
                        pass

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

    def _write_metadata_file(self, path: str, album: Album) -> None:
        try:
            payload = {
                "source": "tidal",
                "album_id": album.id,
                "title": album.title,
                "artist": album.artist.name,
                "release_date": album.release_date,
                "number_of_tracks": album.number_of_tracks,
                "downloaded_at": time.strftime("%Y-%m-%dT%H:%M:%S"),
            }
            with open(path, "w", encoding="utf-8") as f:
                json.dump(payload, f, ensure_ascii=False, indent=2)
        except OSError as exc:
            logger.warning("Failed to write %s: %s", path, exc)
