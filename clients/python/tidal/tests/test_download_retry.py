"""Retry / backoff tests for Tidal AlbumDownloader._download_file.

The Tidal CDN occasionally closes connections mid-stream, surfacing as
``aiohttp.ClientPayloadError`` / ``ContentLengthError``.  The downloader
retries up to 5 times with exponential backoff (2/4/8/16s).  These tests
exercise that path with a fake session that fails N times then succeeds.

In addition to the basic retry behaviour, Tidal also has the encrypted
``.enc`` temp-file cleanup path on top of every retry attempt — verify
those interact correctly.
"""

from __future__ import annotations

import os
from unittest.mock import AsyncMock, MagicMock, patch

import aiohttp
import pytest

from tidal.downloader import AlbumDownloader, DownloadConfig
from tidal.types import StreamManifest


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_downloader(tmp_path) -> AlbumDownloader:
    client = MagicMock()
    client.catalog = MagicMock()
    client.streaming = MagicMock()
    config = DownloadConfig(
        output_dir=str(tmp_path),
        folder_format="{albumartist}/{title}",
        track_format="{tracknumber:02} - {title}",
        tag_files=False,
    )
    return AlbumDownloader(client, config)


def _manifest(*, encrypted: bool = False) -> StreamManifest:
    if encrypted:
        return StreamManifest(
            track_id=1,
            audio_quality="HI_RES",
            codec="mqa",
            url="http://fake/audio",
            encryption_type="OLD_AES",
            encryption_key="A" * 24,  # base64-ish placeholder
        )
    return StreamManifest(
        track_id=1,
        audio_quality="LOSSLESS",
        codec="flac",
        url="http://fake/audio",
    )


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
    status: int = 200,
):
    resp_obj = MagicMock()
    resp_obj.status = status
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


def _wire_session(dl: AlbumDownloader, fail_n: int, success_chunks: list[bytes]):
    """Install a fake transport.session() that fails ``fail_n`` times then succeeds."""
    call_count = {"n": 0}

    def get(url):
        call_count["n"] += 1
        if call_count["n"] <= fail_n:
            return _fake_response([b"partial"], raise_after=1, content_length=999)
        return _fake_response(success_chunks)

    fake_session = MagicMock()
    fake_session.get = get
    dl._client._transport.session = AsyncMock(return_value=fake_session)
    return call_count


# ---------------------------------------------------------------------------
# Happy path
# ---------------------------------------------------------------------------


class TestSucceedsImmediately:
    async def test_writes_full_file_on_first_attempt(self, tmp_path):
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        chunks = [b"hello ", b"world", b"!"]
        calls = _wire_session(dl, 0, chunks)

        with patch(
            "tidal.downloader.asyncio.sleep", new_callable=AsyncMock
        ) as sleep_mock:
            await dl._download_file(_manifest(), target, track_num=1)

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
        chunks = [b"abc", b"de", b"fgh"]
        _wire_session(dl, 0, chunks)

        with patch("tidal.downloader.asyncio.sleep", new_callable=AsyncMock):
            await dl._download_file(_manifest(), target, track_num=7)

        assert [done for _, done, _ in progress_log] == [3, 5, 8]
        assert all(num == 7 for num, _, _ in progress_log)


# ---------------------------------------------------------------------------
# Retry behaviour
# ---------------------------------------------------------------------------


class TestRetryRecovers:
    async def test_recovers_after_two_transient_failures(self, tmp_path):
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        success_chunks = [b"final ", b"content"]
        calls = _wire_session(dl, 2, success_chunks)

        with patch(
            "tidal.downloader.asyncio.sleep", new_callable=AsyncMock
        ) as sleep_mock:
            await dl._download_file(_manifest(), target, track_num=1)

        assert calls["n"] == 3  # 2 fails + 1 success
        assert open(target, "rb").read() == b"final content"
        assert sleep_mock.call_count == 2
        assert sleep_mock.call_args_list[0].args == (2,)
        assert sleep_mock.call_args_list[1].args == (4,)

    async def test_partial_file_cleaned_up_between_retries(self, tmp_path):
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        success_chunks = [b"good"]
        _wire_session(dl, 1, success_chunks)

        with patch("tidal.downloader.asyncio.sleep", new_callable=AsyncMock):
            await dl._download_file(_manifest(), target, track_num=1)

        # Final file is the success chunks only — no leftover bytes
        assert open(target, "rb").read() == b"good"
        assert b"partial" not in open(target, "rb").read()


