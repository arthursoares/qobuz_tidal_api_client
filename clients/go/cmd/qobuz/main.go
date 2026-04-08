package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"

	qobuz "github.com/arthursoares/qobuz_api_client/clients/go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: qobuz <login|status|token>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "login":
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
		login(port, noBrowser)
	case "status":
		creds := qobuz.LoadCredentials()
		if creds == nil {
			fmt.Println("Not authenticated. Run: qobuz login")
			os.Exit(1)
		}
		fmt.Printf("Logged in as: %s\n", creds.DisplayName)
		fmt.Printf("User ID: %s\n", creds.UserID)
		fmt.Printf("Credentials: %s\n", qobuz.CredentialsPath())
	case "token":
		creds := qobuz.LoadCredentials()
		if creds == nil {
			fmt.Fprintln(os.Stderr, "Not authenticated. Run: qobuz login")
			os.Exit(1)
		}
		fmt.Println(creds.UserAuthToken)
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func login(port int, noBrowser bool) {
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
		redirectURL := scanner.Text()
		code, err = qobuz.ExtractCodeFromURL(redirectURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("Opening browser for Qobuz login...")
		if browserErr := qobuz.OpenBrowser(url); browserErr != nil {
			fmt.Fprintf(os.Stderr, "Could not open browser: %v\n", browserErr)
			fmt.Printf("Open this URL manually:\n  %s\n\n", url)
		}
		fmt.Printf("Waiting for callback on localhost:%d...\n", port)
		code, err = qobuz.WaitForCallback(port)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Exchanging code for token...")
	creds, err := qobuz.ExchangeCode(ctx, code)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := qobuz.SaveCredentials(creds); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving credentials: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nAuthenticated as %s\n", creds.DisplayName)
	fmt.Printf("  Token saved to %s\n", qobuz.CredentialsPath())
}
