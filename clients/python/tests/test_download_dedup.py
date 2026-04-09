"""Filesystem dedup tests for AlbumDownloader.

Covers ``DownloadConfig.skip_downloaded`` + ``downloads_db_path``.

The dedup DB is a tiny SQLite file with one column (track id).  On
start, the downloader loads the existing IDs into a set; per-track,
it short-circuits the download if the track is in that set.  After
a successful download, ``_mark_downloaded`` adds the track ID so a
subsequent run skips it.

These tests cover the SDK in isolation — see also the backend's
test_download_service tests for the per-source DB filename
disambiguation (downloads.db vs downloads-tidal.db).
"""

from __future__ import annotations

import os
import sqlite3
from unittest.mock import AsyncMock, MagicMock

import pytest

from qobuz.downloader import AlbumDownloader, DownloadConfig


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_downloader(
    tmp_path,
    *,
    skip_downloaded: bool = True,
    downloads_db_path: str | None = None,
) -> AlbumDownloader:
    client = MagicMock()
    client.streaming = MagicMock()
    config = DownloadConfig(
        output_dir=str(tmp_path),
        max_connections=1,
        skip_downloaded=skip_downloaded,
        downloads_db_path=downloads_db_path,
    )
    return AlbumDownloader(client, config)


def _seed_dedup_db(path: str, track_ids: list[int | str]) -> None:
    """Create a downloads DB with the given track IDs."""
    os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
    conn = sqlite3.connect(path)
    conn.execute("CREATE TABLE IF NOT EXISTS downloads (id TEXT UNIQUE NOT NULL)")
    for tid in track_ids:
        conn.execute("INSERT OR IGNORE INTO downloads (id) VALUES (?)", (str(tid),))
    conn.commit()
    conn.close()


# ---------------------------------------------------------------------------
# _load_downloaded_ids
# ---------------------------------------------------------------------------


class TestLoadDownloadedIds:
    def test_loads_existing_ids_into_set(self, tmp_path):
        db_path = str(tmp_path / "downloads.db")
        _seed_dedup_db(db_path, [101, 102, 103])

        dl = _make_downloader(tmp_path, downloads_db_path=db_path)

        # __init__ should auto-load when skip_downloaded=True + path is set
        assert dl._downloaded_ids == {"101", "102", "103"}

    def test_does_not_load_when_skip_downloaded_disabled(self, tmp_path):
        db_path = str(tmp_path / "downloads.db")
        _seed_dedup_db(db_path, [101, 102])

        dl = _make_downloader(
            tmp_path, skip_downloaded=False, downloads_db_path=db_path
        )

        # Empty: load is gated on skip_downloaded
        assert dl._downloaded_ids == set()

    def test_does_not_load_when_path_missing(self, tmp_path):
        # No path → no load, no error
        dl = _make_downloader(tmp_path, downloads_db_path=None)
        assert dl._downloaded_ids == set()

    def test_handles_nonexistent_db_file_gracefully(self, tmp_path):
        # Path is set but the file doesn't exist (first ever run)
        db_path = str(tmp_path / "never_created.db")
        dl = _make_downloader(tmp_path, downloads_db_path=db_path)
        assert dl._downloaded_ids == set()
        # File was NOT created — _load_downloaded_ids returns early
        assert not os.path.exists(db_path)

    def test_handles_corrupted_db_file_gracefully(self, tmp_path):
        # A non-SQLite file at the path shouldn't crash __init__
        db_path = str(tmp_path / "downloads.db")
        with open(db_path, "wb") as f:
            f.write(b"this is not a sqlite file")
        # Should swallow the exception and start with an empty set
        dl = _make_downloader(tmp_path, downloads_db_path=db_path)
        assert dl._downloaded_ids == set()


# ---------------------------------------------------------------------------
# _mark_downloaded
# ---------------------------------------------------------------------------


