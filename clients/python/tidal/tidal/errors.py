"""Tidal API error hierarchy."""

from __future__ import annotations


class TidalError(Exception):
    """Base exception for all Tidal API errors."""

    def __init__(self, status: int, message: str):
        self.status = status
        self.message = message
        super().__init__(f"[{status}] {message}")


class AuthenticationError(TidalError):
    """401 — Invalid or expired access token."""


class ForbiddenError(TidalError):
    """403 — Insufficient subscription tier for the requested quality."""


class NotFoundError(TidalError):
    """404 — Track / album / playlist does not exist."""


class RateLimitError(TidalError):
    """429 — Too many requests."""


class NonStreamableError(TidalError):
    """The track exists but cannot be streamed (region-locked, restricted, etc.).

    Status defaults to 200 because the underlying request usually succeeds —
    the restriction surfaces inside the response body.
    """

    def __init__(self, message: str, status: int = 200):
        super().__init__(status=status, message=message)


_STATUS_MAP: dict[int, type[TidalError]] = {
    401: AuthenticationError,
    403: ForbiddenError,
    404: NotFoundError,
    429: RateLimitError,
}


def raise_for_status(status: int, body: dict | None = None) -> None:
    """Raise the appropriate TidalError if status indicates failure."""
    if status < 400:
        return
    exc_cls = _STATUS_MAP.get(status, TidalError)
    message = "HTTP error"
    if isinstance(body, dict):
        message = body.get("userMessage") or body.get("description") or body.get("error", message)
    raise exc_cls(status=status, message=str(message))
