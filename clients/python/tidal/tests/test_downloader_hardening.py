"""Regression tests for the fixes from the Codex review of PR #2.

Covers the risky paths that were previously unguarded:
- folder quality metadata derived from Tidal's audioQuality enum
- disc subdirectory naming (``Disc N`` for streamrip layout compatibility)
- temp-file cleanup for encrypted downloads on every failure path
- ``asyncio.gather`` resilience to uncaught track exceptions
- client auto-refresh on ``__aenter__`` and 401 retry hook wiring
"""

import asyncio
import os

import pytest
from unittest.mock import AsyncMock, MagicMock, patch

from tidal.downloader import (
    AlbumDownloader,
    DownloadConfig,
    TrackResult,
    _tidal_quality_fields,
)
from tidal.types import Album, ArtistSummary, StreamManifest, Track


# ---------------------------------------------------------------------------
# Folder quality metadata
# ---------------------------------------------------------------------------


class TestTidalQualityFields:
    def test_lossless_is_cd_quality_flac(self):
        assert _tidal_quality_fields("LOSSLESS", 3) == ("FLAC", 16, "44.1")

    def test_hi_res_is_24_44_1_flac(self):
        # Tidal MQA / HiRes lives in a 44.1 kHz container, not 96 kHz.
        assert _tidal_quality_fields("HI_RES", 3) == ("FLAC", 24, "44.1")

    def test_high_is_aac(self):
        assert _tidal_quality_fields("HIGH", 1) == ("AAC", 16, "44.1")

    def test_low_is_aac(self):
        assert _tidal_quality_fields("LOW", 0) == ("AAC", 16, "44.1")

    def test_missing_album_quality_falls_back_to_config(self):
        # If Tidal didn't report audioQuality, use the configured tier so
        # the folder still matches what was actually downloaded.
        assert _tidal_quality_fields(None, 3) == ("FLAC", 24, "44.1")
        assert _tidal_quality_fields("", 1) == ("AAC", 16, "44.1")

    def test_unknown_quality_defaults_to_cd_quality(self):
        # Don't lie about HiRes — fall back to CD quality FLAC on unknown.
        assert _tidal_quality_fields("WEIRD", 3) == ("FLAC", 16, "44.1")

    def test_case_insensitive(self):
        assert _tidal_quality_fields("hi_res", 3) == ("FLAC", 24, "44.1")


# ---------------------------------------------------------------------------
# Disc naming
# ---------------------------------------------------------------------------


def _make_album(number_of_volumes: int = 1) -> Album:
    return Album(
        id=1,
        title="Test Album",
        artist=ArtistSummary(id=1, name="Test Artist"),
        release_date="2024-01-01",
        audio_quality="LOSSLESS",
        number_of_tracks=4,
        number_of_volumes=number_of_volumes,
    )


def _make_track(volume_number: int = 1, track_number: int = 1) -> Track:
    return Track(
        id=1000 + track_number,
        title=f"Track {track_number}",
        artist=ArtistSummary(id=1, name="Test Artist"),
        track_number=track_number,
        volume_number=volume_number,
    )


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


class TestDiscSubdirectories:
    def test_single_disc_has_no_subdir(self, tmp_path):
        dl = _make_downloader(tmp_path)
        album = _make_album(number_of_volumes=1)
        track = _make_track(volume_number=1, track_number=3)
        manifest = StreamManifest(
            track_id=1001, audio_quality="LOSSLESS", codec="flac", url="http://x"
        )
        path = dl._track_target_path(track, album, "/base", manifest)
        assert "Disc" not in path
        assert path.endswith("03 - Track 3.flac")

    def test_multi_disc_uses_streamrip_layout(self, tmp_path):
        dl = _make_downloader(tmp_path)
        album = _make_album(number_of_volumes=3)
        track = _make_track(volume_number=2, track_number=5)
        manifest = StreamManifest(
            track_id=1001, audio_quality="LOSSLESS", codec="flac", url="http://x"
        )
        path = dl._track_target_path(track, album, "/base", manifest)
        # Must match streamrip's ``Disc {N}`` folder so existing libraries
        # don't re-download.
        assert "Disc 2" in path
        assert "/base/Disc 2/" in path

    def test_disc_subdirectories_can_be_disabled(self, tmp_path):
        dl = _make_downloader(tmp_path)
        dl._config.disc_subdirectories = False
        album = _make_album(number_of_volumes=3)
        track = _make_track(volume_number=2, track_number=5)
        manifest = StreamManifest(
            track_id=1001, audio_quality="LOSSLESS", codec="flac", url="http://x"
        )
        path = dl._track_target_path(track, album, "/base", manifest)
        assert "Disc" not in path


