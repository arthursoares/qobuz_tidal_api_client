# Qobuz & Tidal API Clients

Unofficial clients for the Qobuz and Tidal music APIs — Go and Python libraries with a matching facade-shape (`client.catalog`, `client.favorites`, `client.playlists`, `client.streaming`), a CLI binary for Qobuz, and end-to-end album downloaders that handle tagging, retries, dedup, and MQA decryption.

API surfaces validated from real Proxyman captures of the official clients.

## What's in the box

| | Qobuz | Tidal |
|---|---|---|
| **Python** (`clients/python/`) | `qobuz` — full API + downloader + CLI | `tidal` — full API + downloader (v1) |
| **Go** (`clients/go/`) | full API + CLI binary | full API (v2, read-only) |

**Downloads are in Python only.** The Go Tidal client targets Tidal's public OpenAPI v2, which is metadata-only — it has Favorites / Playlists / Catalog / Search but no playback endpoint. The Python Tidal SDK targets the legacy v1 API (`api.tidalhifi.com/v1`), the only Tidal API that exposes `playbackinfopostpaywall`, so it can do downloads including MQA AES-CTR decryption. Pick the client that matches your language *and* your use case.

## CLI (Qobuz)

### Install

```bash
# Go (builds a single binary)
cd clients/go && go build -o ~/bin/qobuz ./cmd/qobuz

# Python
pip install -e clients/python
```

### Authenticate

```bash
# On a machine with a browser
qobuz login

# On a headless/remote machine
qobuz login --no-browser
# Opens a URL you can paste into any browser (phone, laptop)
# After login, paste the redirect URL back into the terminal
```

Credentials are saved to `~/.config/qobuz/credentials.json` (permissions `0600`).

```bash
qobuz status    # show who you're logged in as
qobuz token     # print the auth token (pipe to scripts)
```

### Favorites

```bash
qobuz favorites list                  # list favorite albums
qobuz favorites list --limit 100      # with custom limit
qobuz favorites artists               # list favorite artists
qobuz favorites tracks                # list favorite tracks

qobuz favorites add <album_id>        # add album to favorites
qobuz favorites remove <album_id>     # remove album
qobuz favorites add-track <track_id>  # love a track
qobuz favorites remove-track <id>     # unlove a track
qobuz favorites add-artist <id>       # follow an artist
qobuz favorites remove-artist <id>    # unfollow
```

### Playlists

```bash
qobuz playlists list                              # list your playlists
qobuz playlists create "Chill Vibes"              # create private playlist
qobuz playlists create "Public Mix" --public      # create public playlist
qobuz playlists show <playlist_id>                # show tracks in a playlist
qobuz playlists rename <playlist_id> New Name     # rename
qobuz playlists add-tracks <playlist_id> <track_id> [track_id...]  # add tracks
qobuz playlists delete <playlist_id>              # delete
```

### Search

```bash
qobuz search albums "radiohead ok computer"
qobuz search tracks "everything in its right place"
qobuz search artists "talk talk"
```

### Catalog

```bash
qobuz album <album_id>     # album details (label, genre, quality, track count)
qobuz artist <artist_id>   # artist page with releases
```

### Discovery

```bash
qobuz genres                          # list all genres
qobuz new-releases                    # browse new releases
qobuz new-releases --genre 80         # new jazz releases
```

### Shortcuts

`fav` = `favorites`, `pl` = `playlists`

```bash
qobuz fav list
qobuz pl show 13723132
```

## Python Library

```bash
# Qobuz only
pip install -e clients/python

# Qobuz + Tidal (two separate packages, installed side-by-side)
pip install -e clients/python
pip install -e clients/python/tidal
```

### Qobuz

```python
import asyncio
from qobuz import QobuzClient, AlbumDownloader, DownloadConfig

async def main():
    # Auth from saved credentials (after `qobuz login`)
    async with QobuzClient.from_credentials() as client:
        # Favorites
        favorites = await client.favorites.get_albums(limit=50)
        for album in favorites.items:
            print(f"{album.artist.name} — {album.title}")

        await client.favorites.add_album("0825646233397")

        # Playlists
        playlist = await client.playlists.create("New Playlist")
        await client.playlists.add_tracks(playlist.id, ["33967376", "33967377"])

        # Search
        results = await client.catalog.search_albums("radiohead")

        # Discovery
        genres = await client.discovery.list_genres()
        new = await client.discovery.new_releases(genre_ids=[80])

        # Download an album with live progress callbacks
        config = DownloadConfig(output_dir="/music", quality=3)
        downloader = AlbumDownloader(
            client, config,
            on_track_start=lambda n, title: print(f"→ {n}. {title}"),
            on_track_progress=lambda n, done, total: None,
            on_track_complete=lambda n, title, ok: print(f"  {'✓' if ok else '✗'} {title}"),
        )
        result = await downloader.download("0825646233397")
        print(f"Downloaded {result.successful}/{result.total} tracks")

asyncio.run(main())
```

