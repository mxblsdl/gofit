package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cli/browser"
)

// Config holds the application configuration
type Config struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri"`
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	UserID       string `json:"user_id"`
}

// TokenInfo stores token data with expiry time
type TokenInfo struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	UserID       string    `json:"user_id"`
}

// FitbitDownloader manages downloading Fitbit data
type FitbitDownloader struct {
	Config    Config
	TokenInfo TokenInfo
	DataDir   string
}

func (fd *FitbitDownloader) ClearAllData() error {
	// Clear all data files in the data directory
	files, err := os.ReadDir(fd.DataDir)
	if err != nil {
		return fmt.Errorf("failed to read data directory: %w", err)
	}

	for _, file := range files {
		if file.Name() == "token_info.json" {
			continue
		}
		err := os.Remove(fd.DataDir + "/" + file.Name())
		if err != nil {
			return fmt.Errorf("failed to remove file %s: %w", file.Name(), err)
		}
	}

	return nil
}

// StartAuthFlow initiates the OAuth authorization flow
func (fd *FitbitDownloader) StartAuthFlow() error {
	// Create a channel to receive the authorization code
	authCodeChan := make(chan string)
	serverErrChan := make(chan error)

	// Start the local server to handle the callback
	go fd.startCallbackServer(authCodeChan, serverErrChan)

	// Generate authorization URL
	authURL := "https://www.fitbit.com/oauth2/authorize"
	params := url.Values{}
	params.Add("client_id", fd.Config.ClientID)
	params.Add("response_type", "code")
	params.Add("scope", "activity heartrate location nutrition profile settings sleep social weight")
	params.Add("redirect_uri", fd.Config.RedirectURI)

	fullAuthURL := fmt.Sprintf("%s?%s", authURL, params.Encode())
	fmt.Println("Opening browser for authorization:", fullAuthURL)

	// Open the authorization URL in the browser
	err := browser.OpenURL(fullAuthURL)
	if err != nil {
		return fmt.Errorf("failed to open browser: %v", err)
	}

	// Wait for the authorization code or an error
	select {
	case authCode := <-authCodeChan:
		return fd.getAccessToken(authCode)
	case err := <-serverErrChan:
		return err
	}
}

// startCallbackServer starts a local server to receive the OAuth callback
func (fd *FitbitDownloader) startCallbackServer(authCodeChan chan<- string, errChan chan<- error) {
	server := &http.Server{Addr: ":8080"}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		queryParams := r.URL.Query()
		code := queryParams.Get("code")

		if code != "" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("Authorization successful! You can close this window and return to the application."))
			authCodeChan <- code
		} else {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Authorization failed. Please try again."))
			errChan <- fmt.Errorf("authorization failed, no code received")
		}

		// Shutdown the server after handling the request
		go func() {
			time.Sleep(100 * time.Millisecond)
			server.Close()
		}()
	})

	fmt.Println("Starting local server to receive callback...")
	fmt.Println("Waiting for authorization callback...")

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		errChan <- fmt.Errorf("HTTP server error: %v", err)
	}
}

// getAccessToken exchanges the authorization code for an access token
func (fd *FitbitDownloader) getAccessToken(authCode string) error {
	fmt.Println("Exchanging authorization code for access token...")

	tokenURL := "https://api.fitbit.com/oauth2/token"
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", authCode)
	data.Set("redirect_uri", fd.Config.RedirectURI)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create token request: %v", err)
	}

	// Create the authorization header value
	authValue := base64.StdEncoding.EncodeToString([]byte(fd.Config.ClientID + ":" + fd.Config.ClientSecret))
	req.Header.Set("Authorization", "Basic "+authValue)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("token request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to obtain access token: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&tokenResp)
	if err != nil {
		return fmt.Errorf("failed to decode token response: %v", err)
	}

	fd.TokenInfo = TokenInfo{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		UserID:       tokenResp.UserID,
	}

	// Save token information to a file
	err = fd.saveTokenInfo()
	if err != nil {
		return fmt.Errorf("failed to save token information: %v", err)
	}

	fmt.Println("Successfully obtained access token!")
	return nil
}

