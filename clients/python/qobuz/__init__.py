"""Async Python client for the Qobuz API."""

from .auth import load_credentials, login, save_credentials
from .client import QobuzClient
from .downloader import AlbumDownloader, AlbumResult, DownloadConfig, TrackResult
from .spoofer import fetch_app_credentials, find_working_secret
from .errors import (
    AuthenticationError,
    ForbiddenError,
    InvalidAppError,
    NonStreamableError,
    NotFoundError,
    QobuzError,
    RateLimitError,
)
from .types import (
    Album,
    AlbumSummary,
    ArtistRole,
    ArtistSummary,
    AudioInfo,
    Award,
    FavoriteIds,
    FileUrl,
    Genre,
    ImageSet,
    Label,
    LastUpdate,
    PaginatedResult,
    Playlist,
    Rights,
    Session,
    Track,
    UserSummary,
)

__all__ = [
    "QobuzClient",
    "load_credentials", "login", "save_credentials",
    "AlbumDownloader", "AlbumResult", "DownloadConfig", "TrackResult",
    "fetch_app_credentials", "find_working_secret",
    "QobuzError", "AuthenticationError", "ForbiddenError", "InvalidAppError",
    "NonStreamableError", "NotFoundError", "RateLimitError",
    "Album", "AlbumSummary", "ArtistRole", "ArtistSummary", "AudioInfo", "Award",
    "FavoriteIds", "FileUrl", "Genre", "ImageSet", "Label", "LastUpdate",
    "PaginatedResult", "Playlist", "Rights", "Session", "Track", "UserSummary",
]
