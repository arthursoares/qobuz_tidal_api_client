"""Retry / backoff tests for AlbumDownloader._download_file.

The Qobuz CDN occasionally closes connections mid-stream, surfacing as
``aiohttp.ClientPayloadError`` / ``ContentLengthError``.  The downloader
retries up to 5 times with exponential backoff (2/4/8/16s).  These tests
exercise that path with a fake aiohttp.ClientSession that fails N times
then succeeds.
"""

from __future__ import annotations

import os
from unittest.mock import AsyncMock, MagicMock, patch

import aiohttp
import pytest

from qobuz.downloader import AlbumDownloader, DownloadConfig


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_downloader(tmp_path) -> AlbumDownloader:
    client = MagicMock()
    client.streaming = MagicMock()
    config = DownloadConfig(
        output_dir=str(tmp_path),
        max_connections=1,
        skip_downloaded=False,
        downloads_db_path=None,
    )
    return AlbumDownloader(client, config)


class _ChunkIter:
    """Async iterator that either yields chunks or raises mid-stream."""

    def __init__(self, chunks: list[bytes], raise_after: int | None = None):
        self._chunks = chunks
        self._idx = 0
        self._raise_after = raise_after

    def __aiter__(self):
        return self

    async def __anext__(self):
        if self._raise_after is not None and self._idx >= self._raise_after:
            raise aiohttp.ClientPayloadError(
                "Response payload is not completed: ContentLengthError"
            )
        if self._idx >= len(self._chunks):
            raise StopAsyncIteration
        chunk = self._chunks[self._idx]
        self._idx += 1
        return chunk


def _fake_response(
    chunks: list[bytes],
    raise_after: int | None = None,
    content_length: int | None = None,
):
    """Build a fake aiohttp response context manager."""
    resp_obj = MagicMock()
    resp_obj.status = 200
    resp_obj.raise_for_status = MagicMock()
    total = (
        content_length
        if content_length is not None
        else sum(len(c) for c in chunks)
    )
    resp_obj.headers = {"Content-Length": str(total)}
    resp_obj.content = MagicMock()
    resp_obj.content.iter_chunked = lambda size: _ChunkIter(chunks, raise_after)

    cm = MagicMock()
    cm.__aenter__ = AsyncMock(return_value=resp_obj)
    cm.__aexit__ = AsyncMock(return_value=None)
    return cm


def _failing_then_succeeding_session(fail_n: int, success_chunks: list[bytes]):
    """Build a fake ClientSession that fails ``fail_n`` times then succeeds.

    Each failure raises mid-stream after writing some bytes (so the
    partial-file cleanup branch fires).
    """
    call_count = {"n": 0}

    def get(url):
        call_count["n"] += 1
        if call_count["n"] <= fail_n:
            # Fail mid-stream after the first chunk so the partial file
            # exists at the moment the exception is raised.
            return _fake_response([b"partial"], raise_after=1, content_length=999)
        return _fake_response(success_chunks)

    session = MagicMock()
    session.get = get
    session.__aenter__ = AsyncMock(return_value=session)
    session.__aexit__ = AsyncMock(return_value=None)
    return session, call_count


def _patch_aiohttp_session(session):
    """Patch ``aiohttp.ClientSession`` in the qobuz.downloader module."""
    return patch("qobuz.downloader.aiohttp.ClientSession", return_value=session)


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------


class TestSucceedsImmediately:
    async def test_writes_full_file_on_first_attempt(self, tmp_path):
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        chunks = [b"hello ", b"world", b"!"]
        session, calls = _failing_then_succeeding_session(0, chunks)

        with _patch_aiohttp_session(session), patch(
            "qobuz.downloader.asyncio.sleep", new_callable=AsyncMock
        ) as sleep_mock:
            await dl._download_file("http://x/audio", target, track_num=1)

        assert calls["n"] == 1
        assert open(target, "rb").read() == b"hello world!"
        sleep_mock.assert_not_called()

    async def test_progress_callback_fires_per_chunk(self, tmp_path):
        dl = _make_downloader(tmp_path)
        progress_log: list[tuple[int, int, int]] = []
        dl._on_track_progress = (
            lambda num, done, total: progress_log.append((num, done, total))
        )
        target = str(tmp_path / "track.flac")
        chunks = [b"abc", b"de", b"fgh"]  # 3 + 2 + 3 = 8 bytes
        session, _ = _failing_then_succeeding_session(0, chunks)

        with _patch_aiohttp_session(session), patch(
            "qobuz.downloader.asyncio.sleep", new_callable=AsyncMock
        ):
            await dl._download_file("http://x/audio", target, track_num=7)

        # One callback per chunk, monotonically increasing bytes_done
        assert [done for _, done, _ in progress_log] == [3, 5, 8]
        assert all(num == 7 for num, _, _ in progress_log)
        assert all(total == 8 for _, _, total in progress_log)


