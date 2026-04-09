"""TidalClient — main entry point tying all API namespaces together."""

from __future__ import annotations

import time
from pathlib import Path
from typing import Any

from . import auth as auth_mod
from ._http import HttpTransport
from .catalog import CatalogAPI
from .favorites import FavoritesAPI
from .streaming import StreamingAPI


class TidalClient:
    """Async Tidal API client.

    Usage::

        async with TidalClient(
            access_token="...",
            refresh_token="...",
            user_id="123",
            country_code="US",
        ) as client:
            page = await client.favorites.get_albums()
            album = await client.catalog.get_album(123456)
            manifest = await client.streaming.get_manifest(789, quality=3)

    Or from saved credentials::

        async with TidalClient.from_credentials() as client:
            ...
    """

    def __init__(
        self,
        access_token: str,
        refresh_token: str | None = None,
        user_id: int | str = 0,
        country_code: str = "US",
        token_expiry: float = 0.0,
        requests_per_minute: int = 240,
    ) -> None:
        self._transport = HttpTransport(
            access_token=access_token,
            country_code=country_code,
            requests_per_minute=requests_per_minute,
        )
        self._refresh_token = refresh_token
        self._token_expiry = token_expiry

        self.catalog = CatalogAPI(self._transport)
        self.favorites = FavoritesAPI(self._transport, user_id=user_id)
        self.streaming = StreamingAPI(self._transport)

    @classmethod
    def from_credentials(
        cls,
        credentials_path: str | Path | None = None,
        **kwargs: Any,
    ) -> TidalClient:
        """Create a client from a saved credentials file.

        Defaults to ``~/.config/tidal/credentials.json``. Raises
        ``FileNotFoundError`` if no credentials are present.
        """
        path = Path(credentials_path) if credentials_path else None
        creds = auth_mod.load_credentials(path)
        if not creds:
            target = path or auth_mod.CREDENTIALS_FILE
            raise FileNotFoundError(
                f"No Tidal credentials found at {target}. "
                "Run the device-code OAuth flow first (see auth.request_device_code)."
            )
        return cls(
            access_token=creds["access_token"],
            refresh_token=creds.get("refresh_token"),
            user_id=creds.get("user_id", 0),
            country_code=creds.get("country_code", "US"),
            token_expiry=float(creds.get("token_expiry", 0)),
            **kwargs,
        )

    @property
    def access_token(self) -> str:
        return self._transport.access_token

    @property
    def refresh_token(self) -> str | None:
        return self._refresh_token

    @property
    def token_expiry(self) -> float:
        return self._token_expiry

    async def ensure_token(self, *, refresh_window_seconds: int = 86400) -> bool:
        """Refresh the access token if it expires within ``refresh_window_seconds``.

        Returns True if a refresh was performed, False otherwise. Raises
        ``RuntimeError`` if no refresh token is available and the access
        token is expired.
        """
        if self._token_expiry == 0:
            return False
        if self._token_expiry - time.time() > refresh_window_seconds:
            return False
        if not self._refresh_token:
            raise RuntimeError(
                "Tidal access token is expired and no refresh token is available"
            )
        new = await auth_mod.refresh_access_token(self._refresh_token)
        self._transport.set_access_token(new["access_token"])
        self._token_expiry = new["token_expiry"]
        if new.get("refresh_token"):
            self._refresh_token = new["refresh_token"]
        return True

    async def __aenter__(self) -> TidalClient:
        await self._transport.__aenter__()
        return self

    async def __aexit__(self, *args: object) -> None:
        await self._transport.__aexit__(*args)
