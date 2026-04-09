"""Tidal authentication helpers — device-code OAuth + token refresh.

The Tidal v1 mobile-app OAuth flow is what the streamrip CLI uses. The
hard-coded ``CLIENT_ID`` / ``CLIENT_SECRET`` are extracted from the iOS
app and are public knowledge in the Tidal-tooling community.
"""

from __future__ import annotations

import base64
import json
import time
from pathlib import Path
from typing import Any

import aiohttp

AUTH_URL = "https://auth.tidal.com/v1/oauth2"
SCOPE = "r_usr+w_usr+w_sub"

# Mobile-app client credentials (public knowledge — see e.g. tidal-dl).
CLIENT_ID = base64.b64decode("ZlgySnhkbW50WldLMGl4VA==").decode("ascii")
CLIENT_SECRET = base64.b64decode(
    "MU5tNUFmREFqeHJnSkZKYktOV0xlQXlLR1ZHbUlOdVhQUExIVlhBdnhBZz0=",
).decode("ascii")

CONFIG_DIR = Path.home() / ".config" / "tidal"
CREDENTIALS_FILE = CONFIG_DIR / "credentials.json"


def _basic_auth() -> aiohttp.BasicAuth:
    return aiohttp.BasicAuth(login=CLIENT_ID, password=CLIENT_SECRET)


# -- Credentials file helpers ----------------------------------------------


def save_credentials(creds: dict, path: Path | None = None) -> Path:
    """Persist credentials to disk in ~/.config/tidal/credentials.json."""
    target = path or CREDENTIALS_FILE
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(json.dumps(creds, indent=2))
    return target


def load_credentials(path: Path | None = None) -> dict | None:
    """Load credentials from disk, or None if not present."""
    target = path or CREDENTIALS_FILE
    if not target.exists():
        return None
    try:
        return json.loads(target.read_text())
    except (json.JSONDecodeError, OSError):
        return None


# -- Device-code OAuth flow ------------------------------------------------


async def request_device_code(
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Initiate the device-code OAuth flow.

    Returns a dict with ``deviceCode``, ``userCode``,
    ``verificationUriComplete``, ``expiresIn``, ``interval``.
    Open ``verificationUriComplete`` in a browser to authorize.
    """
    own_session = session is None
    if own_session:
        session = aiohttp.ClientSession()
    try:
        async with session.post(  # type: ignore[union-attr]
            f"{AUTH_URL}/device_authorization",
            data={"client_id": CLIENT_ID, "scope": SCOPE},
        ) as resp:
            body = await resp.json(content_type=None)
            if resp.status >= 400:
                raise RuntimeError(f"device_authorization failed: {body}")
            return body
    finally:
        if own_session:
            await session.close()  # type: ignore[union-attr]


async def poll_device_code(
    device_code: str,
    *,
    session: aiohttp.ClientSession | None = None,
) -> tuple[int, dict]:
    """Poll the token endpoint to check if the user has authorized yet.

    Returns ``(status_code, response_dict)``. Status codes:
        0  — authorized; response contains ``access_token``, ``refresh_token``,
             ``expires_in``, ``user``.
        1  — generic error / not yet authorized.
        2  — pending (still waiting for the user to authorize in browser).
    """
    own_session = session is None
    if own_session:
        session = aiohttp.ClientSession()
    try:
        async with session.post(  # type: ignore[union-attr]
            f"{AUTH_URL}/token",
            data={
                "client_id": CLIENT_ID,
                "device_code": device_code,
                "grant_type": "urn:ietf:params:oauth:grant-type:device_code",
                "scope": SCOPE,
            },
            auth=_basic_auth(),
        ) as resp:
            body = await resp.json(content_type=None)
    finally:
        if own_session:
            await session.close()  # type: ignore[union-attr]

    if "status" in body and body["status"] != 200:
        if body.get("status") == 400 and body.get("sub_status") == 1002:
            return 2, {}
        return 1, body

    user = body.get("user", {})
    return 0, {
        "user_id": user.get("userId"),
        "country_code": user.get("countryCode"),
        "access_token": body["access_token"],
        "refresh_token": body["refresh_token"],
        "token_expiry": body["expires_in"] + time.time(),
    }


async def refresh_access_token(
    refresh_token: str,
    *,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Exchange a refresh token for a new access token.

    Returns ``{access_token, token_expiry, refresh_token}``. The refresh
    token may be rotated; the caller should persist whatever's returned.
    """
    own_session = session is None
    if own_session:
        session = aiohttp.ClientSession()
    try:
        async with session.post(  # type: ignore[union-attr]
            f"{AUTH_URL}/token",
            data={
                "client_id": CLIENT_ID,
                "refresh_token": refresh_token,
                "grant_type": "refresh_token",
                "scope": SCOPE,
            },
            auth=_basic_auth(),
        ) as resp:
            body = await resp.json(content_type=None)
            if resp.status >= 400:
                raise RuntimeError(f"refresh failed: {body}")
    finally:
        if own_session:
            await session.close()  # type: ignore[union-attr]

    return {
        "access_token": body["access_token"],
        "token_expiry": body["expires_in"] + time.time(),
        # Tidal sometimes rotates the refresh token, sometimes doesn't.
        "refresh_token": body.get("refresh_token", refresh_token),
    }
