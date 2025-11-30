// Google Photos Picker API - Album Selection and Download
// Run: go run picker_server.go
// Then open http://localhost:8085 in your browser

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
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	clientID     = "63287604763-jlprfv7l05du7s0pnru25dj767jn68rq.apps.googleusercontent.com"
	clientSecret = "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"
	tokenFile    = "picker_token.json"
	downloadDir  = "./timeliner_repo/data"
)

var oauthConfig = &oauth2.Config{
	ClientID:     clientID,
	ClientSecret: clientSecret,
	Scopes: []string{
		"https://www.googleapis.com/auth/photospicker.mediaitems.readonly",
	},
	Endpoint:    google.Endpoint,
	RedirectURL: "http://localhost:8085/callback",
}

var (
	currentToken *oauth2.Token
	tokenMutex   sync.RWMutex
)

func main() {
	// Try to load existing token
	token, err := loadToken(tokenFile)
	if err == nil {
		tokenMutex.Lock()
		currentToken = token
		tokenMutex.Unlock()
	}

	mux := http.NewServeMux()

	// Serve the main page
	mux.HandleFunc("/", handleHome)
	mux.HandleFunc("/auth", handleAuth)
	mux.HandleFunc("/callback", handleCallback)
	mux.HandleFunc("/picker", handlePicker)
	mux.HandleFunc("/sessions", handleSessions)
	mux.HandleFunc("/session/", handleSessionMedia)
	mux.HandleFunc("/download", handleDownload)
	mux.HandleFunc("/proxy", handleProxy)

	fmt.Println("🚀 Server starting at http://localhost:8085")
	fmt.Println("Open this URL in your browser to use the Google Photos Picker")
	log.Fatal(http.ListenAndServe(":8085", mux))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tokenMutex.RLock()
	hasToken := currentToken != nil && currentToken.Valid()
	tokenMutex.RUnlock()

	html := `<!DOCTYPE html>
<html>
<head>
    <title>Google Photos Picker</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #1a73e8; }
        .btn { display: inline-block; padding: 12px 24px; background: #1a73e8; color: white; text-decoration: none; border-radius: 4px; margin: 10px 5px 10px 0; border: none; cursor: pointer; font-size: 16px; }
        .btn:hover { background: #1557b0; }
        .btn-secondary { background: #5f6368; }
        .btn-secondary:hover { background: #3c4043; }
        .status { padding: 15px; border-radius: 4px; margin: 20px 0; }
        .status.success { background: #e6f4ea; color: #137333; }
        .status.warning { background: #fef7e0; color: #b06000; }
        .status.error { background: #fce8e6; color: #c5221f; }
        #results { margin-top: 20px; }
        .media-item { display: inline-block; margin: 5px; }
        .media-item img { width: 150px; height: 150px; object-fit: cover; border-radius: 4px; }
        .album { padding: 10px; margin: 5px 0; background: #f1f3f4; border-radius: 4px; cursor: pointer; }
        .album:hover { background: #e8eaed; }
        #loading { display: none; padding: 20px; text-align: center; }
    </style>
</head>
<body>
    <h1>📷 Google Photos Picker</h1>
    <p>Use the Picker API to select photos from your Google Photos library.</p>
    `

	if hasToken {
		html += `
    <div class="status success">✅ Authenticated! You can now use the Picker.</div>
    <button class="btn" onclick="createSession()">Open Photo Picker</button>
    <button class="btn btn-secondary" onclick="window.location='/auth'">Re-authenticate</button>
    `
	} else {
		html += `
    <div class="status warning">⚠️ Not authenticated. Please sign in first.</div>
    <a href="/auth" class="btn">Sign in with Google</a>
    `
	}

	html += `
    <div id="loading">Loading...</div>
    <div id="results"></div>

    <script>
    async function createSession() {
        document.getElementById('loading').style.display = 'block';
        document.getElementById('results').innerHTML = '';
        
        try {
            const resp = await fetch('/picker', { method: 'POST' });
            const data = await resp.json();
            
            if (data.error) {
                document.getElementById('results').innerHTML = '<div class="status error">Error: ' + data.error + '</div>';
                return;
            }
            
            // Open the picker URL
            const pickerWindow = window.open(data.pickerUri, 'picker', 'width=800,height=600');
            
            document.getElementById('results').innerHTML = 
                '<div class="status success">Picker opened! Select your photos in the popup window, then click Done.</div>' +
                '<p>Session ID: ' + data.id + '</p>' +
                '<button class="btn" onclick="checkSession(\'' + data.id + '\')">Check Selected Photos</button>' +
                '<p style="color:#666; margin-top:10px;">After clicking Done in the picker, click "Check Selected Photos" above.</p>';
            
            // Start polling for session updates
            const pollInterval = setInterval(async () => {
                try {
                    const pollResp = await fetch('/session/' + data.id);
                    const pollData = await pollResp.json();
                    if (pollData.mediaItems && pollData.mediaItems.length > 0) {
                        clearInterval(pollInterval);
                        checkSession(data.id);
                    }
                } catch (e) {
                    // ignore polling errors
                }
            }, 2000);
                
        } catch (err) {
            document.getElementById('results').innerHTML = '<div class="status error">Error: ' + err.message + '</div>';
        } finally {
            document.getElementById('loading').style.display = 'none';
        }
    }
    
    async function checkSession(sessionId) {
        document.getElementById('loading').style.display = 'block';
        
        try {
            const resp = await fetch('/session/' + sessionId);
            const data = await resp.json();
            
            if (data.error) {
                document.getElementById('results').innerHTML = '<div class="status error">Error: ' + data.error + '</div>';
                return;
            }
            
            let html = '<h2>Selected Photos (' + (data.mediaItems?.length || 0) + ')</h2>';
            
            if (data.mediaItems && data.mediaItems.length > 0) {
                html += '<button class="btn" onclick="downloadAll(\'' + sessionId + '\')">Download All Photos</button>';
                html += '<div style="margin-top: 20px;">';
                for (const item of data.mediaItems) {
                    const thumbUrl = '/proxy?url=' + encodeURIComponent(item.mediaFile?.baseUrl + '=w150-h150-c');
                    html += '<div class="media-item"><img src="' + thumbUrl + '" title="' + (item.mediaFile?.filename || 'photo') + '"></div>';
                }
                html += '</div>';
            } else {
                html += '<p>No photos selected yet. Select photos in the picker window and click "Check Selected Photos" again.</p>';
            }
            
            document.getElementById('results').innerHTML = html;
            
        } catch (err) {
            document.getElementById('results').innerHTML = '<div class="status error">Error: ' + err.message + '</div>';
        } finally {
            document.getElementById('loading').style.display = 'none';
        }
    }
    
    async function downloadAll(sessionId) {
        document.getElementById('loading').style.display = 'block';
        document.getElementById('loading').innerHTML = 'Downloading photos...';
        
        try {
            const resp = await fetch('/download?session=' + sessionId, { method: 'POST' });
            const data = await resp.json();
            
            if (data.error) {
                document.getElementById('results').innerHTML += '<div class="status error">Error: ' + data.error + '</div>';
                return;
            }
            
            document.getElementById('results').innerHTML += 
                '<div class="status success">✅ Downloaded ' + data.downloaded + ' photos to ' + data.directory + '</div>';
                
        } catch (err) {
            document.getElementById('results').innerHTML += '<div class="status error">Error: ' + err.message + '</div>';
        } finally {
            document.getElementById('loading').style.display = 'none';
            document.getElementById('loading').innerHTML = 'Loading...';
        }
    }
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func handleAuth(w http.ResponseWriter, r *http.Request) {
	authURL := oauthConfig.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No code in callback", http.StatusBadRequest)
		return
	}

	token, err := oauthConfig.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, fmt.Sprintf("Token exchange failed: %v", err), http.StatusInternalServerError)
		return
	}

	tokenMutex.Lock()
	currentToken = token
	tokenMutex.Unlock()

	saveToken(tokenFile, token)

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func handlePicker(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenMutex.RLock()
	token := currentToken
	tokenMutex.RUnlock()

	if token == nil {
		jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Refresh token if needed
	if !token.Valid() {
		tokenSource := oauthConfig.TokenSource(context.Background(), token)
		newToken, err := tokenSource.Token()
		if err != nil {
			jsonError(w, fmt.Sprintf("Token refresh failed: %v", err), http.StatusUnauthorized)
			return
		}
		tokenMutex.Lock()
		currentToken = newToken
		token = newToken
		tokenMutex.Unlock()
		saveToken(tokenFile, newToken)
	}

	// Create a picker session
	client := oauthConfig.Client(context.Background(), token)

	// Create empty session - picker will show user's photos
	reqBody := `{}`
	resp, err := client.Post(
		"https://photospicker.googleapis.com/v1/sessions",
		"application/json",
		strings.NewReader(reqBody),
	)
	if err != nil {
		jsonError(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		jsonError(w, fmt.Sprintf("API error %d: %s", resp.StatusCode, string(body)), resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func handleSessions(w http.ResponseWriter, r *http.Request) {
	tokenMutex.RLock()
	token := currentToken
	tokenMutex.RUnlock()

	if token == nil {
		jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	client := oauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://photospicker.googleapis.com/v1/sessions")
	if err != nil {
		jsonError(w, fmt.Sprintf("Failed to list sessions: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func handleSessionMedia(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/session/")
	if sessionID == "" {
		jsonError(w, "Session ID required", http.StatusBadRequest)
		return
	}

	tokenMutex.RLock()
	token := currentToken
	tokenMutex.RUnlock()

	if token == nil {
		jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	client := oauthConfig.Client(context.Background(), token)
	
	// First get the session status
	resp, err := client.Get(fmt.Sprintf("https://photospicker.googleapis.com/v1/sessions/%s", sessionID))
	if err != nil {
		jsonError(w, fmt.Sprintf("Failed to get session: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Session %s response: %s", sessionID, string(body))
	
	// Parse to check if we need to get media items separately
	var session map[string]interface{}
	json.Unmarshal(body, &session)
	
	// If session has mediaItemsSet but no mediaItems, we need to fetch them
	if _, hasMediaItemsSet := session["mediaItemsSet"]; hasMediaItemsSet {
		if _, hasMediaItems := session["mediaItems"]; !hasMediaItems {
			// Fetch media items for this session
			mediaResp, err := client.Get(fmt.Sprintf("https://photospicker.googleapis.com/v1/mediaItems?sessionId=%s&pageSize=100", sessionID))
			if err != nil {
				log.Printf("Failed to get media items: %v", err)
			} else {
				defer mediaResp.Body.Close()
				mediaBody, _ := io.ReadAll(mediaResp.Body)
				log.Printf("Media items response: %s", string(mediaBody))
				
				var mediaResult map[string]interface{}
				if err := json.Unmarshal(mediaBody, &mediaResult); err == nil {
					if items, ok := mediaResult["mediaItems"]; ok {
						session["mediaItems"] = items
					}
				}
			}
		}
	}
	
	// Return the combined result
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		jsonError(w, "Session ID required", http.StatusBadRequest)
		return
	}

	tokenMutex.RLock()
	token := currentToken
	tokenMutex.RUnlock()

	if token == nil {
		jsonError(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	client := oauthConfig.Client(context.Background(), token)

	// First check if session has media items set
	sessionResp, err := client.Get(fmt.Sprintf("https://photospicker.googleapis.com/v1/sessions/%s", sessionID))
	if err != nil {
		jsonError(w, fmt.Sprintf("Failed to get session: %v", err), http.StatusInternalServerError)
		return
	}
	sessionBody, _ := io.ReadAll(sessionResp.Body)
	sessionResp.Body.Close()
	log.Printf("Download - Session response: %s", string(sessionBody))

	// Fetch media items for this session
	mediaResp, err := client.Get(fmt.Sprintf("https://photospicker.googleapis.com/v1/mediaItems?sessionId=%s&pageSize=100", sessionID))
	if err != nil {
		jsonError(w, fmt.Sprintf("Failed to get media items: %v", err), http.StatusInternalServerError)
		return
	}
	defer mediaResp.Body.Close()

	mediaBody, _ := io.ReadAll(mediaResp.Body)
	log.Printf("Download - Media items response: %s", string(mediaBody))

	var session struct {
		MediaItems []struct {
			ID        string `json:"id"`
			MediaFile struct {
				BaseURL   string `json:"baseUrl"`
				Filename  string `json:"filename"`
				MimeType  string `json:"mimeType"`
				MediaType string `json:"mediaType"`
			} `json:"mediaFile"`
			CreateTime string `json:"createTime"`
		} `json:"mediaItems"`
	}

	if err := json.Unmarshal(mediaBody, &session); err != nil {
		jsonError(w, fmt.Sprintf("Failed to parse media items: %v", err), http.StatusInternalServerError)
		return
	}
	
	log.Printf("Download - Found %d media items", len(session.MediaItems))

	downloaded := 0
	for _, item := range session.MediaItems {
		// Parse create time to organize by date
		createTime, err := time.Parse(time.RFC3339, item.CreateTime)
		if err != nil {
			createTime = time.Now()
		}

		// Create directory structure: data/YYYY/MM/google_photos/
		dir := filepath.Join(downloadDir, createTime.Format("2006"), createTime.Format("01"), "google_photos")
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Failed to create directory %s: %v", dir, err)
			continue
		}

		// Download the file
		// For photos, append =d to get the original
		// For videos, append =dv to get the video
		downloadURL := item.MediaFile.BaseURL
		if strings.HasPrefix(item.MediaFile.MimeType, "video/") {
			downloadURL += "=dv"
		} else {
			downloadURL += "=d"
		}

		filename := item.MediaFile.Filename
		if filename == "" {
			filename = item.ID + ".jpg"
		}

		filePath := filepath.Join(dir, filename)

		// Skip if already exists
		if _, err := os.Stat(filePath); err == nil {
			log.Printf("Skipping existing file: %s", filePath)
			downloaded++
			continue
		}

		log.Printf("Downloading: %s from %s", filename, downloadURL)

		// Use authenticated client to download
		imgResp, err := client.Get(downloadURL)
		if err != nil {
			log.Printf("Failed to download %s: %v", filename, err)
			continue
		}

		if imgResp.StatusCode != 200 {
			imgResp.Body.Close()
			log.Printf("Failed to download %s: HTTP %d", filename, imgResp.StatusCode)
			continue
		}

		f, err := os.Create(filePath)
		if err != nil {
			imgResp.Body.Close()
			log.Printf("Failed to create file %s: %v", filePath, err)
			continue
		}

		_, err = io.Copy(f, imgResp.Body)
		f.Close()
		imgResp.Body.Close()

		if err != nil {
			log.Printf("Failed to save %s: %v", filename, err)
			os.Remove(filePath)
			continue
		}

		downloaded++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"downloaded": downloaded,
		"total":      len(session.MediaItems),
		"directory":  downloadDir,
	})
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	imageURL := r.URL.Query().Get("url")
	if imageURL == "" {
		http.Error(w, "URL required", http.StatusBadRequest)
		return
	}

	tokenMutex.RLock()
	token := currentToken
	tokenMutex.RUnlock()

	if token == nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	client := oauthConfig.Client(context.Background(), token)
	resp, err := client.Get(imageURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch image: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy headers
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "max-age=3600")
	io.Copy(w, resp.Body)
}

func jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
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
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}