# ---------------------------------------------------------------------------
# Temp-file cleanup
# ---------------------------------------------------------------------------


class TestTempFileCleanup:
    async def test_encrypted_temp_removed_on_get_failure(self, tmp_path):
        """An HTTP failure during GET must clean up the ``.enc`` temp file."""
        dl = _make_downloader(tmp_path)
        manifest = StreamManifest(
            track_id=1,
            audio_quality="HI_RES",
            codec="mqa",
            url="http://fake/audio",
            encryption_type="OLD_AES",
            encryption_key="fakekey==",
        )

        # Fake session that raises mid-download after creating the temp file.
        target = str(tmp_path / "track.flac")
        temp = target + ".enc"

        # Pre-create the temp file to simulate a partial write.
        open(temp, "wb").close()
        assert os.path.exists(temp)

        fake_session = MagicMock()
        # session.get(...) must be an async context manager that raises.
        fake_resp = MagicMock()
        fake_resp.__aenter__ = AsyncMock(
            side_effect=RuntimeError("connection reset mid-download")
        )
        fake_resp.__aexit__ = AsyncMock(return_value=None)
        fake_session.get = MagicMock(return_value=fake_resp)
        dl._client._transport.session = AsyncMock(return_value=fake_session)

        with pytest.raises(RuntimeError):
            await dl._download_file(manifest, target, track_num=1)

        assert not os.path.exists(temp), "encrypted temp leaked after GET failure"

    async def test_encrypted_temp_removed_on_decrypt_failure(self, tmp_path):
        """A decrypt exception must still clean up the ``.enc`` temp file."""
        dl = _make_downloader(tmp_path)
        manifest = StreamManifest(
            track_id=1,
            audio_quality="HI_RES",
            codec="mqa",
            url="http://fake/audio",
            encryption_type="OLD_AES",
            encryption_key="!!! not valid base64 !!!",
        )

        target = str(tmp_path / "track.flac")
        temp = target + ".enc"

        # Fake session that writes some bytes successfully.
        fake_resp = MagicMock()

        class _ChunkIter:
            def __aiter__(self):
                self._done = False
                return self

            async def __anext__(self):
                if self._done:
                    raise StopAsyncIteration
                self._done = True
                return b"some fake encrypted bytes"

        fake_resp_entered = MagicMock()
        fake_resp_entered.status = 200
        fake_resp_entered.headers = {"Content-Length": "25"}
        fake_resp_entered.content = MagicMock()
        fake_resp_entered.content.iter_chunked = lambda size: _ChunkIter()

        fake_resp.__aenter__ = AsyncMock(return_value=fake_resp_entered)
        fake_resp.__aexit__ = AsyncMock(return_value=None)
        fake_session = MagicMock()
        fake_session.get = MagicMock(return_value=fake_resp)
        dl._client._transport.session = AsyncMock(return_value=fake_session)

        with pytest.raises(Exception):
            await dl._download_file(manifest, target, track_num=1)

        # Temp file should be gone, target file should not exist.
        assert not os.path.exists(temp), "encrypted temp leaked after decrypt failure"


# ---------------------------------------------------------------------------
# asyncio.gather resilience
# ---------------------------------------------------------------------------