Or with a manual token:

```python
async with QobuzClient(app_id="304027809", user_auth_token="YOUR_TOKEN") as client:
    ...
```

#### Qobuz API Namespaces

| Namespace | Operations |
|-----------|-----------|
| `client.favorites` | add/remove albums, tracks, artists; list favorites; get all IDs |
| `client.playlists` | create, update, delete; add tracks (auto-batched); list, search |
| `client.catalog` | get/search albums, artists, tracks; `get_album_with_tracks` (for downloader); batch lookup; suggestions |
| `client.discovery` | genres, new releases, curated playlists, ideal discography |
| `client.streaming` | file URLs (signed), sessions, playback reporting |
| `client.last_update()` | poll for library changes (timestamps per section) |
| `client.login()` | validate token, get user profile |

Plus the top-level `AlbumDownloader` / `DownloadConfig` / `AlbumResult` / `TrackResult` classes for orchestrating full album downloads, and the `spoofer` module (`fetch_app_credentials`, `find_working_secret`) for request signing.

### Tidal

```python
import asyncio
from tidal import TidalClient, AlbumDownloader, DownloadConfig

async def main():
    # Construct from saved credentials (~/.config/tidal/credentials.json)
    async with TidalClient.from_credentials() as client:
        # Favorites
        page = await client.favorites.get_albums(limit=50)
        for item in page.items:
            album = item["item"]  # raw Tidal envelope
            print(f"{album['artist']['name']} — {album['title']}")

        # Catalog with typed models
        album, tracks = await client.catalog.get_album_with_tracks(67890)
        print(f"{album.title} ({album.number_of_tracks} tracks)")

        # Search
        results = await client.catalog.search_albums("radiohead")

        # Streaming manifest (falls back to lower quality on failure)
        manifest = await client.streaming.get_manifest(1234567, quality=3)
        print(f"Codec: {manifest.codec}, encrypted: {manifest.is_encrypted}")

        # Download an album (handles MQA AES-CTR decryption automatically)
        config = DownloadConfig(output_dir="/music", quality=3)
        result = await AlbumDownloader(client, config).download(67890)
        print(f"Downloaded {result.successful}/{result.total} tracks")

asyncio.run(main())
```

Or with explicit tokens:

```python
async with TidalClient(
    access_token="...",
    refresh_token="...",
    user_id=12345,
    country_code="US",
) as client:
    ...
```

The Tidal client automatically refreshes any access token with less than 24 hours of lifetime remaining on `__aenter__`, and also has a 401 retry hook on the HTTP transport — so once valid credentials are set, expired tokens are handled transparently.

#### Tidal API Namespaces

| Namespace | Operations |
|-----------|-----------|
| `client.favorites` | `get_albums` / `get_tracks` / `get_artists` (paginated); `all_albums()` walks full pagination |
| `client.catalog` | `get_album` / `get_album_with_tracks` / `get_track` / `get_track_lyrics`; `search_albums/tracks/artists`; artist endpoints |
| `client.streaming` | `get_manifest(track_id, quality)` — calls `playbackinfopostpaywall`, decodes the base64 manifest, falls back through quality tiers on failure |

Plus the top-level `AlbumDownloader` / `DownloadConfig` / `AlbumResult` / `TrackResult` classes, and the `auth` module for device-code OAuth (`request_device_code`, `poll_device_code`, `refresh_access_token`) and credentials file helpers.

#### Tidal v1 vs v2

The Python `tidal` package targets **Tidal v1** (`api.tidalhifi.com/v1`) — the legacy/internal API that the mobile app uses. It's the only Tidal API with a playback endpoint, so it's the only path to downloads. v1 auth uses the mobile-app device-code OAuth2 flow with hard-coded client credentials (public knowledge, extracted from the iOS app).

The Go `tidal` package (below) targets **Tidal v2** (`openapi.tidal.com/v2`), the public OpenAPI, which is metadata-only. Use it when you only need library reads and want to stay on the supported public API.

## Go Library

### Qobuz