// saveTokenInfo saves the token information to a file
func (fd *FitbitDownloader) saveTokenInfo() error {
	tokenFile := filepath.Join(fd.DataDir, "token_info.json")
	tokenData, err := json.MarshalIndent(fd.TokenInfo, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tokenFile, tokenData, 0600)
}

// loadTokenInfo loads token information from a file
func (fd *FitbitDownloader) LoadTokenInfo() error {
	tokenFile := filepath.Join(fd.DataDir, "token_info.json")
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		return fmt.Errorf("token file does not exist")
	}

	tokenData, err := os.ReadFile(tokenFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(tokenData, &fd.TokenInfo)
}

// refreshAccessToken refreshes the access token if expired
func (fd *FitbitDownloader) refreshAccessToken() error {
	// Try to load token info if not available
	if fd.TokenInfo.AccessToken == "" {
		err := fd.LoadTokenInfo()
		if err != nil {
			return fmt.Errorf("no access token available: %v", err)
		}
	}

	// Check if token is expired
	if time.Now().After(fd.TokenInfo.ExpiresAt) {
		fmt.Println("Access token expired. Refreshing...")

		tokenURL := "https://api.fitbit.com/oauth2/token"
		data := url.Values{}
		data.Set("grant_type", "refresh_token")
		data.Set("refresh_token", fd.TokenInfo.RefreshToken)

		req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
		if err != nil {
			return fmt.Errorf("failed to create refresh token request: %v", err)
		}

		// Create the authorization header value
		authValue := base64.StdEncoding.EncodeToString([]byte(fd.Config.ClientID + ":" + fd.Config.ClientSecret))
		req.Header.Set("Authorization", "Basic "+authValue)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("refresh token request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to refresh access token: %d %s", resp.StatusCode, string(bodyBytes))
		}

		var tokenResp TokenResponse
		err = json.NewDecoder(resp.Body).Decode(&tokenResp)
		if err != nil {
			return fmt.Errorf("failed to decode refresh token response: %v", err)
		}

		fd.TokenInfo.AccessToken = tokenResp.AccessToken
		fd.TokenInfo.RefreshToken = tokenResp.RefreshToken
		fd.TokenInfo.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

		// Save updated token information
		err = fd.saveTokenInfo()
		if err != nil {
			return fmt.Errorf("failed to save updated token information: %v", err)
		}

		fmt.Println("Successfully refreshed access token!")
	}

	return nil
}

// DownloadProfile downloads user profile data
func (fd *FitbitDownloader) DownloadProfile() error {
	err := fd.refreshAccessToken()
	if err != nil {
		return err
	}

	fmt.Println("Downloading user profile data...")

	url := "https://api.fitbit.com/1/user/-/profile.json"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create profile request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+fd.TokenInfo.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("profile request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to download profile data: %d %s", resp.StatusCode, string(bodyBytes))
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read profile response: %v", err)
	}

	// Indent the JSON for better readability
	var profileData interface{}
	err = json.Unmarshal(bodyBytes, &profileData)
	if err != nil {
		return fmt.Errorf("failed to parse profile JSON: %v", err)
	}

	formattedJSON, err := json.MarshalIndent(profileData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format profile JSON: %v", err)
	}

	// Save to file
	filename := filepath.Join(fd.DataDir, "profile.json")
	err = os.WriteFile(filename, formattedJSON, 0644)
	if err != nil {
		return fmt.Errorf("failed to save profile data: %v", err)
	}

	fmt.Println("Profile data downloaded successfully!")
	return nil
}

