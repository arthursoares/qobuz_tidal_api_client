"""Favorites API — user library reads (albums, tracks, artists)."""

from __future__ import annotations

from typing import Any

from ._http import HttpTransport
from .types import PaginatedResult


class FavoritesAPI:
    """Wraps users/{user_id}/favorites/* Tidal v1 endpoints."""

    def __init__(self, transport: HttpTransport, user_id: int | str) -> None:
        self._t = transport
        self._user_id = str(user_id)

    @property
    def user_id(self) -> str:
        return self._user_id

    def set_user_id(self, user_id: int | str) -> None:
        """Update the cached user ID after auth changes."""
        self._user_id = str(user_id)

    async def get_albums(
        self,
        *,
        limit: int = 100,
        offset: int = 0,
        order: str = "DATE",
        order_direction: str = "DESC",
    ) -> PaginatedResult:
        """GET users/{id}/favorites/albums — one page of favorite albums.

        For full pagination across the whole library, see :meth:`all_albums`.
        """
        params: dict[str, Any] = {
            "limit": limit,
            "offset": offset,
            "order": order,
            "orderDirection": order_direction,
        }
        _, body = await self._t.get(
            f"users/{self._user_id}/favorites/albums", params
        )
        return PaginatedResult.from_dict(body)

    async def all_albums(self, *, page_size: int = 500) -> list[dict]:
        """Fetch every favorite album by walking through pagination.

        Each item is the raw Tidal entry, which usually has the shape
        ``{created: "...", item: {<album fields>}}``. Callers can pass each
        ``item`` field through :meth:`Album.from_dict` if they want typed
        models.
        """
        all_items: list[dict] = []
        offset = 0
        while True:
            page = await self.get_albums(limit=page_size, offset=offset)
            if not page.items:
                break
            all_items.extend(page.items)
            if not page.has_more:
                break
            offset += len(page.items)
        return all_items

    async def get_tracks(
        self,
        *,
        limit: int = 100,
        offset: int = 0,
    ) -> PaginatedResult:
        """GET users/{id}/favorites/tracks — one page of favorite tracks."""
        _, body = await self._t.get(
            f"users/{self._user_id}/favorites/tracks",
            {"limit": limit, "offset": offset},
        )
        return PaginatedResult.from_dict(body)

    async def get_artists(
        self,
        *,
        limit: int = 100,
        offset: int = 0,
    ) -> PaginatedResult:
        """GET users/{id}/favorites/artists — one page of favorite artists."""
        _, body = await self._t.get(
            f"users/{self._user_id}/favorites/artists",
            {"limit": limit, "offset": offset},
        )
        return PaginatedResult.from_dict(body)
