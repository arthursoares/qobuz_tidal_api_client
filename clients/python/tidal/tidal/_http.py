"""Async HTTP transport for the Tidal v1 API.

The v1 API is the legacy/internal Tidal endpoint set (api.tidalhifi.com/v1
and listen.tidal.com/v1). It's the only one that exposes
``playbackinfopostpaywall`` for downloads, so this SDK targets it instead
of the public openapi.tidal.com/v2 API used by the Go Tidal client.
"""

from __future__ import annotations

from typing import Any

import aiohttp
from aiolimiter import AsyncLimiter

from .errors import raise_for_status

BASE_URL = "https://api.tidalhifi.com/v1"
LISTEN_URL = "https://listen.tidal.com/v1"

_USER_AGENT = "tidal-python-sdk/0.1.0"


class HttpTransport:
    """Low-level async HTTP client for the Tidal v1 REST API.

    Manages session lifecycle, auth headers, the ``countryCode`` query
    param required by every endpoint, rate limiting, and error mapping.
    Must be used as an async context manager.
    """

    def __init__(
        self,
        access_token: str,
        country_code: str = "US",
        requests_per_minute: int = 240,
    ) -> None:
        self._access_token = access_token
        self._country_code = country_code
        self._limiter = AsyncLimiter(requests_per_minute, 60)
        self._session: aiohttp.ClientSession | None = None

    # -- Token management ----------------------------------------------------

    @property
    def access_token(self) -> str:
        return self._access_token

    def set_access_token(self, token: str) -> None:
        """Update the bearer token after a refresh."""
        self._access_token = token

    # -- Header helpers ------------------------------------------------------

    def _headers(self) -> dict[str, str]:
        return {
            "User-Agent": _USER_AGENT,
            "Authorization": f"Bearer {self._access_token}",
        }

    # -- Context manager -----------------------------------------------------

    async def __aenter__(self) -> HttpTransport:
        self._session = aiohttp.ClientSession(headers=self._headers())
        return self

    async def __aexit__(self, *args: object) -> None:
        if self._session is not None:
            await self._session.close()
            self._session = None

    # -- Internal request impl -----------------------------------------------

    def _build_params(self, params: dict[str, Any] | None) -> dict[str, Any]:
        merged: dict[str, Any] = {"countryCode": self._country_code}
        if params:
            merged.update(params)
        return merged

    async def _do(
        self,
        method: str,
        url: str,
        *,
        params: dict[str, Any] | None = None,
        data: dict[str, Any] | None = None,
        json_body: dict[str, Any] | None = None,
        raise_errors: bool = True,
    ) -> tuple[int, dict]:
        if self._session is None:
            raise RuntimeError("HttpTransport must be used as an async context manager")

        # Refresh auth header in case the token was rotated.
        headers = {"Authorization": f"Bearer {self._access_token}"}

        async with self._limiter:
            async with self._session.request(
                method,
                url,
                params=self._build_params(params),
                data=data,
                json=json_body,
                headers=headers,
            ) as resp:
                status = resp.status
                try:
                    body = await resp.json(content_type=None)
                except (aiohttp.ContentTypeError, ValueError):
                    body = {}

        if raise_errors:
            raise_for_status(status, body if isinstance(body, dict) else None)
        return status, body if isinstance(body, dict) else {}

    # -- Public request methods ---------------------------------------------

    async def get(
        self,
        endpoint: str,
        params: dict[str, Any] | None = None,
        *,
        base: str = BASE_URL,
        raise_errors: bool = True,
    ) -> tuple[int, dict]:
        url = f"{base}/{endpoint.lstrip('/')}"
        return await self._do("GET", url, params=params, raise_errors=raise_errors)

    async def post_form(
        self,
        endpoint: str,
        data: dict[str, Any],
        *,
        base: str = BASE_URL,
        raise_errors: bool = True,
    ) -> tuple[int, dict]:
        url = f"{base}/{endpoint.lstrip('/')}"
        return await self._do("POST", url, data=data, raise_errors=raise_errors)

    async def session(self) -> aiohttp.ClientSession:
        """Return the underlying aiohttp session.

        Used by the downloader for direct file downloads (which don't go
        through the Tidal API request flow).
        """
        if self._session is None:
            raise RuntimeError("HttpTransport must be used as an async context manager")
        return self._session