// DownloadActivities downloads activity data for a date range
func (fd *FitbitDownloader) DownloadActivities(activity, startDate, endDate string) error {
	err := fd.refreshAccessToken()
	if err != nil {
		return err
	}

	// If no dates are provided, use the last 30 days
	if startDate == "" || endDate == "" {
		endDate = time.Now().Format("2006-01-02")
		startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}

	fmt.Printf("Downloading %s data from %s to %s...\n", activity, startDate, endDate)

	url := fmt.Sprintf("https://api.fitbit.com/1/user/-/activities/%s/date/%s/%s.json", activity, startDate, endDate)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create activities request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+fd.TokenInfo.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("activities request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to download activities data: %d %s", resp.StatusCode, string(bodyBytes))
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read activities response: %v", err)
	}

	// Indent the JSON for better readability
	var activitiesData interface{}
	err = json.Unmarshal(bodyBytes, &activitiesData)
	if err != nil {
		return fmt.Errorf("failed to parse activities JSON: %v", err)
	}

	formattedJSON, err := json.MarshalIndent(activitiesData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format activities JSON: %v", err)
	}

	// Save to file
	filename := filepath.Join(fd.DataDir, fmt.Sprintf("%s_%s_to_%s.json", activity, startDate, endDate))
	err = os.WriteFile(filename, formattedJSON, 0644)
	if err != nil {
		return fmt.Errorf("failed to save activities data: %v", err)
	}

	fmt.Println("Activity data downloaded successfully!")
	return nil
}

// DownloadHeartRate downloads heart rate data for a date range
func (fd *FitbitDownloader) DownloadHeartRate(startDate, endDate string) error {
	err := fd.refreshAccessToken()
	if err != nil {
		return err
	}

	// If no dates are provided, use the last 7 days (heart rate data can be large)
	if startDate == "" || endDate == "" {
		endDate = time.Now().Format("2006-01-02")
		startDate = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	}

	fmt.Printf("Downloading heart rate data from %s to %s...\n", startDate, endDate)

	url := fmt.Sprintf("https://api.fitbit.com/1/user/-/activities/heart/date/%s/%s.json", startDate, endDate)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create heart rate request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+fd.TokenInfo.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("heart rate request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to download heart rate data: %d %s", resp.StatusCode, string(bodyBytes))
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read heart rate response: %v", err)
	}

	// Indent the JSON for better readability
	var heartRateData interface{}
	err = json.Unmarshal(bodyBytes, &heartRateData)
	if err != nil {
		return fmt.Errorf("failed to parse heart rate JSON: %v", err)
	}

	formattedJSON, err := json.MarshalIndent(heartRateData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format heart rate JSON: %v", err)
	}

	// Save to file
	filename := filepath.Join(fd.DataDir, fmt.Sprintf("heart_rate_%s_to_%s.json", startDate, endDate))
	err = os.WriteFile(filename, formattedJSON, 0644)
	if err != nil {
		return fmt.Errorf("failed to save heart rate data: %v", err)
	}

	fmt.Println("Heart rate data downloaded successfully!")
	return nil
}

// DownloadAllData downloads all types of data
func (fd *FitbitDownloader) DownloadAllData(daysBack int) error {
	endDate := time.Now().Format("2006-01-02")
	startDate := time.Now().AddDate(0, 0, -daysBack).Format("2006-01-02")

	// Download each type of data
	err := fd.DownloadProfile()
	if err != nil {
		return fmt.Errorf("failed to download profile: %v", err)
	}

	activites := []string{"steps", "distance", "floors", "calories", "elevation", "minutesSedentary", "minutesLightlyActive", "minutesFairlyActive", "minutesVeryActive"}
	for _, activity := range activites {
		err = fd.DownloadActivities(activity, startDate, endDate)
		if err != nil {
			return fmt.Errorf("failed to download activities: %v", err)
		}
	}

	err = fd.DownloadHeartRate(startDate, endDate)
	if err != nil {
		return fmt.Errorf("failed to download heart rate data: %v", err)
	}

	fmt.Printf("All data downloaded successfully to the '%s' directory!\n", fd.DataDir)
	return nil
}
