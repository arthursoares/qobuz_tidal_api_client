package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	tidal "github.com/arthursoares/qobuz_api_client/clients/go/tidal"
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
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: tidal <command> [args]

Auth:
  login [--no-browser] [--client-id <id>]  Authenticate with Tidal (OAuth2 PKCE)
  status                                    Show authentication status
  token                                     Print saved access token

Favorites:
  favorites list              List favorite albums
  favorites artists           List favorite artists
  favorites tracks            List favorite tracks
  favorites add <id>          Add album to favorites
  favorites remove <id>       Remove album from favorites
  favorites add-track <id>    Add track to favorites
  favorites remove-track <id> Remove track from favorites
  favorites add-artist <id>   Add artist to favorites
  favorites remove-artist <id> Remove artist from favorites

Playlists:
  playlists list                         List your playlists
  playlists create <name> [--public]     Create a playlist
  playlists show <id>                    Show playlist details + tracks
  playlists delete <id>                  Delete a playlist
  playlists add-tracks <id> <track_ids...>  Add tracks to playlist
  playlists rename <id> <new_name>       Rename a playlist

Search:
  search albums <query>    Search albums
  search tracks <query>    Search tracks
  search artists <query>   Search artists

Catalog:
  album <id>               Show album details
  artist <id>              Show artist page + albums
  genres                   List all genres

