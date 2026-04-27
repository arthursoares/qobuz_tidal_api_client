"""Tidal authentication helpers — device-code + PKCE OAuth + token refresh.

Two OAuth flows live here, picked by *audio entitlement*:

- **Device-code** (legacy iOS-app credentials): simple, headless, but the
  client identity itself is capped at AAC ~320kbps regardless of the
  user's subscription. Use only when HiRes/Lossless isn't required.
- **PKCE** (Authorization Code + PKCE, redirect-based): unlocks LOSSLESS,
  HI_RES, and HI_RES_LOSSLESS for users whose subscription includes them.
  Same client_id/redirect_uri as upstream ``python-tidal`` so the same
  Tidal-side allowlist applies.

Both client credentials are public knowledge in the Tidal-tooling
community (extracted from official Tidal apps).
"""

from __future__ import annotations

import base64
import hashlib
import json
import os
import secrets
import time
from pathlib import Path
from urllib.parse import parse_qs, urlencode, urlsplit

import aiohttp

AUTH_URL = "https://auth.tidal.com/v1/oauth2"
PKCE_AUTHORIZE_URL = "https://login.tidal.com/authorize"
PKCE_REDIRECT_URI = "https://tidal.com/android/login/auth"
SCOPE = "r_usr+w_usr+w_sub"

# -- Device-code client (320k AAC max — kept for back-compat) --------------
CLIENT_ID = base64.b64decode("ZlgySnhkbW50WldLMGl4VA==").decode("ascii")
CLIENT_SECRET = base64.b64decode(
    "MU5tNUFmREFqeHJnSkZKYktOV0xlQXlLR1ZHbUlOdVhQUExIVlhBdnhBZz0=",
).decode("ascii")

# -- PKCE client (unlocks LOSSLESS / HI_RES / HI_RES_LOSSLESS per sub) -----
# Mirrors python-tidal's ``client_id_pkce`` / ``client_secret_pkce``. The
# secret is sent on the *refresh* call (Tidal doesn't follow textbook PKCE
# here) but not on the initial token-exchange — that one uses the verifier.
CLIENT_ID_PKCE = base64.b64decode("NkJEU1JkcEs5aHFFQlRnVQ==").decode("ascii")
CLIENT_SECRET_PKCE = base64.b64decode(
    "eGV1UG1ZN25icFo5SUliTEFjUTkzc2hrYTFWTmhlVUFxTjZJY3N6alRHOD0=",
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


# -- PKCE OAuth flow (HiRes-capable) --------------------------------------


def generate_pkce_pair() -> tuple[str, str, str]:
    """Generate a fresh PKCE verifier + S256 challenge + client_unique_key.

    The verifier is a 32-byte URL-safe random string; the challenge is the
    base64-url-encoded SHA-256 of the verifier (no padding). The
    ``client_unique_key`` is a 64-bit hex blob Tidal binds to the auth
    request — it must match between the authorize URL and the token
    exchange. Returns ``(verifier, challenge, client_unique_key)``.
    """
    verifier = base64.urlsafe_b64encode(os.urandom(32)).rstrip(b"=").decode("ascii")
    challenge = (
        base64.urlsafe_b64encode(hashlib.sha256(verifier.encode("ascii")).digest())
        .rstrip(b"=")
        .decode("ascii")
    )
    unique_key = secrets.token_hex(8)
    return verifier, challenge, unique_key


def build_pkce_authorize_url(challenge: str, unique_key: str) -> str:
    """Return the URL the user opens in a browser to authorize via PKCE."""
    params = {
        "response_type": "code",
        "redirect_uri": PKCE_REDIRECT_URI,
        "client_id": CLIENT_ID_PKCE,
        "lang": "EN",
        "appMode": "android",
        "client_unique_key": unique_key,
        "code_challenge": challenge,
        "code_challenge_method": "S256",
        "restrict_signup": "true",
    }
    return f"{PKCE_AUTHORIZE_URL}?{urlencode(params)}"


def extract_code_from_redirect(redirect_url: str) -> str:
    """Pull the ``code`` query param out of the URL the user pasted back.

    The Tidal redirect lands on a stub page (``tidal.com/android/login/auth``)
    with the auth code in the query string. We don't actually own that
    page — it 404s — but the user can copy the URL from their address bar.
    """
    if not redirect_url or "code=" not in redirect_url:
        raise ValueError(
            "Pasted URL doesn't contain an auth code. Make sure you copied "
            "the full URL from the browser address bar after Tidal redirected."
        )
    qs = parse_qs(urlsplit(redirect_url).query)
    codes = qs.get("code")
    if not codes:
        raise ValueError("Pasted URL has no `code` query parameter")
    return codes[0]


async def exchange_pkce_code(
    code: str,
    verifier: str,
    unique_key: str,
    *,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Exchange a PKCE auth code for access + refresh tokens.

    The PKCE token exchange uses the verifier (not a client_secret) — that's
    the point of PKCE. The unique_key must match what was sent in the
    authorize URL. Returns the same shape as ``poll_for_token``.
    """
    own_session = session is None
    if own_session:
        session = aiohttp.ClientSession()
    try:
        async with session.post(  # type: ignore[union-attr]
            f"{AUTH_URL}/token",
            data={
                "code": code,
                "client_id": CLIENT_ID_PKCE,
                "grant_type": "authorization_code",
                "redirect_uri": PKCE_REDIRECT_URI,
                "scope": SCOPE,
                "code_verifier": verifier,
                "client_unique_key": unique_key,
            },
        ) as resp:
            body = await resp.json(content_type=None)
            if resp.status >= 400:
                raise RuntimeError(f"PKCE token exchange failed: {body}")
    finally:
        if own_session:
            await session.close()  # type: ignore[union-attr]

    user = body.get("user", {})
    return {
        "user_id": user.get("userId"),
        "country_code": user.get("countryCode"),
        "access_token": body["access_token"],
        "refresh_token": body.get("refresh_token", ""),
        "token_expiry": body["expires_in"] + time.time(),
        # Marker so refresh knows to use the PKCE client_id+secret pair
        # rather than the legacy device-code one.
        "auth_method": "pkce",
    }


async def refresh_pkce_token(
    refresh_token: str,
    *,
    session: aiohttp.ClientSession | None = None,
) -> dict:
    """Refresh a PKCE-issued access token. Uses the PKCE client credentials."""
    own_session = session is None
    if own_session:
        session = aiohttp.ClientSession()
    try:
        async with session.post(  # type: ignore[union-attr]
            f"{AUTH_URL}/token",
            data={
                "client_id": CLIENT_ID_PKCE,
                "client_secret": CLIENT_SECRET_PKCE,
                "refresh_token": refresh_token,
                "grant_type": "refresh_token",
                "scope": SCOPE,
            },
        ) as resp:
            body = await resp.json(content_type=None)
            if resp.status >= 400:
                raise RuntimeError(f"PKCE refresh failed: {body}")
    finally:
        if own_session:
            await session.close()  # type: ignore[union-attr]

    return {
        "access_token": body["access_token"],
        "token_expiry": body["expires_in"] + time.time(),
        "refresh_token": body.get("refresh_token", refresh_token),
    }
