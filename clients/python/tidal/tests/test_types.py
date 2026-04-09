"""Tests for the Tidal type dataclasses."""

from conftest import (
    SAMPLE_ALBUM,
    SAMPLE_PLAYLIST,
    SAMPLE_TRACK,
)

from tidal.types import (
    QUALITY_MAP,
    Album,
    AlbumSummary,
    ArtistSummary,
    PaginatedResult,
    Playlist,
    StreamManifest,
    Track,
)


class TestArtistSummary:
    def test_from_dict_parses_main_artist(self):
        a = ArtistSummary.from_dict({"id": 100, "name": "Radiohead", "type": "MAIN"})
        assert a.id == 100
        assert a.name == "Radiohead"
        assert a.type == "MAIN"

    def test_from_dict_handles_none(self):
        a = ArtistSummary.from_dict(None)
        assert a.id == 0
        assert a.name == "Unknown"

    def test_from_dict_handles_empty(self):
        # Empty dict is falsy, so we treat it the same as None.
        a = ArtistSummary.from_dict({})
        assert a.id == 0
        assert a.name == "Unknown"


class TestAlbumSummary:
    def test_from_dict(self):
        a = AlbumSummary.from_dict({"id": 1, "title": "X", "cover": "c", "releaseDate": "2024-01-01"})
        assert a.id == 1
        assert a.title == "X"
        assert a.cover == "c"
        assert a.release_date == "2024-01-01"

    def test_from_dict_handles_none(self):
        a = AlbumSummary.from_dict(None)
        assert a.id == 0
        assert a.title == ""


class TestAlbum:
    def test_from_dict_parses_sample(self):
        album = Album.from_dict(SAMPLE_ALBUM)
        assert album.id == 12345
        assert album.title == "Test Album"
        assert album.artist.name == "Test Artist"
        assert len(album.artists) == 2
        assert album.artists[1].name == "Featured Artist"
        assert album.duration == 3600
        assert album.number_of_tracks == 12
        assert album.audio_quality == "HI_RES"
        assert album.upc == "0123456789012"
        assert album.copyright == "(C) 2024 Test Label"

    def test_tracks_field_defaults_to_empty(self):
        album = Album.from_dict(SAMPLE_ALBUM)
        assert album.tracks == []

    def test_tracks_field_accepts_list(self):
        payload = {**SAMPLE_ALBUM, "tracks": [SAMPLE_TRACK, SAMPLE_TRACK]}
        album = Album.from_dict(payload)
        assert len(album.tracks) == 2

    def test_tracks_field_ignores_non_list(self):
        payload = {**SAMPLE_ALBUM, "tracks": {"items": [SAMPLE_TRACK]}}
        album = Album.from_dict(payload)
        # Tidal v1 album responses don't nest tracks under .items, so the
        # parser only treats top-level lists as tracks. Dict should be ignored.
        assert album.tracks == []

    def test_handles_missing_artists_array(self):
        payload = {**SAMPLE_ALBUM}
        del payload["artists"]
        album = Album.from_dict(payload)
        assert album.artists == []


class TestTrack:
    def test_from_dict_parses_sample(self):
        track = Track.from_dict(SAMPLE_TRACK)
        assert track.id == 67890
        assert track.title == "Test Track"
        assert track.track_number == 3
        assert track.volume_number == 1
        assert track.explicit is True
        assert track.isrc == "USABC2412345"
        assert track.artist.name == "Test Artist"
        assert track.album is not None
        assert track.album.id == 12345
        assert track.copyright == "(C) 2024 Test Label"

    def test_album_can_be_none(self):
        payload = {**SAMPLE_TRACK}
        del payload["album"]
        track = Track.from_dict(payload)
        assert track.album is None


class TestPlaylist:
    def test_from_dict_parses_sample(self):
        pl = Playlist.from_dict(SAMPLE_PLAYLIST)
        assert pl.uuid == "1234abcd-5678-efgh-ijkl-mnopqrstuvwx"
        assert pl.title == "Test Playlist"
        assert pl.number_of_tracks == 25
        assert pl.public is True
        assert pl.creator["name"] == "test-user"


class TestPaginatedResult:
    def test_from_dict(self):
        page = PaginatedResult.from_dict({
            "items": [{"id": 1}, {"id": 2}],
            "totalNumberOfItems": 10,
            "limit": 100,
            "offset": 0,
        })
        assert len(page.items) == 2
        assert page.total == 10
        assert page.limit == 100
        assert page.offset == 0

    def test_has_more_when_more_pages_remain(self):
        page = PaginatedResult.from_dict({
            "items": [{"id": 1}],
            "totalNumberOfItems": 10,
            "limit": 1,
            "offset": 0,
        })
        assert page.has_more is True

    def test_has_more_false_at_end(self):
        page = PaginatedResult.from_dict({
            "items": [{"id": 10}],
            "totalNumberOfItems": 10,
            "limit": 1,
            "offset": 9,
        })
        assert page.has_more is False

    def test_handles_missing_fields(self):
        page = PaginatedResult.from_dict({})
        assert page.items == []
        assert page.total == 0
        assert page.has_more is False


class TestStreamManifest:
    def test_flac_extension(self):
        m = StreamManifest(
            track_id=1, audio_quality="LOSSLESS", codec="flac", url="http://x",
        )
        assert m.file_extension == "flac"
        assert m.is_encrypted is False

    def test_mqa_extension(self):
        m = StreamManifest(
            track_id=1, audio_quality="HI_RES", codec="MQA", url="http://x",
        )
        assert m.file_extension == "flac"

    def test_aac_extension(self):
        m = StreamManifest(
            track_id=1, audio_quality="HIGH", codec="aac", url="http://x",
        )
        assert m.file_extension == "m4a"

    def test_is_encrypted_when_key_present(self):
        m = StreamManifest(
            track_id=1,
            audio_quality="HI_RES",
            codec="mqa",
            url="http://x",
            encryption_type="OLD_AES",
            encryption_key="abc==",
        )
        assert m.is_encrypted is True

    def test_not_encrypted_when_type_none(self):
        m = StreamManifest(
            track_id=1,
            audio_quality="LOSSLESS",
            codec="flac",
            url="http://x",
            encryption_type="NONE",
            encryption_key="abc==",  # ignored
        )
        assert m.is_encrypted is False


class TestQualityMap:
    def test_all_quality_tiers_mapped(self):
        assert QUALITY_MAP[0] == "LOW"
        assert QUALITY_MAP[1] == "HIGH"
        assert QUALITY_MAP[2] == "LOSSLESS"
        assert QUALITY_MAP[3] == "HI_RES"
