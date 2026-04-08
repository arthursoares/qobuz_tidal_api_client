package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	qobuz "github.com/arthursoares/qobuz_api_client/clients/go"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "login":
		cmdLogin()
	case "status":
		cmdStatus()
	case "token":
		cmdToken()
	case "favorites", "fav":
		cmdFavorites()
	case "playlists", "pl":
		cmdPlaylists()
	case "search":
		cmdSearch()
	case "album":
		cmdAlbum()
	case "artist":
		cmdArtist()
	case "genres":
		cmdGenres()
	case "new-releases":
		cmdNewReleases()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: qobuz <command> [args]

Auth:
  login [--no-browser]    Authenticate with Qobuz
  status                  Show authentication status
  token                   Print saved auth token

Favorites:
  favorites list          List favorite albums
  favorites artists       List favorite artists
  favorites tracks        List favorite tracks
  favorites add <id>      Add album to favorites
  favorites remove <id>   Remove album from favorites
  favorites add-track <id>      Add track to favorites
  favorites remove-track <id>   Remove track from favorites
  favorites add-artist <id>     Add artist to favorites
  favorites remove-artist <id>  Remove artist from favorites

Playlists:
  playlists list                      List your playlists
  playlists create <name> [--public]  Create a playlist
  playlists show <id>                 Show playlist details + tracks
  playlists delete <id>               Delete a playlist
  playlists add-tracks <id> <track_ids...>   Add tracks to playlist
  playlists rename <id> <new_name>    Rename a playlist

Search:
  search albums <query>   Search albums
  search tracks <query>   Search tracks
  search artists <query>  Search artists

Catalog:
  album <id>              Show album details
  artist <id>             Show artist page + releases

Discovery:
  genres                  List all genres
  new-releases [--genre <id>]  Browse new releases

