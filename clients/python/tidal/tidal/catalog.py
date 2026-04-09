"""Catalog API — albums, artists, tracks, search."""

from __future__ import annotations

from typing import Any

from ._http import HttpTransport, LISTEN_URL
from .errors import NonStreamableError
from .types import Album, PaginatedResult, Track


class CatalogAPI:
    """Wraps album/*, artist/*, track/*, and search/* Tidal v1 endpoints."""

    def __init__(self, transport: HttpTransport) -> None:
        self._t = transport

    # -- Albums ---------------------------------------------------------------

    async def get_album(self, album_id: int | str) -> Album:
        """GET albums/{id} — fetch a single album by ID."""
        _, body = await self._t.get(f"albums/{album_id}")
        return Album.from_dict(body)

    async def get_album_with_tracks(
        self,
        album_id: int | str,
    ) -> tuple[Album, list[Track]]:
        """Fetch album metadata and its full track list.

        Tidal returns the first page of tracks via ``albums/{id}/items``;
        if the album has more than 100 tracks, additional pages are fetched
        and concatenated.
        """
        album = await self.get_album(album_id)
        tracks = await self._fetch_album_items(album_id, total=album.number_of_tracks)
        return album, tracks

    async def _fetch_album_items(
        self, album_id: int | str, *, total: int = 0, page_size: int = 100
    ) -> list[Track]:
        items: list[Track] = []
        offset = 0
        while True:
            _, body = await self._t.get(
                f"albums/{album_id}/items",
                {"limit": page_size, "offset": offset},
            )
            page_items = body.get("items", [])
            for entry in page_items:
                # Tidal wraps each entry as {item: {...}, type: "track"}
                inner = entry.get("item", entry)
                items.append(Track.from_dict(inner))
            if not page_items or (total and len(items) >= total):
                break
            if len(page_items) < page_size:
                break
            offset += page_size
        return items

    # -- Tracks ---------------------------------------------------------------

    async def get_track(self, track_id: int | str) -> Track:
        """GET tracks/{id} — fetch a single track by ID."""
        _, body = await self._t.get(f"tracks/{track_id}")
        return Track.from_dict(body)

    async def get_track_lyrics(self, track_id: int | str) -> dict:
        """GET tracks/{id}/lyrics — fetch lyrics from listen.tidal.com.

        Returns the raw response dict (typically with ``lyrics`` and
        ``subtitles`` keys). Returns an empty dict on 404 / 401.
        """
        try:
            _, body = await self._t.get(
                f"tracks/{track_id}/lyrics", base=LISTEN_URL
            )
            return body
        except NonStreamableError:
            return {}
        except Exception:
            return {}

    # -- Artists --------------------------------------------------------------

    async def get_artist(self, artist_id: int | str) -> dict:
        """GET artists/{id} — raw dict (Tidal artist schema is light)."""
        _, body = await self._t.get(f"artists/{artist_id}")
        return body

    async def get_artist_albums(
        self,
        artist_id: int | str,
        *,
        limit: int = 100,
        offset: int = 0,
        eps_and_singles: bool = False,
    ) -> PaginatedResult:
        """GET artists/{id}/albums — paginated album list."""
        params: dict[str, Any] = {"limit": limit, "offset": offset}
        if eps_and_singles:
            params["filter"] = "EPSANDSINGLES"
        _, body = await self._t.get(f"artists/{artist_id}/albums", params)
        return PaginatedResult.from_dict(body)

    # -- Search ---------------------------------------------------------------

    async def search_albums(
        self,
        query: str,
        *,
        limit: int = 50,
    ) -> PaginatedResult:
        """GET search/albums — search albums by query string."""
        _, body = await self._t.get(
            "search/albums", {"query": query, "limit": limit}
        )
        return PaginatedResult.from_dict(body)

    async def search_tracks(
        self,
        query: str,
        *,
        limit: int = 50,
    ) -> PaginatedResult:
        """GET search/tracks — search tracks by query string."""
        _, body = await self._t.get(
            "search/tracks", {"query": query, "limit": limit}
        )
        return PaginatedResult.from_dict(body)

    async def search_artists(
        self,
        query: str,
        *,
        limit: int = 50,
    ) -> PaginatedResult:
        """GET search/artists — search artists by query string."""
        _, body = await self._t.get(
            "search/artists", {"query": query, "limit": limit}
        )
        return PaginatedResult.from_dict(body)