```go
import qobuz "github.com/arthursoares/qobuz_api_client/clients/go"

// From saved credentials
client, err := qobuz.NewClientFromCredentials()

// Or with explicit token
client := qobuz.NewClient("304027809", "YOUR_TOKEN",
    qobuz.WithRateLimit(1.0, 10),
)

// All methods take context.Context
ctx := context.Background()

// Favorites
albums, _ := client.Favorites.GetAlbums(ctx, 50, 0)
client.Favorites.AddAlbum(ctx, "0825646233397")

// Playlists
pl, _ := client.Playlists.Create(ctx, "New Playlist", "", false, false)
client.Playlists.AddTracks(ctx, pl.ID, []string{"33967376"}, true)

// Search
results, _ := client.Catalog.SearchAlbums(ctx, "radiohead", 20, 0)

// Discovery
genres, _ := client.Discovery.ListGenres(ctx)
```

### Tidal (v2, read-only)

```go
import "github.com/arthursoares/qobuz_api_client/clients/go/tidal"

client := tidal.NewClient(
    "access-token",
    "US",           // country code
    "user-id",
    tidal.WithRateLimit(1.0, 10),
)

ctx := context.Background()

// Favorites (cursor-based pagination)
albums, cursor, err := client.Favorites.GetAlbums(ctx, 50)

// Catalog
album, err := client.Catalog.GetAlbum(ctx, "album-id")

// Search
results, cursor, err := client.Search.Albums(ctx, "radiohead", 20)

// Playlists
playlists, err := client.Playlists.List(ctx)
```

The Go Tidal client is intentionally read-only and uses the supported public API. For downloads, use the Python `tidal` package.

## Running Tests

```bash
# Python qobuz (122 tests)
cd clients/python && pip install -e ".[dev]" && pytest -v

# Python tidal (116 tests)
cd clients/python/tidal && pip install -e ".[dev]" && pytest -v

# Go — runs both qobuz and tidal test suites from their respective modules
cd clients/go && go test -v ./...           # Go qobuz (90 tests)
cd clients/go/tidal && go test -v ./...     # Go tidal (51 tests)
```

## API Spec

Full API documentation with endpoint catalog, request/response shapes, and auth flow details (validated from Proxyman captures):

[docs/api-spec.md](docs/api-spec.md)

## Project Structure

```
qobuz_api_client/
├── clients/
│   ├── python/
│   │   ├── qobuz/            # Qobuz Python package (async, aiohttp)
│   │   │   ├── client.py     # QobuzClient facade
│   │   │   ├── downloader.py # AlbumDownloader
│   │   │   ├── spoofer.py    # App credential spoofing for request signing
│   │   │   └── ...
│   │   ├── tests/            # Qobuz pytest suite
│   │   └── tidal/            # Tidal Python package (v1, with downloads)
│   │       ├── tidal/        # source
│   │       │   ├── client.py     # TidalClient facade
│   │       │   ├── downloader.py # AlbumDownloader + MQA AES-CTR decryption
│   │       │   ├── catalog.py
│   │       │   ├── favorites.py
│   │       │   ├── streaming.py  # playbackinfopostpaywall manifest decoding
│   │       │   └── ...
│   │       └── tests/        # Tidal pytest suite
│   └── go/
│       ├── *.go              # Qobuz Go package
│       ├── cmd/qobuz/        # Qobuz CLI binary
│       └── tidal/            # Tidal Go package (v2, read-only)
│           └── *.go
├── docs/
│   └── api-spec.md           # Full API specification
└── README.md
```

## Consumers

- [`arthursoares/libsync`](https://github.com/arthursoares/libsync) — a FastAPI + SvelteKit web UI for managing Qobuz/Tidal libraries (formerly `arthursoares/streamrip`). Uses both Python packages as a git submodule. Source of truth for how these SDKs wire into a real app — see the backend's `services/library.py`, `services/download.py`, and `main.py`.

## Disclaimer

These are **unofficial** clients, reverse-engineered from the behavior of Qobuz's and Tidal's own applications. They are intended for personal use with your own legitimately-purchased streaming subscriptions, and you are responsible for complying with the terms of service of Qobuz and Tidal. The authors take no responsibility for how you use these libraries.

## License

GPL-3.0-only. This code was extracted and refactored from [`nathom/streamrip`](https://github.com/nathom/streamrip) (GPL-3.0) — see the streamrip repo for the original Qobuz/Tidal client logic, tagging helpers, and MQA decryption primitives that seed this project.
