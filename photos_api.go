// Direct Google Photos API access
// Run: go run photos_api.go
//
// NOTE: As of March 2025, Google restricted the photoslibrary.readonly scope.
// This script may only work for photos uploaded by this specific app,
// not your entire library.
//
// SETUP: Add http://localhost:8085/callback to your OAuth client's
// Authorized redirect URIs in Google Cloud Console.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Change these to match your Google Cloud project
const (
	clientID     = "63287604763-jlprfv7l05du7s0pnru25dj767jn68rq.apps.googleusercontent.com"
	clientSecret = "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"
	tokenFile    = "photos_token.json"
)

var oauthConfig = &oauth2.Config{
	ClientID:     clientID,
	ClientSecret: clientSecret,
	Scopes: []string{
		"https://www.googleapis.com/auth/photoslibrary.readonly",
	},
	Endpoint:    google.Endpoint,
	RedirectURL: "http://localhost:8085/callback",
}

func main() {
	ctx := context.Background()

	// Try to load existing token
	token, err := loadToken(tokenFile)
	if err != nil {
		// No token found, need to authenticate
		token, err = authenticate(ctx)
		if err != nil {
			log.Fatalf("Authentication failed: %v", err)
		}
		saveToken(tokenFile, token)
	}

	// Check if token needs refresh
	if !token.Valid() {
		tokenSource := oauthConfig.TokenSource(ctx, token)
		newToken, err := tokenSource.Token()
		if err != nil {
			log.Printf("Token refresh failed, re-authenticating: %v", err)
			os.Remove(tokenFile)
			token, err = authenticate(ctx)
			if err != nil {
				log.Fatalf("Authentication failed: %v", err)
			}
			saveToken(tokenFile, token)
		} else {
			token = newToken
			saveToken(tokenFile, token)
		}
	}

	// Create HTTP client with token
	client := oauthConfig.Client(ctx, token)

	// Test: List albums
	fmt.Println("\n=== Listing Albums ===")
	if err := listAlbums(client); err != nil {
		log.Printf("Error listing albums: %v", err)
	}

	// Test: List media items
	fmt.Println("\n=== Listing Media Items ===")
	if err := listMediaItems(client); err != nil {
		log.Printf("Error listing media items: %v", err)
	}
}

func authenticate(ctx context.Context) (*oauth2.Token, error) {
	// Start local server for callback
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()
	server := &http.Server{Addr: ":8085", Handler: mux}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in callback: %v", r.URL.Query().Get("error"))
			fmt.Fprintf(w, "<h1>Authentication failed</h1><p>%s</p>", r.URL.Query().Get("error"))
			return
		}
		fmt.Fprintf(w, "<h1>Authentication successful!</h1><p>You can close this window.</p>")
		codeChan <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Generate auth URL with offline access
	authURL := oauthConfig.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("\n🔐 Open this URL in your browser to authenticate:\n\n%s\n\n", authURL)
	fmt.Println("Waiting for authentication...")

	// Wait for callback
	var code string
	select {
	case code = <-codeChan:
	case err := <-errChan:
		server.Shutdown(ctx)
		return nil, err
	case <-time.After(5 * time.Minute):
		server.Shutdown(ctx)
		return nil, fmt.Errorf("authentication timeout")
	}

	server.Shutdown(ctx)

	// Exchange code for token
	token, err := oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	return token, nil
}

func listAlbums(client *http.Client) error {
	resp, err := client.Get("https://photoslibrary.googleapis.com/v1/albums?pageSize=10")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	albums, ok := result["albums"].([]interface{})
	if !ok || len(albums) == 0 {
		fmt.Println("No albums found (or no access)")
		return nil
	}

	for _, a := range albums {
		album := a.(map[string]interface{})
		fmt.Printf("- %s (ID: %s)\n", album["title"], album["id"])
	}

	return nil
}

func listMediaItems(client *http.Client) error {
	resp, err := client.Get("https://photoslibrary.googleapis.com/v1/mediaItems?pageSize=10")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	items, ok := result["mediaItems"].([]interface{})
	if !ok || len(items) == 0 {
		fmt.Println("No media items found (or no access)")
		return nil
	}

	for _, m := range items {
		item := m.(map[string]interface{})
		fmt.Printf("- %s (%s)\n", item["filename"], item["mimeType"])
	}

	return nil
}

func loadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var token oauth2.Token
	err = json.NewDecoder(f).Decode(&token)
	return &token, err
}

func saveToken(path string, token *oauth2.Token) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		os.MkdirAll(dir, 0755)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}

