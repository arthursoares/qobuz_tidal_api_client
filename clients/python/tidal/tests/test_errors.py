"""Tests for the Tidal error hierarchy and status mapping."""

import pytest

from tidal.errors import (
    AuthenticationError,
    ForbiddenError,
    NonStreamableError,
    NotFoundError,
    RateLimitError,
    TidalError,
    raise_for_status,
)


class TestRaiseForStatus:
    def test_passes_through_2xx(self):
        # Should not raise
        raise_for_status(200, {})
        raise_for_status(204, None)

    def test_passes_through_3xx(self):
        raise_for_status(304, {})

    def test_401_raises_authentication(self):
        with pytest.raises(AuthenticationError) as exc:
            raise_for_status(401, {"userMessage": "expired"})
        assert exc.value.status == 401
        assert "expired" in exc.value.message

    def test_403_raises_forbidden(self):
        with pytest.raises(ForbiddenError):
            raise_for_status(403, {"userMessage": "subscription required"})

    def test_404_raises_notfound(self):
        with pytest.raises(NotFoundError):
            raise_for_status(404, {"userMessage": "not found"})

    def test_429_raises_ratelimit(self):
        with pytest.raises(RateLimitError):
            raise_for_status(429, {})

    def test_other_4xx_raises_base(self):
        with pytest.raises(TidalError) as exc:
            raise_for_status(418, {"userMessage": "I'm a teapot"})
        # Base class, not a subclass
        assert type(exc.value) is TidalError

    def test_5xx_raises_base(self):
        with pytest.raises(TidalError):
            raise_for_status(500, {})

    def test_extracts_message_from_description(self):
        with pytest.raises(NotFoundError) as exc:
            raise_for_status(404, {"description": "track gone"})
        assert "track gone" in exc.value.message

    def test_extracts_message_from_error(self):
        with pytest.raises(NotFoundError) as exc:
            raise_for_status(404, {"error": "boom"})
        assert "boom" in exc.value.message

    def test_falls_back_to_default_message(self):
        with pytest.raises(NotFoundError) as exc:
            raise_for_status(404, {})
        assert exc.value.message == "HTTP error"

    def test_handles_none_body(self):
        with pytest.raises(NotFoundError):
            raise_for_status(404, None)


class TestNonStreamableError:
    def test_default_status_is_200(self):
        exc = NonStreamableError("region locked")
        assert exc.status == 200
        assert "region locked" in exc.message

    def test_can_override_status(self):
        exc = NonStreamableError("forbidden", status=403)
        assert exc.status == 403

    def test_inherits_from_tidal_error(self):
        assert isinstance(NonStreamableError("x"), TidalError)