class TestMarkDownloaded:
    def test_creates_db_and_inserts_id(self, tmp_path):
        db_path = str(tmp_path / "subdir" / "downloads.db")  # nested dir
        dl = _make_downloader(tmp_path, downloads_db_path=db_path)

        dl._mark_downloaded(42)

        assert os.path.exists(db_path)
        conn = sqlite3.connect(db_path)
        rows = conn.execute("SELECT id FROM downloads").fetchall()
        conn.close()
        assert ("42",) in rows
        # Also added to the in-memory set
        assert "42" in dl._downloaded_ids

    def test_duplicate_inserts_are_ignored(self, tmp_path):
        db_path = str(tmp_path / "downloads.db")
        dl = _make_downloader(tmp_path, downloads_db_path=db_path)

        dl._mark_downloaded(99)
        dl._mark_downloaded(99)
        dl._mark_downloaded(99)

        conn = sqlite3.connect(db_path)
        rows = conn.execute("SELECT id FROM downloads").fetchall()
        conn.close()
        # Single row despite three inserts
        assert rows == [("99",)]

    def test_no_op_when_path_missing(self, tmp_path):
        dl = _make_downloader(tmp_path, downloads_db_path=None)
        # Should not raise
        dl._mark_downloaded(123)
        assert dl._downloaded_ids == set()

    def test_marks_persist_across_instances(self, tmp_path):
        """A second downloader pointed at the same DB sees the first one's marks."""
        db_path = str(tmp_path / "downloads.db")

        dl_a = _make_downloader(tmp_path, downloads_db_path=db_path)
        dl_a._mark_downloaded(1)
        dl_a._mark_downloaded(2)
        dl_a._mark_downloaded(3)

        dl_b = _make_downloader(tmp_path, downloads_db_path=db_path)
        assert dl_b._downloaded_ids == {"1", "2", "3"}


# ---------------------------------------------------------------------------
# Skip path inside _download_track
# ---------------------------------------------------------------------------


def _fake_track(track_id: int, title: str = "Track"):
    """Build a stand-in for qobuz.types.Track with the fields the dedup
    path actually reads (id, title)."""
    track = MagicMock()
    track.id = track_id
    track.title = title
    return track


class TestSkipDownloaded:
    async def test_already_downloaded_track_is_short_circuited(self, tmp_path):
        """If a track ID is in _downloaded_ids and skip_downloaded=True,
        _download_track must return success without hitting the SDK at all."""
        db_path = str(tmp_path / "downloads.db")
        _seed_dedup_db(db_path, [555])
        dl = _make_downloader(tmp_path, downloads_db_path=db_path)

        completes: list[tuple[int, str, bool]] = []
        dl._on_track_complete = (
            lambda n, t, ok: completes.append((n, t, ok))
        )
        starts: list[int] = []
        dl._on_track_start = lambda n, t: starts.append(n)

        # The streaming API call must NEVER be made for a skipped track
        dl.client.streaming.get_file_url = AsyncMock(
            side_effect=AssertionError(
                "get_file_url should not be called for a skipped track"
            )
        )

        track = _fake_track(555, "Already Have This")
        result = await dl._download_track(
            track_num=1,
            track=track,
            album=MagicMock(),
            album_folder=str(tmp_path),
            cover_path=None,
        )

        assert result.success is True
        assert "skipped" in (result.error or "")
        assert completes == [(1, "Already Have This", True)]
        assert starts == []  # start NOT fired for a skipped track

    async def test_skip_disabled_means_track_is_attempted(self, tmp_path):
        """When skip_downloaded=False, even an in-set track should be re-downloaded."""
        db_path = str(tmp_path / "downloads.db")
        _seed_dedup_db(db_path, [555])

        # skip_downloaded=False: the load is gated, so the set stays empty
        dl = _make_downloader(
            tmp_path, skip_downloaded=False, downloads_db_path=db_path
        )
        assert dl._downloaded_ids == set()

        # Force a controlled failure so the test exits fast — the point is
        # that get_file_url IS called (no short-circuit)
        dl.client.streaming.get_file_url = AsyncMock(
            side_effect=RuntimeError("network would have been called")
        )
        dl._on_track_start = MagicMock()
        dl._on_track_complete = MagicMock()

        track = _fake_track(555, "Re-download me")
        result = await dl._download_track(
            track_num=1,
            track=track,
            album=MagicMock(),
            album_folder=str(tmp_path),
            cover_path=None,
        )

        # Failed because we forced an error AFTER get_file_url was called.
        # The important part: get_file_url WAS called.
        dl.client.streaming.get_file_url.assert_called_once()
        assert result.success is False
        # And on_track_start fired (as opposed to skip path which doesn't fire it)
        dl._on_track_start.assert_called_once_with(1, "Re-download me")