# ---------------------------------------------------------------------------
# Retry behaviour
# ---------------------------------------------------------------------------


class TestRetryRecovers:
    async def test_recovers_after_two_transient_failures(self, tmp_path):
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        success_chunks = [b"final ", b"content"]
        session, calls = _failing_then_succeeding_session(2, success_chunks)

        with _patch_aiohttp_session(session), patch(
            "qobuz.downloader.asyncio.sleep", new_callable=AsyncMock
        ) as sleep_mock:
            await dl._download_file("http://x", target, track_num=1)

        # 2 failures + 1 success
        assert calls["n"] == 3
        assert open(target, "rb").read() == b"final content"
        # Slept twice (after attempt 1 and attempt 2), with exponential backoff
        assert sleep_mock.call_count == 2
        assert sleep_mock.call_args_list[0].args == (2,)
        assert sleep_mock.call_args_list[1].args == (4,)

    async def test_partial_file_cleaned_up_between_retries(self, tmp_path):
        """Partial file from a failed attempt must be removed before retrying.

        Otherwise the retry would resume mid-file or include garbage from the
        previous attempt.
        """
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        # Fake will write 'partial' on the failing attempt, then 'good' on success
        success_chunks = [b"good"]
        session, _ = _failing_then_succeeding_session(1, success_chunks)

        with _patch_aiohttp_session(session), patch(
            "qobuz.downloader.asyncio.sleep", new_callable=AsyncMock
        ):
            await dl._download_file("http://x", target, track_num=1)

        # Final file should be the success chunks only — no leftover bytes
        # from the failed attempt
        assert open(target, "rb").read() == b"good"


class TestRetryExhausted:
    async def test_raises_after_all_retries_exhausted(self, tmp_path):
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        # Fail more times than the retry limit (5)
        session, calls = _failing_then_succeeding_session(99, [b"unused"])

        with _patch_aiohttp_session(session), patch(
            "qobuz.downloader.asyncio.sleep", new_callable=AsyncMock
        ) as sleep_mock:
            with pytest.raises(aiohttp.ClientPayloadError):
                await dl._download_file("http://x", target, track_num=1, retries=5)

        # 5 attempts total, 4 sleeps between them (2, 4, 8, 16)
        assert calls["n"] == 5
        assert sleep_mock.call_count == 4
        assert [c.args[0] for c in sleep_mock.call_args_list] == [2, 4, 8, 16]
        # Partial file from the last failed attempt is left on disk by the
        # final raise — verify it's not the success contents
        assert not os.path.exists(target) or open(target, "rb").read() != b"unused"

    async def test_exponential_backoff_doubles_each_attempt(self, tmp_path):
        """Backoff should follow 2, 4, 8, 16 (powers of two starting at 2)."""
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        session, _ = _failing_then_succeeding_session(99, [])

        with _patch_aiohttp_session(session), patch(
            "qobuz.downloader.asyncio.sleep", new_callable=AsyncMock
        ) as sleep_mock:
            with pytest.raises(aiohttp.ClientPayloadError):
                await dl._download_file("http://x", target, track_num=1, retries=5)

        sleeps = [c.args[0] for c in sleep_mock.call_args_list]
        # Each sleep should be exactly double the previous one
        assert sleeps == [2, 4, 8, 16]
        for i in range(1, len(sleeps)):
            assert sleeps[i] == sleeps[i - 1] * 2

    async def test_retries_default_is_five(self, tmp_path):
        """The default retry count should be 5 (was 2 before the fix)."""
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        session, calls = _failing_then_succeeding_session(99, [])

        with _patch_aiohttp_session(session), patch(
            "qobuz.downloader.asyncio.sleep", new_callable=AsyncMock
        ):
            with pytest.raises(aiohttp.ClientPayloadError):
                # No `retries` kwarg → should use the default
                await dl._download_file("http://x", target, track_num=1)

        assert calls["n"] == 5  # Confirms default retries=5


# ---------------------------------------------------------------------------
# Mid-flight failures don't pollute the output file
# ---------------------------------------------------------------------------


class TestPartialCleanup:
    async def test_no_leftover_on_disk_when_first_attempt_fails(self, tmp_path):
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        success_chunks = [b"clean"]
        session, _ = _failing_then_succeeding_session(1, success_chunks)

        with _patch_aiohttp_session(session), patch(
            "qobuz.downloader.asyncio.sleep", new_callable=AsyncMock
        ):
            await dl._download_file("http://x", target, track_num=1)

        # The 'partial' bytes from the failed first attempt must NOT appear
        # anywhere in the final file
        contents = open(target, "rb").read()
        assert b"partial" not in contents
        assert contents == b"clean"
