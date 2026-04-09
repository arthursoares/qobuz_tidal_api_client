"""Tests for the Tidal catalog API."""

import pytest
from unittest.mock import AsyncMock

from conftest import (
    SAMPLE_ALBUM,
    SAMPLE_ALBUM_ITEMS_PAGE,
    SAMPLE_SEARCH_RESPONSE,
    SAMPLE_TRACK,
)

from tidal.catalog import CatalogAPI
from tidal.errors import NonStreamableError
from tidal.types import Album, PaginatedResult, Track


@pytest.fixture
def catalog(mock_transport: AsyncMock) -> CatalogAPI:
    return CatalogAPI(mock_transport)


# ---------------------------------------------------------------------------
# Albums
# ---------------------------------------------------------------------------


async def test_get_album_returns_typed_album(
    catalog: CatalogAPI, mock_transport: AsyncMock
):
    mock_transport.get.return_value = (200, SAMPLE_ALBUM)

    album = await catalog.get_album(12345)

    assert isinstance(album, Album)
    assert album.id == 12345
    assert album.title == "Test Album"
    mock_transport.get.assert_awaited_once_with("albums/12345")


async def test_get_album_with_tracks_combines_album_and_items(
    catalog: CatalogAPI, mock_transport: AsyncMock
):
    # First call: album/get. Second call: album items page.
    mock_transport.get.side_effect = [
        (200, {**SAMPLE_ALBUM, "numberOfTracks": 2}),
        (200, SAMPLE_ALBUM_ITEMS_PAGE),
    ]

    album, tracks = await catalog.get_album_with_tracks(12345)

    assert isinstance(album, Album)
    assert album.id == 12345
    assert len(tracks) == 2
    assert all(isinstance(t, Track) for t in tracks)
    assert tracks[0].id == 67890
    assert tracks[1].title == "Track Two"


async def test_get_album_with_tracks_paginates_when_needed(
    catalog: CatalogAPI, mock_transport: AsyncMock
):
    """If the album has more than page_size tracks, fetch additional pages."""
    big_album = {**SAMPLE_ALBUM, "numberOfTracks": 150}
    full_page = {
        "limit": 100,
        "offset": 0,
        "totalNumberOfItems": 150,
        "items": [{"item": SAMPLE_TRACK, "type": "track"}] * 100,
    }
    second_page = {
        "limit": 100,
        "offset": 100,
        "totalNumberOfItems": 150,
        "items": [{"item": SAMPLE_TRACK, "type": "track"}] * 50,
    }
    mock_transport.get.side_effect = [
        (200, big_album),
        (200, full_page),
        (200, second_page),
    ]

    _, tracks = await catalog.get_album_with_tracks(12345)
    assert len(tracks) == 150


# ---------------------------------------------------------------------------
# Tracks
# ---------------------------------------------------------------------------


async def test_get_track_returns_typed_track(
    catalog: CatalogAPI, mock_transport: AsyncMock
):
    mock_transport.get.return_value = (200, SAMPLE_TRACK)

    track = await catalog.get_track(67890)

    assert isinstance(track, Track)
    assert track.id == 67890
    mock_transport.get.assert_awaited_once_with("tracks/67890")


async def test_get_track_lyrics_returns_dict(
    catalog: CatalogAPI, mock_transport: AsyncMock
):
    mock_transport.get.return_value = (200, {"lyrics": "test", "subtitles": "synced"})

    lyrics = await catalog.get_track_lyrics(67890)
    assert lyrics == {"lyrics": "test", "subtitles": "synced"}


async def test_get_track_lyrics_returns_empty_on_failure(
    catalog: CatalogAPI, mock_transport: AsyncMock
):
    mock_transport.get.side_effect = NonStreamableError("not found", status=404)

    lyrics = await catalog.get_track_lyrics(67890)
    assert lyrics == {}


async def test_get_track_lyrics_swallows_unknown_errors(
    catalog: CatalogAPI, mock_transport: AsyncMock
):
    mock_transport.get.side_effect = RuntimeError("network blew up")

    lyrics = await catalog.get_track_lyrics(67890)
    assert lyrics == {}


# ---------------------------------------------------------------------------
# Search
# ---------------------------------------------------------------------------


async def test_search_albums_returns_paginated(
    catalog: CatalogAPI, mock_transport: AsyncMock
):
    mock_transport.get.return_value = (200, SAMPLE_SEARCH_RESPONSE)

    result = await catalog.search_albums("radiohead", limit=20)

    assert isinstance(result, PaginatedResult)
    assert result.total == 1
    mock_transport.get.assert_awaited_once_with(
        "search/albums", {"query": "radiohead", "limit": 20}
    )


async def test_search_tracks_uses_track_endpoint(
    catalog: CatalogAPI, mock_transport: AsyncMock
):
    mock_transport.get.return_value = (200, SAMPLE_SEARCH_RESPONSE)

    await catalog.search_tracks("test", limit=10)

    mock_transport.get.assert_awaited_once_with(
        "search/tracks", {"query": "test", "limit": 10}
    )


async def test_search_artists_uses_artist_endpoint(
    catalog: CatalogAPI, mock_transport: AsyncMock
):
    mock_transport.get.return_value = (200, SAMPLE_SEARCH_RESPONSE)

    await catalog.search_artists("test", limit=10)

    mock_transport.get.assert_awaited_once_with(
        "search/artists", {"query": "test", "limit": 10}
    )


# ---------------------------------------------------------------------------
# Artists
# ---------------------------------------------------------------------------


async def test_get_artist_returns_raw_dict(
    catalog: CatalogAPI, mock_transport: AsyncMock
):
    mock_transport.get.return_value = (200, {"id": 100, "name": "Test"})

    artist = await catalog.get_artist(100)
    assert artist == {"id": 100, "name": "Test"}


async def test_get_artist_albums_passes_filter(
    catalog: CatalogAPI, mock_transport: AsyncMock
):
    mock_transport.get.return_value = (200, SAMPLE_SEARCH_RESPONSE)

    await catalog.get_artist_albums(100, eps_and_singles=True)

    mock_transport.get.assert_awaited_once_with(
        "artists/100/albums",
        {"limit": 100, "offset": 0, "filter": "EPSANDSINGLES"},
    )
