"""Tests for the Tidal favorites API."""

import pytest
from unittest.mock import AsyncMock

from conftest import SAMPLE_FAVORITES_PAGE

from tidal.favorites import FavoritesAPI
from tidal.types import PaginatedResult


@pytest.fixture
def favorites(mock_transport: AsyncMock) -> FavoritesAPI:
    return FavoritesAPI(mock_transport, user_id=999)


async def test_get_albums_default_params(
    favorites: FavoritesAPI, mock_transport: AsyncMock
):
    mock_transport.get.return_value = (200, SAMPLE_FAVORITES_PAGE)

    page = await favorites.get_albums()

    assert isinstance(page, PaginatedResult)
    assert page.total == 2
    assert len(page.items) == 2
    mock_transport.get.assert_awaited_once_with(
        "users/999/favorites/albums",
        {
            "limit": 100,
            "offset": 0,
            "order": "DATE",
            "orderDirection": "DESC",
        },
    )


async def test_get_albums_custom_params(
    favorites: FavoritesAPI, mock_transport: AsyncMock
):
    mock_transport.get.return_value = (200, SAMPLE_FAVORITES_PAGE)

    await favorites.get_albums(limit=50, offset=100, order="NAME", order_direction="ASC")

    mock_transport.get.assert_awaited_once_with(
        "users/999/favorites/albums",
        {"limit": 50, "offset": 100, "order": "NAME", "orderDirection": "ASC"},
    )


async def test_get_tracks_uses_tracks_endpoint(
    favorites: FavoritesAPI, mock_transport: AsyncMock
):
    mock_transport.get.return_value = (200, SAMPLE_FAVORITES_PAGE)

    await favorites.get_tracks(limit=25)

    mock_transport.get.assert_awaited_once_with(
        "users/999/favorites/tracks", {"limit": 25, "offset": 0}
    )


async def test_get_artists_uses_artists_endpoint(
    favorites: FavoritesAPI, mock_transport: AsyncMock
):
    mock_transport.get.return_value = (200, SAMPLE_FAVORITES_PAGE)

    await favorites.get_artists()

    mock_transport.get.assert_awaited_once_with(
        "users/999/favorites/artists", {"limit": 100, "offset": 0}
    )


async def test_all_albums_concatenates_pages(
    favorites: FavoritesAPI, mock_transport: AsyncMock
):
    # First page returns 2 items but says 4 total → has_more is True.
    page1 = {**SAMPLE_FAVORITES_PAGE, "totalNumberOfItems": 4}
    page2 = {
        "limit": 500,
        "offset": 2,
        "totalNumberOfItems": 4,
        "items": [
            {"item": {"id": 99}},
            {"item": {"id": 100}},
        ],
    }
    mock_transport.get.side_effect = [(200, page1), (200, page2)]

    items = await favorites.all_albums()
    assert len(items) == 4


async def test_all_albums_stops_on_empty_page(
    favorites: FavoritesAPI, mock_transport: AsyncMock
):
    empty = {"limit": 500, "offset": 0, "totalNumberOfItems": 0, "items": []}
    mock_transport.get.return_value = (200, empty)

    items = await favorites.all_albums()
    assert items == []


def test_set_user_id_updates_url(mock_transport: AsyncMock):
    favs = FavoritesAPI(mock_transport, user_id=1)
    assert favs.user_id == "1"
    favs.set_user_id(42)
    assert favs.user_id == "42"