class TestRetryExhausted:
    async def test_raises_after_all_retries_exhausted(self, tmp_path):
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        calls = _wire_session(dl, 99, [b"unused"])

        with patch(
            "tidal.downloader.asyncio.sleep", new_callable=AsyncMock
        ) as sleep_mock:
            with pytest.raises(aiohttp.ClientPayloadError):
                await dl._download_file(_manifest(), target, track_num=1, retries=5)

        assert calls["n"] == 5
        assert sleep_mock.call_count == 4
        assert [c.args[0] for c in sleep_mock.call_args_list] == [2, 4, 8, 16]

    async def test_exponential_backoff_doubles_each_attempt(self, tmp_path):
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        _wire_session(dl, 99, [])

        with patch(
            "tidal.downloader.asyncio.sleep", new_callable=AsyncMock
        ) as sleep_mock:
            with pytest.raises(aiohttp.ClientPayloadError):
                await dl._download_file(_manifest(), target, track_num=1, retries=5)

        sleeps = [c.args[0] for c in sleep_mock.call_args_list]
        assert sleeps == [2, 4, 8, 16]
        for i in range(1, len(sleeps)):
            assert sleeps[i] == sleeps[i - 1] * 2

    async def test_retries_default_is_five(self, tmp_path):
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        calls = _wire_session(dl, 99, [])

        with patch("tidal.downloader.asyncio.sleep", new_callable=AsyncMock):
            with pytest.raises(aiohttp.ClientPayloadError):
                # No retries kwarg — should use the default
                await dl._download_file(_manifest(), target, track_num=1)

        assert calls["n"] == 5


# ---------------------------------------------------------------------------
# Encrypted .enc temp-file interaction with retry
# ---------------------------------------------------------------------------


class TestEncryptedRetry:
    async def test_enc_temp_cleaned_between_retries(self, tmp_path):
        """Encrypted downloads stream to ``<target>.enc`` first.  Each failed
        retry must drop the temp file so the next attempt starts fresh."""
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        temp = target + ".enc"
        success_chunks = [b"encrypted_payload"]
        _wire_session(dl, 2, success_chunks)

        # Decrypt would normally consume the .enc file; stub it out so we
        # only exercise the download/retry path.
        with patch(
            "tidal.downloader._decrypt_mqa"
        ) as decrypt_mock, patch(
            "tidal.downloader.asyncio.sleep", new_callable=AsyncMock
        ):
            await dl._download_file(_manifest(encrypted=True), target, track_num=1)

        # Decrypt was called once with the .enc temp + final target
        decrypt_mock.assert_called_once()
        # Outer try/finally must remove the .enc temp on the success path too
        assert not os.path.exists(temp), "encrypted temp leaked after success"

    async def test_enc_temp_cleaned_when_all_retries_fail(self, tmp_path):
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        temp = target + ".enc"
        _wire_session(dl, 99, [])

        with patch(
            "tidal.downloader.asyncio.sleep", new_callable=AsyncMock
        ):
            with pytest.raises(aiohttp.ClientPayloadError):
                await dl._download_file(
                    _manifest(encrypted=True), target, track_num=1, retries=3
                )

        # The outer try/finally cleans up even on the final raise
        assert not os.path.exists(temp), "encrypted temp leaked after failed retries"

    async def test_decrypt_failure_is_not_retried(self, tmp_path):
        """Decrypt errors indicate a bad key, not a network blip — don't retry."""
        dl = _make_downloader(tmp_path)
        target = str(tmp_path / "track.flac")
        success_chunks = [b"encrypted_payload"]
        calls = _wire_session(dl, 0, success_chunks)

        with patch(
            "tidal.downloader._decrypt_mqa", side_effect=ValueError("bad key")
        ), patch("tidal.downloader.asyncio.sleep", new_callable=AsyncMock):
            with pytest.raises(ValueError, match="bad key"):
                await dl._download_file(_manifest(encrypted=True), target, track_num=1)

        # Network call was made exactly once — decrypt failure didn't trigger
        # the download retry loop
        assert calls["n"] == 1
        # Temp file is still cleaned up by the outer try/finally
        assert not os.path.exists(target + ".enc")
