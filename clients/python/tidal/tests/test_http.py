"""Tests for HttpTransport — rate limiter, auth refresh, 401 retry."""

import pytest
from unittest.mock import AsyncMock

from tidal._http import HttpTransport
from tidal.errors import AuthenticationError


class _FakeResponse:
    def __init__(self, status: int, body: dict | None = None):
        self.status = status
        self._body = body or {}

    async def json(self, content_type=None):
        return self._body

    async def __aenter__(self):
        return self

    async def __aexit__(self, *a):
        return None


class _FakeSession:
    def __init__(self, responses: list[_FakeResponse]):
        self._responses = responses
        self.requests: list[dict] = []

    def request(self, method, url, *, params, data, json, headers):
        self.requests.append(
            {
                "method": method,
                "url": url,
                "params": params,
                "data": data,
                "json": json,
                "headers": headers,
            }
        )
        return self._responses.pop(0)

    async def close(self):
        pass


@pytest.fixture
def transport() -> HttpTransport:
    return HttpTransport(
        access_token="initial-token", country_code="US", requests_per_minute=240
    )


async def test_inject_country_code_into_every_request(transport: HttpTransport):
    transport._session = _FakeSession([_FakeResponse(200, {"ok": True})])  # type: ignore
    await transport.get("albums/1", {"limit": 10})
    assert transport._session.requests[0]["params"] == {"countryCode": "US", "limit": 10}


async def test_bearer_header_uses_current_token(transport: HttpTransport):
    transport._session = _FakeSession([_FakeResponse(200, {})])  # type: ignore
    await transport.get("albums/1")
    assert (
        transport._session.requests[0]["headers"]["Authorization"]
        == "Bearer initial-token"
    )


async def test_set_access_token_takes_effect_immediately(transport: HttpTransport):
    transport._session = _FakeSession(  # type: ignore
        [_FakeResponse(200, {}), _FakeResponse(200, {})]
    )
    await transport.get("albums/1")
    transport.set_access_token("new-token")
    await transport.get("albums/2")
    assert (
        transport._session.requests[1]["headers"]["Authorization"] == "Bearer new-token"
    )


# ---------------------------------------------------------------------------
# 401 retry flow
# ---------------------------------------------------------------------------


async def test_401_triggers_refresh_callback_and_retries(transport: HttpTransport):
    """When the API returns 401, the callback should run and the request retry."""
    transport._session = _FakeSession(  # type: ignore
        [
            _FakeResponse(401, {"userMessage": "expired"}),
            _FakeResponse(200, {"ok": True}),
        ]
    )

    refresh = AsyncMock(return_value=True)

    async def _cb():
        # Simulate the callback rotating the token.
        transport.set_access_token("fresh-token")
        return await refresh()

    transport.set_refresh_callback(_cb)

    status, body = await transport.get("albums/1")
    assert status == 200
    assert body == {"ok": True}
    refresh.assert_awaited_once()
    # Second request should have used the fresh token.
    assert (
        transport._session.requests[1]["headers"]["Authorization"]
        == "Bearer fresh-token"
    )


async def test_401_without_callback_raises(transport: HttpTransport):
    transport._session = _FakeSession(  # type: ignore
        [_FakeResponse(401, {"userMessage": "expired"})]
    )

    with pytest.raises(AuthenticationError):
        await transport.get("albums/1")


async def test_401_callback_failure_surfaces_original_error(transport: HttpTransport):
    """If the refresh callback raises, we don't loop — surface the 401."""
    transport._session = _FakeSession(  # type: ignore
        [_FakeResponse(401, {"userMessage": "expired"})]
    )

    async def _bad_cb():
        raise RuntimeError("refresh server down")

    transport.set_refresh_callback(_bad_cb)

    with pytest.raises(AuthenticationError):
        await transport.get("albums/1")


async def test_401_callback_returning_false_surfaces_error(transport: HttpTransport):
    transport._session = _FakeSession(  # type: ignore
        [_FakeResponse(401, {"userMessage": "expired"})]
    )

    async def _cb():
        return False

    transport.set_refresh_callback(_cb)

    with pytest.raises(AuthenticationError):
        await transport.get("albums/1")


async def test_raise_for_status_maps_404(transport: HttpTransport):
    transport._session = _FakeSession([_FakeResponse(404, {"userMessage": "nope"})])  # type: ignore

    from tidal.errors import NotFoundError

    with pytest.raises(NotFoundError):
        await transport.get("albums/1")


async def test_raise_errors_false_returns_status(transport: HttpTransport):
    transport._session = _FakeSession([_FakeResponse(500, {"error": "oops"})])  # type: ignore

    status, body = await transport.get("albums/1", raise_errors=False)
    assert status == 500
    assert body == {"error": "oops"}


async def test_transport_must_be_context_manager(transport: HttpTransport):
    with pytest.raises(RuntimeError, match="async context manager"):
        await transport.get("albums/1")
