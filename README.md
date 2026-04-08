# Qobuz API Client

Unofficial Qobuz API client with a CLI and libraries for Python and Go. Manage your Qobuz library (favorites, playlists), browse the catalog, and stream — all from the command line or your own code.

API surface validated from real Proxyman captures of the Qobuz desktop app.

## CLI

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
pip install -e clients/python
```

```python
import asyncio
from qobuz import QobuzClient

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

asyncio.run(main())
```

Or with a manual token:

```python
async with QobuzClient(app_id="304027809", user_auth_token="YOUR_TOKEN") as client:
    ...
```

### API Namespaces

| Namespace | Operations |
|-----------|-----------|
| `client.favorites` | add/remove albums, tracks, artists; list favorites; get all IDs |
| `client.playlists` | create, update, delete; add tracks (auto-batched); list, search |
| `client.catalog` | get/search albums, artists, tracks; batch lookup; suggestions |
| `client.discovery` | genres, new releases, curated playlists, ideal discography |
| `client.streaming` | file URLs (signed), sessions, playback reporting |
| `client.last_update()` | poll for library changes (timestamps per section) |
| `client.login()` | validate token, get user profile |

## Go Library

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

## Running Tests

```bash
# Python (107 tests)
cd clients/python && pip install -e ".[dev]" && pytest -v

# Go (86 tests)
cd clients/go && go test -v
```

## API Spec

Full API documentation with endpoint catalog, request/response shapes, and auth flow details (validated from Proxyman captures):

[docs/api-spec.md](docs/api-spec.md)

## Project Structure

```
qobuz_api_client/
├── clients/
│   ├── python/          # Async Python client (aiohttp)
│   │   ├── qobuz/      # Library package
│   │   └── tests/      # pytest suite
│   └── go/              # Go client (net/http, stdlib)
│       ├── *.go         # Library package
│       └── cmd/qobuz/  # CLI binary
├── docs/
│   └── api-spec.md      # Full API specification
└── README.md
```