Shortcuts: fav = favorites, pl = playlists`)
}

// --- Auth commands ---

func cmdLogin() {
	noBrowser := false
	port := 11111
	for i, arg := range os.Args[2:] {
		if arg == "--no-browser" {
			noBrowser = true
		}
		if arg == "--port" && i+1 < len(os.Args[2:]) {
			p, err := strconv.Atoi(os.Args[i+3])
			if err == nil {
				port = p
			}
		}
	}

	ctx := context.Background()
	url := qobuz.OAuthURL(port)

	var code string
	var err error

	if noBrowser {
		fmt.Printf("\nOpen this URL in a browser on any device:\n\n")
		fmt.Printf("  %s\n\n", url)
		fmt.Printf("After logging in, you'll be redirected to a page that may not load.\n")
		fmt.Printf("Copy the FULL URL from your browser's address bar and paste it here:\n\n")
		fmt.Print("> ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		code, err = qobuz.ExtractCodeFromURL(scanner.Text())
		fatal(err)
	} else {
		fmt.Println("Opening browser for Qobuz login...")
		if browserErr := qobuz.OpenBrowser(url); browserErr != nil {
			fmt.Printf("Could not open browser. Open this URL manually:\n  %s\n\n", url)
		}
		fmt.Printf("Waiting for callback on localhost:%d...\n", port)
		code, err = qobuz.WaitForCallback(port)
		fatal(err)
	}

	fmt.Println("Exchanging code for token...")
	creds, err := qobuz.ExchangeCode(ctx, code)
	fatal(err)
	fatal(qobuz.SaveCredentials(creds))
	fmt.Printf("\nAuthenticated as %s\n", creds.DisplayName)
	fmt.Printf("  Token saved to %s\n", qobuz.CredentialsPath())
}

func cmdStatus() {
	creds := qobuz.LoadCredentials()
	if creds == nil {
		fmt.Println("Not authenticated. Run: qobuz login")
		os.Exit(1)
	}
	fmt.Printf("Logged in as: %s\n", creds.DisplayName)
	fmt.Printf("User ID: %s\n", creds.UserID)
	fmt.Printf("Credentials: %s\n", qobuz.CredentialsPath())
}

func cmdToken() {
	creds := qobuz.LoadCredentials()
	if creds == nil {
		fmt.Fprintln(os.Stderr, "Not authenticated. Run: qobuz login")
		os.Exit(1)
	}
	fmt.Println(creds.UserAuthToken)
}

// --- Favorites commands ---

func cmdFavorites() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: qobuz favorites <list|artists|tracks|add|remove|add-track|remove-track|add-artist|remove-artist> [id]")
		os.Exit(1)
	}

	client := mustClient()
	ctx := context.Background()

	switch os.Args[2] {
	case "list":
		limit := 50
		if v := flagInt("--limit"); v > 0 {
			limit = v
		}
		albums, err := client.Favorites.GetAlbums(ctx, limit, 0)
		fatal(err)
		fmt.Printf("Favorite albums (%d total):\n\n", albums.Total)
		for _, a := range albums.Items {
			hi := ""
			if a.Hires {
				hi = " [Hi-Res]"
			}
			fmt.Printf("  %s — %s%s\n    ID: %s | %d tracks | %s\n\n",
				a.Artist.Name, a.Title, hi, a.ID, a.TracksCount, fmtDuration(a.Duration))
		}

	case "artists":
		result, err := client.Favorites.GetArtists(ctx, 100, 0)
		fatal(err)
		fmt.Printf("Favorite artists (%d):\n\n", len(result.Items))
		for _, item := range result.Items {
			b, _ := json.Marshal(item)
			var a qobuz.ArtistSummary
			json.Unmarshal(b, &a)
			fmt.Printf("  %s (ID: %d)\n", a.Name, a.ID)
		}

	case "tracks":
		result, err := client.Favorites.GetTracks(ctx, 100, 0)
		fatal(err)
		fmt.Printf("Favorite tracks (%d):\n\n", len(result.Items))
		for _, item := range result.Items {
			b, _ := json.Marshal(item)
			var t qobuz.Track
			json.Unmarshal(b, &t)
			fmt.Printf("  %s — %s (%s)\n", t.Performer.Name, t.Title, fmtDuration(t.Duration))
		}

	case "add":
		requireArg(3, "album ID")
		fatal(client.Favorites.AddAlbum(ctx, os.Args[3]))
		fmt.Printf("Added album %s to favorites\n", os.Args[3])

	case "remove":
		requireArg(3, "album ID")
		fatal(client.Favorites.RemoveAlbum(ctx, os.Args[3]))
		fmt.Printf("Removed album %s from favorites\n", os.Args[3])

	case "add-track":
		requireArg(3, "track ID")
		fatal(client.Favorites.AddTrack(ctx, os.Args[3]))
		fmt.Printf("Added track %s to favorites\n", os.Args[3])

	case "remove-track":
		requireArg(3, "track ID")
		fatal(client.Favorites.RemoveTrack(ctx, os.Args[3]))
		fmt.Printf("Removed track %s from favorites\n", os.Args[3])

	case "add-artist":
		requireArg(3, "artist ID")
		fatal(client.Favorites.AddArtist(ctx, os.Args[3]))
		fmt.Printf("Added artist %s to favorites\n", os.Args[3])

	case "remove-artist":
		requireArg(3, "artist ID")
		fatal(client.Favorites.RemoveArtist(ctx, os.Args[3]))
		fmt.Printf("Removed artist %s from favorites\n", os.Args[3])

	default:
		fmt.Fprintf(os.Stderr, "Unknown favorites command: %s\n", os.Args[2])
		os.Exit(1)
	}
}

// --- Playlists commands ---

func cmdPlaylists() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: qobuz playlists <list|create|show|delete|add-tracks|rename>")
		os.Exit(1)
	}

	client := mustClient()
	ctx := context.Background()

	switch os.Args[2] {
	case "list":
		result, err := client.Playlists.List(ctx, 500)
		fatal(err)
		fmt.Printf("Your playlists (%d):\n\n", len(result.Items))
		for _, item := range result.Items {
			b, _ := json.Marshal(item)
			var pl qobuz.Playlist
			json.Unmarshal(b, &pl)
			pub := "private"
			if pl.IsPublic {
				pub = "public"
			}
			fmt.Printf("  %s (%s)\n    ID: %d | %d tracks | %s\n\n",
				pl.Name, pub, pl.ID, pl.TracksCount, fmtDuration(pl.Duration))
		}

	case "create":
		requireArg(3, "playlist name")
		name := os.Args[3]
		public := hasFlag("--public")
		pl, err := client.Playlists.Create(ctx, name, "", public, false)
		fatal(err)
		fmt.Printf("Created playlist: %s (ID: %d)\n", pl.Name, pl.ID)

	case "show":
		requireArg(3, "playlist ID")
		id, err := strconv.Atoi(os.Args[3])
		fatal(err)
		pl, err := client.Playlists.Get(ctx, id, nil)
		fatal(err)
		fmt.Printf("%s (%d tracks, %s)\n", pl.Name, pl.TracksCount, fmtDuration(pl.Duration))
		if pl.Description != "" {
			fmt.Printf("  %s\n", pl.Description)
		}
		if pl.Tracks != nil {
			fmt.Println()
			for i, t := range pl.Tracks.Items {
				fmt.Printf("  %2d. %s — %s (%s)\n", i+1, t.Performer.Name, t.Title, fmtDuration(t.Duration))
			}
		}

	case "delete":
		requireArg(3, "playlist ID")
		id, err := strconv.Atoi(os.Args[3])
		fatal(err)
		fatal(client.Playlists.Delete(ctx, id))
		fmt.Printf("Deleted playlist %d\n", id)

	case "add-tracks":
		requireArg(4, "playlist ID and track IDs")
		id, err := strconv.Atoi(os.Args[3])
		fatal(err)
		trackIDs := os.Args[4:]
		fatal(client.Playlists.AddTracks(ctx, id, trackIDs, true))
		fmt.Printf("Added %d tracks to playlist %d\n", len(trackIDs), id)

	case "rename":
		requireArg(4, "playlist ID and new name")
		id, err := strconv.Atoi(os.Args[3])
		fatal(err)
		name := strings.Join(os.Args[4:], " ")
		pl, err := client.Playlists.Update(ctx, id, &qobuz.PlaylistUpdateOptions{Name: &name})
		fatal(err)
		fmt.Printf("Renamed playlist %d to: %s\n", pl.ID, pl.Name)

	default:
		fmt.Fprintf(os.Stderr, "Unknown playlists command: %s\n", os.Args[2])
		os.Exit(1)
	}
}

// --- Search commands ---

func cmdSearch() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: qobuz search <albums|tracks|artists> <query>")
		os.Exit(1)
	}

	client := mustClient()
	ctx := context.Background()
	query := strings.Join(os.Args[3:], " ")
	limit := 20

	switch os.Args[2] {
	case "albums":
		results, err := client.Catalog.SearchAlbums(ctx, query, limit, 0)
		fatal(err)
		fmt.Printf("Albums matching \"%s\":\n\n", query)
		for _, item := range results.Items {
			b, _ := json.Marshal(item)
			var a qobuz.Album
			json.Unmarshal(b, &a)
			hi := ""
			if a.Hires {
				hi = " [Hi-Res]"
			}
			fmt.Printf("  %s — %s%s\n    ID: %s | %d tracks\n\n", a.Artist.Name, a.Title, hi, a.ID, a.TracksCount)
		}

	case "tracks":
		results, err := client.Catalog.SearchTracks(ctx, query, limit, 0)
		fatal(err)
		fmt.Printf("Tracks matching \"%s\":\n\n", query)
		for _, item := range results.Items {
			b, _ := json.Marshal(item)
			var t qobuz.Track
			json.Unmarshal(b, &t)
			fmt.Printf("  %s — %s (%s)\n    ID: %d\n\n", t.Performer.Name, t.Title, fmtDuration(t.Duration), t.ID)
		}

	case "artists":
		results, err := client.Catalog.SearchArtists(ctx, query, limit, 0)
		fatal(err)
		fmt.Printf("Artists matching \"%s\":\n\n", query)
		for _, item := range results.Items {
			b, _ := json.Marshal(item)
			var a qobuz.ArtistSummary
			json.Unmarshal(b, &a)
			fmt.Printf("  %s (ID: %d)\n", a.Name, a.ID)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown search type: %s (use albums, tracks, artists)\n", os.Args[2])
		os.Exit(1)
	}
}

// --- Catalog commands ---

func cmdAlbum() {
	requireArg(2, "album ID")
	client := mustClient()
	ctx := context.Background()

	album, err := client.Catalog.GetAlbum(ctx, os.Args[2])
	fatal(err)

	hi := ""
	if album.Hires {
		hi = fmt.Sprintf(" [Hi-Res %d-bit/%.0fkHz]", album.MaximumBitDepth, album.MaximumSamplingRate)
	}
	fmt.Printf("%s — %s%s\n", album.Artist.Name, album.Title, hi)
	if album.Label != nil {
		fmt.Printf("  Label: %s\n", album.Label.Name)
	}
	if album.Genre != nil {
		fmt.Printf("  Genre: %s\n", album.Genre.Name)
	}
	if album.ReleaseDateOriginal != nil {
		fmt.Printf("  Released: %s\n", *album.ReleaseDateOriginal)
	}
	fmt.Printf("  %d tracks, %s\n", album.TracksCount, fmtDuration(album.Duration))
	fmt.Printf("  ID: %s | UPC: %s\n", album.ID, ptrStr(album.UPC))
}

func cmdArtist() {
	requireArg(2, "artist ID")
	client := mustClient()
	ctx := context.Background()

	id, err := strconv.Atoi(os.Args[2])
	fatal(err)

	raw, err := client.Catalog.GetArtistPage(ctx, id)
	fatal(err)

	var page struct {
		ID   int `json:"id"`
		Name struct {
			Display string `json:"display"`
		} `json:"name"`
		Biography struct {
			Content string `json:"content"`
		} `json:"biography"`
	}
	json.Unmarshal(raw, &page)
	fmt.Printf("%s (ID: %d)\n", page.Name.Display, page.ID)

	// Get releases
	releases, err := client.Catalog.GetArtistReleases(ctx, id, nil)
	fatal(err)
	fmt.Printf("\nReleases (%d):\n\n", len(releases.Items))
	for _, item := range releases.Items {
		b, _ := json.Marshal(item)
		var a qobuz.Album
		json.Unmarshal(b, &a)
		fmt.Printf("  %s (%s) — %d tracks\n", a.Title, ptrStr(a.ReleaseDateOriginal), a.TracksCount)
	}
}

// --- Discovery commands ---

func cmdGenres() {
	client := mustClient()
	ctx := context.Background()

	genres, err := client.Discovery.ListGenres(ctx)
	fatal(err)
	fmt.Println("Genres:")
	fmt.Println()
	for _, g := range genres {
		fmt.Printf("  [%d] %s\n", g.ID, g.Name)
	}
}

func cmdNewReleases() {
	client := mustClient()
	ctx := context.Background()

	var genreIDs []int
	if v := flagInt("--genre"); v > 0 {
		genreIDs = []int{v}
	}

	results, err := client.Discovery.NewReleases(ctx, genreIDs, 0, 20)
	fatal(err)
	fmt.Println("New Releases:")
	fmt.Println()
	for _, raw := range results.Items {
		var m map[string]any
		json.Unmarshal(raw, &m)
		title, _ := m["title"].(string)
		id := fmt.Sprint(m["id"])
		artist := ""
		if artists, ok := m["artists"].([]any); ok && len(artists) > 0 {
			if a, ok := artists[0].(map[string]any); ok {
				artist, _ = a["name"].(string)
			}
		}
		if artist == "" {
			if a, ok := m["artist"].(map[string]any); ok {
				artist, _ = a["name"].(string)
			}
		}
		hi := ""
		if ai, ok := m["audio_info"].(map[string]any); ok {
			if bd, ok := ai["maximum_bit_depth"].(float64); ok && bd > 16 {
				hi = " [Hi-Res]"
			}
		}
		fmt.Printf("  %s — %s%s\n    ID: %s\n\n", artist, title, hi, id)
	}
}

// --- Helpers ---

func mustClient() *qobuz.Client {
	client, err := qobuz.NewClientFromCredentials(qobuz.WithRateLimit(1.0, 10))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return client
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func requireArg(idx int, name string) {
	if len(os.Args) <= idx {
		fmt.Fprintf(os.Stderr, "Missing required argument: %s\n", name)
		os.Exit(1)
	}
}

func hasFlag(flag string) bool {
	for _, arg := range os.Args {
		if arg == flag {
			return true
		}
	}
	return false
}

func flagInt(flag string) int {
	for i, arg := range os.Args {
		if arg == flag && i+1 < len(os.Args) {
			v, err := strconv.Atoi(os.Args[i+1])
			if err == nil {
				return v
			}
		}
	}
	return 0
}

func fmtDuration(secs int) string {
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