Shortcuts: fav = favorites, pl = playlists`)
}

// --- Auth commands ---

func cmdLogin() {
	noBrowser := false
	clientID := flagStr("--client-id")
	port := 11111

	for _, arg := range os.Args[2:] {
		if arg == "--no-browser" {
			noBrowser = true
		}
	}

	if clientID == "" {
		// Check if saved credentials have a client ID
		creds := tidal.LoadCredentials()
		if creds != nil && creds.ClientID != "" {
			clientID = creds.ClientID
		} else {
			fmt.Print("Enter your Tidal Client ID (from developer.tidal.com): ")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			clientID = strings.TrimSpace(scanner.Text())
			if clientID == "" {
				fmt.Fprintln(os.Stderr, "Client ID is required")
				os.Exit(1)
			}
		}
	}

	ctx := context.Background()

	pkce, err := tidal.GeneratePKCE()
	fatal(err)

	authURL := tidal.AuthorizeURL(clientID, port, pkce)

	var code string

	if noBrowser {
		fmt.Printf("\nOpen this URL in a browser on any device:\n\n")
		fmt.Printf("  %s\n\n", authURL)
		fmt.Printf("After logging in, you'll be redirected to a page that may not load.\n")
		fmt.Printf("Copy the FULL URL from your browser's address bar and paste it here:\n\n")
		fmt.Print("> ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		rawURL := strings.TrimSpace(scanner.Text())
		// Parse the URL properly so percent-encoded codes are decoded once.
		parsed, err := url.Parse(rawURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not parse URL: %v\n", err)
			os.Exit(1)
		}
		code = parsed.Query().Get("code")
		if code == "" {
			fmt.Fprintln(os.Stderr, "Could not extract code from URL (missing ?code= parameter)")
			os.Exit(1)
		}
	} else {
		fmt.Println("Opening browser for Tidal login...")
		if browserErr := tidal.OpenBrowser(authURL); browserErr != nil {
			fmt.Printf("Could not open browser. Open this URL manually:\n  %s\n\n", authURL)
		}
		fmt.Printf("Waiting for callback on localhost:%d...\n", port)
		code, err = tidal.WaitForCallback(port)
		fatal(err)
	}

	fmt.Println("Exchanging code for token...")
	creds, err := tidal.ExchangeCode(ctx, clientID, code, pkce.CodeVerifier, port)
	fatal(err)
	fatal(tidal.SaveCredentials(creds))
	fmt.Printf("\nAuthenticated!\n")
	fmt.Printf("  User ID: %s\n", creds.UserID)
	fmt.Printf("  Country: %s\n", creds.CountryCode)
	fmt.Printf("  Credentials saved to %s\n", tidal.CredentialsPath())
}

func cmdStatus() {
	creds := tidal.LoadCredentials()
	if creds == nil {
		fmt.Println("Not authenticated. Run: tidal login")
		os.Exit(1)
	}
	fmt.Printf("User ID: %s\n", creds.UserID)
	fmt.Printf("Country: %s\n", creds.CountryCode)
	fmt.Printf("Client ID: %s\n", creds.ClientID)
	if creds.IsExpired() {
		fmt.Println("Token: EXPIRED (will auto-refresh)")
	} else {
		fmt.Println("Token: valid")
	}
	fmt.Printf("Credentials: %s\n", tidal.CredentialsPath())
}

func cmdToken() {
	creds := tidal.LoadCredentials()
	if creds == nil {
		fmt.Fprintln(os.Stderr, "Not authenticated. Run: tidal login")
		os.Exit(1)
	}
	fmt.Println(creds.AccessToken)
}

// --- Favorites commands ---

func cmdFavorites() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: tidal favorites <list|artists|tracks|add|remove|add-track|remove-track|add-artist|remove-artist> [id]")
		os.Exit(1)
	}

	client := mustClient()
	ctx := context.Background()

	switch os.Args[2] {
	case "list":
		albums, _, err := client.Favorites.GetAlbums(ctx, 50)
		fatal(err)
		fmt.Printf("Favorite albums (%d):\n\n", len(albums))
		for _, a := range albums {
			hi := ""
			if a.IsHiRes() {
				hi = " [Hi-Res]"
			}
			fmt.Printf("  %s%s\n    ID: %s | %d tracks | %s\n\n",
				a.Title, hi, a.ID, a.NumberOfItems, a.Duration)
		}

	case "artists":
		artists, _, err := client.Favorites.GetArtists(ctx, 100)
		fatal(err)
		fmt.Printf("Favorite artists (%d):\n\n", len(artists))
		for _, a := range artists {
			fmt.Printf("  %s (ID: %s)\n", a.Name, a.ID)
		}

	case "tracks":
		tracks, _, err := client.Favorites.GetTracks(ctx, 100)
		fatal(err)
		fmt.Printf("Favorite tracks (%d):\n\n", len(tracks))
		for _, t := range tracks {
			fmt.Printf("  %s (%s)\n    ID: %s\n", t.Title, t.Duration, t.ID)
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
		fmt.Fprintln(os.Stderr, "Usage: tidal playlists <list|create|show|delete|add-tracks|rename>")
		os.Exit(1)
	}

	client := mustClient()
	ctx := context.Background()

	switch os.Args[2] {
	case "list":
		playlists, _, err := client.Playlists.List(ctx, 500)
		fatal(err)
		fmt.Printf("Your playlists (%d):\n\n", len(playlists))
		for _, pl := range playlists {
			fmt.Printf("  %s (%s)\n    ID: %s | %d items\n\n",
				pl.Name, pl.AccessType, pl.ID, pl.NumberOfItems)
		}

	case "create":
		requireArg(3, "playlist name")
		name := os.Args[3]
		public := hasFlag("--public")
		pl, err := client.Playlists.Create(ctx, name, "", public)
		fatal(err)
		fmt.Printf("Created playlist: %s (ID: %s)\n", pl.Name, pl.ID)

	case "show":
		requireArg(3, "playlist ID")
		pl, err := client.Playlists.Get(ctx, os.Args[3])
		fatal(err)
		fmt.Printf("%s (%d items, %s)\n", pl.Name, pl.NumberOfItems, pl.AccessType)
		if pl.Description != "" {
			fmt.Printf("  %s\n", pl.Description)
		}

		// Fetch tracks
		tracks, _, err := client.Playlists.GetItems(ctx, os.Args[3])
		if err == nil && len(tracks) > 0 {
			fmt.Println()
			for i, t := range tracks {
				fmt.Printf("  %2d. %s (%s)\n", i+1, t.Title, t.Duration)
			}
		}

	case "delete":
		requireArg(3, "playlist ID")
		fatal(client.Playlists.Delete(ctx, os.Args[3]))
		fmt.Printf("Deleted playlist %s\n", os.Args[3])

	case "add-tracks":
		requireArg(4, "playlist ID and track IDs")
		playlistID := os.Args[3]
		trackIDs := os.Args[4:]
		fatal(client.Playlists.AddTracks(ctx, playlistID, trackIDs))
		fmt.Printf("Added %d tracks to playlist %s\n", len(trackIDs), playlistID)

	case "rename":
		requireArg(4, "playlist ID and new name")
		playlistID := os.Args[3]
		name := strings.Join(os.Args[4:], " ")
		pl, err := client.Playlists.Update(ctx, playlistID, &tidal.PlaylistUpdateOptions{Name: &name})
		fatal(err)
		fmt.Printf("Renamed playlist %s to: %s\n", pl.ID, pl.Name)

	default:
		fmt.Fprintf(os.Stderr, "Unknown playlists command: %s\n", os.Args[2])
		os.Exit(1)
	}
}

// --- Search commands ---

func cmdSearch() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: tidal search <albums|tracks|artists> <query>")
		os.Exit(1)
	}

	client := mustClient()
	ctx := context.Background()
	query := strings.Join(os.Args[3:], " ")

	switch os.Args[2] {
	case "albums":
		albums, _, err := client.Search.Albums(ctx, query, 20)
		fatal(err)
		fmt.Printf("Albums matching \"%s\":\n\n", query)
		for _, a := range albums {
			hi := ""
			if a.IsHiRes() {
				hi = " [Hi-Res]"
			}
			fmt.Printf("  %s%s\n    ID: %s | %d tracks\n\n", a.Title, hi, a.ID, a.NumberOfItems)
		}

	case "tracks":
		tracks, _, err := client.Search.Tracks(ctx, query, 20)
		fatal(err)
		fmt.Printf("Tracks matching \"%s\":\n\n", query)
		for _, t := range tracks {
			fmt.Printf("  %s (%s)\n    ID: %s | ISRC: %s\n\n", t.Title, t.Duration, t.ID, t.ISRC)
		}

	case "artists":
		artists, _, err := client.Search.Artists(ctx, query, 20)
		fatal(err)
		fmt.Printf("Artists matching \"%s\":\n\n", query)
		for _, a := range artists {
			fmt.Printf("  %s (ID: %s)\n", a.Name, a.ID)
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
	if album.IsHiRes() {
		hi = " [Hi-Res]"
	}
	fmt.Printf("%s%s\n", album.Title, hi)
	fmt.Printf("  Type: %s\n", album.AlbumType)
	if album.ReleaseDate != "" {
		fmt.Printf("  Released: %s\n", album.ReleaseDate)
	}
	fmt.Printf("  %d tracks, %s\n", album.NumberOfItems, album.Duration)
	fmt.Printf("  ID: %s | Barcode: %s\n", album.ID, album.BarcodeID)
	if len(album.MediaTags) > 0 {
		fmt.Printf("  Quality: %s\n", strings.Join(album.MediaTags, ", "))
	}

	// Fetch tracks
	tracks, _, err := client.Catalog.GetAlbumItems(ctx, os.Args[2])
	if err == nil && len(tracks) > 0 {
		fmt.Println("\nTracks:")
		for i, t := range tracks {
			expl := ""
			if t.Explicit {
				expl = " [E]"
			}
			fmt.Printf("  %2d. %s%s (%s)\n", i+1, t.Title, expl, t.Duration)
		}
	}
}

func cmdArtist() {
	requireArg(2, "artist ID")
	client := mustClient()
	ctx := context.Background()

	artist, err := client.Catalog.GetArtist(ctx, os.Args[2])
	fatal(err)

	fmt.Printf("%s (ID: %s)\n", artist.Name, artist.ID)

	// Get albums
	albums, _, err := client.Catalog.GetArtistAlbums(ctx, os.Args[2])
	if err == nil && len(albums) > 0 {
		fmt.Printf("\nAlbums (%d):\n\n", len(albums))
		for _, a := range albums {
			hi := ""
			if a.IsHiRes() {
				hi = " [Hi-Res]"
			}
			fmt.Printf("  %s (%s)%s — %d tracks\n", a.Title, a.ReleaseDate, hi, a.NumberOfItems)
		}
	}
}

func cmdGenres() {
	client := mustClient()
	ctx := context.Background()

	genres, err := client.Catalog.GetGenres(ctx)
	fatal(err)
	fmt.Println("Genres:")
	fmt.Println()
	for _, g := range genres {
		fmt.Printf("  [%s] %s\n", g.ID, g.Name)
	}
}

// --- Helpers ---

func mustClient() *tidal.Client {
	client, err := tidal.NewClientFromCredentials(tidal.WithRateLimit(1.0, 10))
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

func flagStr(flag string) string {
	for i, arg := range os.Args {
		if arg == flag && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

// prettyJSON is used for debug output.
func prettyJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
