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

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var AuthFlowInProgress bool

// Config holds the application configuration
type Config struct {
	ClientID     string `json:"client_id" validate:"required"`
	ClientSecret string `json:"client_secret" validate:"required"`
	RedirectURI  string `json:"redirect_uri"`
	RedirectPort string `json:"redirect_port"`
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
	ActivityType string
	Activities   []ActivityEntry
}

type ActivityEntry struct {
	DateTime string `json:"dateTime"`
	Value    string `json:"value"`
}

type ActivitiesHeartList struct {
	ActivitiesHeart []struct {
		DateTime string `json:"dateTime"`
		Value    struct {
			CustomHeartRateZones []struct {
				CaloriesOut float64 `json:"caloriesOut"`
				Max         int     `json:"max"`
				Min         int     `json:"min"`
				Minutes     int     `json:"minutes"`
				Name        string  `json:"name"`
			} `json:"customHeartRateZones"`
			HeartRateZones []struct {
				CaloriesOut float64 `json:"caloriesOut"`
				Max         int     `json:"max"`
				Min         int     `json:"min"`
				Minutes     int     `json:"minutes"`
				Name        string  `json:"name"`
			} `json:"heartRateZones"`
			RestingHeartRate int `json:"restingHeartRate"`
		} `json:"value"`
	} `json:"activities-heart"`
}

type CacheData struct {
	MaxDays   int            `json:"max_days"`
	Timestamp int64          `json:"timestamp"`
	Steps     ChartData      `json:"steps"`
	Calories  ChartData      `json:"calories"`
	Elevation ChartData      `json:"elevation"`
	HeartRate HeartChartData `json:"heart_rate"`
	Profile   ProfileData    `json:"profile"`
}

type RateLimitError struct {
	RetryAfter int    // Seconds until next request is allowed
	Message    string // Error message from Fitbit API
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("Rate limit exceeded: %s. Try again in %d seconds", e.Message, e.RetryAfter)
}

// UnmarshalJSON implements custom unmarshalling for ActivityData to handle Fitbit's activity data structure.
func (a *ActivityData) UnmarshalJSON(data []byte) error {
	// Custom unmarshal to handle the structure of Fitbit activity data
	var temp map[string][]ActivityEntry
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	for _, value := range temp {
		a.Activities = value
		break
	}

	return nil
}

func (s *ActivityData) ProcessData(days_back string) ChartData {
	// Convert StepsData to ChartData for visualization
	tag := language.Make(s.ActivityType)
	series := cases.Title(tag).String(s.ActivityType)
	chart := ChartData{
		Type:     s.ActivityType,
		Title:    fmt.Sprintf("%s Over Time", cases.Title(language.English).String(s.ActivityType)),
		Subtitle: fmt.Sprintf("Daily %s count", s.ActivityType),
		XAxis:    make([]string, len(s.Activities)),
		Series:   map[string][]int{series: make([]int, len(s.Activities))},
	}

	for i, entry := range s.Activities {
		val, err := strconv.Atoi(entry.Value)
		if err != nil {
			val = 0
		}
		t, err := time.Parse("01-02", entry.DateTime)
		if err != nil {
			chart.XAxis[i] = entry.DateTime // Fallback to raw date if parsing fails
		} else {
			chart.XAxis[i] = t.Weekday().String()[:3] + " " + t.Format("01-02")
		}

		chart.Series[series][i] = val
	}

	return chart
}

func (s *ActivitiesHeartList) ProcessData(days_back string) HeartChartData {
	chart := HeartChartData{
		Title:  "Heart Rate Over Time",
		XAxis:  make([]string, len(s.ActivitiesHeart)),
		Series: make(map[string][]HeartRateEntry),
	}

	chart.Series["Heart Rate"] = make([]HeartRateEntry, len(s.ActivitiesHeart))

	for i, entry := range s.ActivitiesHeart {
		t, err := time.Parse("2006-01-02", entry.DateTime)
		if err != nil {
			chart.XAxis[i] = entry.DateTime // Fallback to raw date if parsing fails
		} else {
			chart.XAxis[i] = t.Weekday().String()[:3] + " " + t.Format("01-02")
		}
		// Create heart rate entry with zones map
		heartRateEntry := HeartRateEntry{
			Zones:       make(map[string]int),
			RestingRate: entry.Value.RestingHeartRate,
		}

		// Add minutes for each zone
		for _, zone := range entry.Value.HeartRateZones {
			heartRateEntry.Zones[zone.Name] = zone.Minutes
		}

		chart.Series["Heart Rate"][i] = heartRateEntry

	}
	return chart
}

// StartAuthFlow initiates the OAuth authorization flow
func (fd *FitbitDownloader) StartAuthFlow() error {
	if AuthFlowInProgress {
		fmt.Println("Authorization flow is already in progress. Skipping...")
		return nil
	}
	AuthFlowInProgress = true // Set the flag

	defer func() {
		AuthFlowInProgress = false // Reset the flag after completion
	}()

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

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    "localhost:" + fd.Config.RedirectPort,
		Handler: mux,
	}
	// ERROR handling the index page for the server
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
			fd.callbackRunning = false
			server.Close()
		}()
	})

	log.Println("Waiting for authorization callback...")

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
func (fd *FitbitDownloader) RefreshAccessToken() error {
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
	err := fd.RefreshAccessToken()
	if err != nil {
		return nil, err
	}

	fmt.Println("Downloading user profile data...")
	// TODO implement some type of emssage when quota is hit
	url := "https://api.fitbit.com/1/user/-/profile.json"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+fd.TokenInfo.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if resp.StatusCode == 429 {
		retryAfterStr := resp.Header.Get("Retry-After")
		retryAfter, _ := strconv.Atoi(retryAfterStr)
		if retryAfter == 0 {
			retryAfter = 3600 // Default to 60 seconds if not provided
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &RateLimitError{
			RetryAfter: retryAfter,
			Message:    string(bodyBytes),
		}
	}
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

	// Unmarshal the JSON response into ProfileData struct
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

	// Populate Activity Type
	data.ActivityType = activity
	fmt.Printf("%s data downloaded successfully!\n", activity)
	return data, nil
}

func (fd *FitbitDownloader) DownloadHeartRate(days_back int) (*ActivitiesHeartList, error) {

	endData := time.Now().Format("2006-01-02")
	startDate := time.Now().AddDate(0, 0, -days_back).Format("2006-01-02")

	fmt.Printf("Reading heart rate data from %s to %s...\n", startDate, endData)

	endpoint := fmt.Sprintf("https://api.fitbit.com/1/user/-/activities/heart/date/%s/%s.json", startDate, endData)

	data, err := fd.getDataHeartRate(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to download heart rate data: %v", err)
	}
	fmt.Println("Heart rate data downloaded successfully!")
	return data, nil
}

func (fd *FitbitDownloader) getData(endpoint string) (*ActivityData, error) {

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
	return &data, nil
}

func (fd *FitbitDownloader) getDataHeartRate(endpoint string) (*ActivitiesHeartList, error) {

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
	var data ActivitiesHeartList

	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("failed to parse heart rate data JSON for %s: %v", endpoint, err)
	}
	return &data, nil

}
