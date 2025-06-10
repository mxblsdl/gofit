package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds the application configuration
type Config struct {
	ClientID     string `json:"client_id" validate:"required"`
	ClientSecret string `json:"client_secret" validate:"required"`
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
	Config          Config
	TokenInfo       TokenInfo
	DataDir         string
	callbackRunning bool
}

type ProfileData struct {
	User UserProfile `json:"user"`
}

type UserProfile struct {
	Age               int     `json:"age"`
	AverageDailySteps int     `json:"averageDailySteps"`
	DateOfBirth       string  `json:"dateOfBirth"`
	DisplayName       string  `json:"displayName"`
	FirstName         string  `json:"firstName"`
	FullName          string  `json:"fullName"`
	LastName          string  `json:"lastName"`
	Gender            string  `json:"gender"`
	Height            float64 `json:"height"`
	HeightUnit        string  `json:"heightUnit"`
	TimeZone          string  `json:"timezone"`
	Weight            float64 `json:"weight"`
	WeightUnit        string  `json:"weightUnit"`
}

type ActivityData struct {
	Activities []ActivityEntry `json:"activities-steps"`
}

type ActivityEntry struct {
	DateTime string `json:"dateTime"`
	Value    string `json:"value"`
}

func (s *ActivityData) GetValues() []ActivityEntry {
	entries := make([]ActivityEntry, len(s.Activities))
	for i, activity := range s.Activities {
		entries[i] = ActivityEntry{
			DateTime: activity.DateTime,
			Value:    activity.Value,
		}
	}
	return entries
}

func (s *ActivityData) ProcessData() ChartData {
	// Convert StepsData to ChartData for visualization
	// TODO figure out how I can generalize this method to handle different types of data
	chart := ChartData{
		Title:    "Steps Over Time",
		Subtitle: "Daily step count for the last 30 days",
		XAxis:    make([]string, len(s.Activities)),
		Series:   map[string][]int{"Steps": make([]int, len(s.Activities))},
	}

	for i, entry := range s.Activities {
		val, err := strconv.Atoi(entry.Value)
		if err != nil {
			val = 0 // or handle error as needed
		}

		chart.XAxis[i] = entry.DateTime
		chart.Series["Steps"][i] = val
	}

	return chart
}

// func (fd *FitbitDownloader) ClearAllData() error {
// 	// Clear all data files in the data directory
// 	files, err := os.ReadDir(fd.DataDir)
// 	if err != nil {
// 		return fmt.Errorf("failed to read data directory: %w", err)
// 	}

// 	for _, file := range files {
// 		if file.Name() == "token_info.json" {
// 			continue
// 		}
// 		err := os.Remove(fd.DataDir + "/" + file.Name())
// 		if err != nil {
// 			return fmt.Errorf("failed to remove file %s: %w", file.Name(), err)
// 		}
// 	}

// 	return nil
// }

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

	// Validate the authorization URL by sending a test request
	req, err := http.NewRequest("GET", fullAuthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create authorization request: %v", err)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate authorization URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("invalid client_id or client_secret: %d", resp.StatusCode)
	}

	// Open the authorization URL in the browser
	cmd := exec.Command("chromium", fullAuthURL) // Use xdg-open for Linux, or change to "open" for macOS
	fmt.Println("If no browser opens, please copy and paste the following URL into your browser:")
	fmt.Println(fullAuthURL)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open browser: %v", err)
	}

	// Wait for the authorization code or an error
	select {
	case authCode := <-authCodeChan:
		return fd.getAccessToken(authCode)
	case err := <-serverErrChan:
		return fmt.Errorf("server error: %v", err)
	}
}

// startCallbackServer starts a local server to receive the OAuth callback
func (fd *FitbitDownloader) startCallbackServer(authCodeChan chan<- string, errChan chan<- error) {
	if fd.callbackRunning {
		log.Println("Callback server is already running, skipping start.")
		return
	}
	fd.callbackRunning = true

	log.Println("Starting local server to receive authorization callback...")
	server := &http.Server{Addr: "localhost:8081"}

	mux := http.NewServeMux()
	// ERROR handling the index page for the server
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
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
			log.Printf("Failed to refresh access token: %d %s\n", resp.StatusCode, string(bodyBytes))

			// Remove the old token file
			tokenFile := filepath.Join(fd.DataDir, "token_info.json")
			if err := os.Remove(tokenFile); err != nil {
				return fmt.Errorf("failed to remove token file: %v", err)
			}

			// Restart the authentication process
			fmt.Println("Starting reauthentication process...")
			return fd.StartAuthFlow()
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
func (fd *FitbitDownloader) DownloadProfile() (*ProfileData, error) {
	err := fd.refreshAccessToken()
	if err != nil {
		return nil, err
	}

	fmt.Println("Downloading user profile data...")

	url := "https://api.fitbit.com/1/user/-/profile.json"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+fd.TokenInfo.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("profile request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to download profile data: %d %s", resp.StatusCode, string(bodyBytes))
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile response: %v", err)
	}

	// Indent the JSON for better readability
	var profileData ProfileData
	err = json.Unmarshal(bodyBytes, &profileData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse profile JSON: %v", err)
	}

	fmt.Printf("Profile data for: %s downloaded successfully\n", string(profileData.User.FullName))

	return &profileData, nil
}

func (fd *FitbitDownloader) DownloadActivities(activity string, days_back int) (*ActivityData, error) {

	endDate := time.Now().Format("2006-01-02")
	startDate := time.Now().AddDate(0, 0, -days_back).Format("2006-01-02")

	fmt.Printf("Reading %s data from %s to %s...\n", activity, startDate, endDate)

	endpoint := fmt.Sprintf("https://api.fitbit.com/1/user/-/activities/%s/date/%s/%s.json", activity, startDate, endDate)

	data, err := fd.getData(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to download %s data: %v", activity, err)
	}
	fmt.Printf("%s data downloaded successfully!\n", activity)
	return data, nil
}

func (fd *FitbitDownloader) getData(endpoint string) (*ActivityData, error) {
	err := fd.refreshAccessToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %v", endpoint, err)
	}

	req.Header.Set("Authorization", "Bearer "+fd.TokenInfo.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request for %s failed: %v", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to download %s data: %d", endpoint, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response for %s: %v", endpoint, err)
	}

	var data ActivityData

	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("failed to parse steps data JSON for %s: %v", endpoint, err)
	}

	// TODO remove once not needed
	// formattedJSON, err := json.MarshalIndent(data, "", "  ")
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to format JSON for %s: %v", endpoint, err)
	// }

	// filePath := filepath.Join(fd.DataDir, filename)
	// err = os.WriteFile(filePath, formattedJSON, 0644)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to save %s data: %v", endpoint, err)
	// }

	return &data, nil
}