class TestGatherResilience:
    async def test_callback_exception_does_not_cancel_siblings(self, tmp_path):
        """If on_track_start raises for track A, tracks B/C must still run."""
        dl = _make_downloader(tmp_path)
        album = _make_album()
        tracks = [
            _make_track(track_number=1),
            _make_track(track_number=2),
            _make_track(track_number=3),
        ]

        call_log: list[int] = []

        def bad_on_track_start(num: int, title: str) -> None:
            call_log.append(num)
            if num == 1:
                raise RuntimeError("boom from track 1")

        dl._on_track_start = bad_on_track_start

        # Short-circuit the download pipeline so tracks "succeed" instantly.
        async def fake_one_track(
            track: Track, album: Album, album_dir: str, cover_path: str | None
        ) -> TrackResult:
            # Call the start callback the same way the real path does.
            if dl._on_track_start is not None:
                dl._on_track_start(track.track_number, track.title)
            return TrackResult(
                track_id=track.id, title=track.title, success=True
            )

        dl._download_one_track = fake_one_track  # type: ignore[assignment]
        dl._catalog.get_album_with_tracks = AsyncMock(return_value=(album, tracks))
        dl._download_cover = AsyncMock(return_value=None)  # type: ignore[assignment]

        result = await dl.download(album.id)

        # All three tracks must have been attempted.
        assert call_log == [1, 2, 3]
        # Track 1 should be failed; 2 and 3 should be OK.
        by_tn = {tracks[i].id: result.tracks[i] for i in range(3)}
        assert by_tn[tracks[0].id].success is False
        assert by_tn[tracks[1].id].success is True
        assert by_tn[tracks[2].id].success is True
        assert result.successful == 2


# ---------------------------------------------------------------------------
# Client auto-refresh
# ---------------------------------------------------------------------------


class TestClientAutoRefresh:
    async def test_aenter_refreshes_near_expired_token(self):
        """A saved credential with an expiring token should refresh on open."""
        import time as _time

        from tidal.client import TidalClient

        client = TidalClient(
            access_token="old-token",
            refresh_token="refresh-me",
            user_id=42,
            country_code="US",
            token_expiry=_time.time() + 3600,  # 1h — inside 24h window
        )

        with patch(
            "tidal.client.auth_mod.refresh_access_token",
            new_callable=AsyncMock,
        ) as mock_refresh:
            mock_refresh.return_value = {
                "access_token": "new-token",
                "token_expiry": _time.time() + 604800,
                "refresh_token": "new-refresh",
            }

            async with client as c:
                assert c.access_token == "new-token"
                assert c.refresh_token == "new-refresh"
                mock_refresh.assert_awaited_once_with("refresh-me")

    async def test_aenter_does_not_refresh_fresh_token(self):
        """A token with plenty of lifetime remaining should not be refreshed."""
        import time as _time

        from tidal.client import TidalClient

        client = TidalClient(
            access_token="fresh-token",
            refresh_token="refresh-me",
            user_id=42,
            country_code="US",
            token_expiry=_time.time() + 604800,  # 7 days remaining
        )

        with patch(
            "tidal.client.auth_mod.refresh_access_token",
            new_callable=AsyncMock,
        ) as mock_refresh:
            async with client as c:
                assert c.access_token == "fresh-token"
                mock_refresh.assert_not_awaited()

    async def test_aenter_swallows_refresh_failure(self):
        """If refresh raises (e.g. network down), fall through rather than crash."""
        import time as _time

        from tidal.client import TidalClient

        client = TidalClient(
            access_token="old-token",
            refresh_token="refresh-me",
            user_id=42,
            country_code="US",
            token_expiry=_time.time() + 3600,
        )

        with patch(
            "tidal.client.auth_mod.refresh_access_token",
            new_callable=AsyncMock,
            side_effect=RuntimeError("network down"),
        ):
            async with client as c:
                # The old token stays in place so the first API call can
                # surface the real error.
                assert c.access_token == "old-token"

    async def test_auto_refresh_disabled_skips_refresh(self):
        import time as _time

        from tidal.client import TidalClient

        client = TidalClient(
            access_token="old-token",
            refresh_token="refresh-me",
            user_id=42,
            country_code="US",
            token_expiry=_time.time() + 3600,
            auto_refresh=False,
        )

        with patch(
            "tidal.client.auth_mod.refresh_access_token",
            new_callable=AsyncMock,
        ) as mock_refresh:
            async with client as c:
                assert c.access_token == "old-token"
                mock_refresh.assert_not_awaited()

    def test_transport_has_refresh_callback_wired(self):
        """The transport should have the 401-retry hook installed."""
        from tidal.client import TidalClient

        client = TidalClient(
            access_token="t",
            refresh_token="r",
            user_id=1,
            country_code="US",
        )
        assert client._transport._refresh_callback is not None

    def test_no_refresh_token_means_no_callback(self):
        from tidal.client import TidalClient

        client = TidalClient(
            access_token="t",
            refresh_token=None,
            user_id=1,
            country_code="US",
        )
        assert client._transport._refresh_callback is None
