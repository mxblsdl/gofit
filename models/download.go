package models

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

var Store = DataStore{
	StepsData:     ChartData{},
	CaloriesData:  ChartData{},
	ProfileData:   ProfileData{},
	ElevationData: ChartData{},
	HeartRateData: HeartChartData{},
}

// NewFitbitDownloader creates a new downloader instance
func NewFitbitDownloader(clientID, clientSecret, dataDir string) *FitbitDownloader {
	// Create data directory if it doesn't exist
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		err := os.Mkdir(dataDir, 0755)
		if err != nil {
			log.Fatal("Failed to create data directory:", err)
		}
	}

	return &FitbitDownloader{
		Config: Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURI:  "http://localhost:8080",
			RedirectPort: "8080",
		},
		DataDir: dataDir,
	}
}

const MAX_DAYS = 28

func PopulateDataStore(clientID, clientSecret, dataDir string, requestedDays int) error {
	cache, err := loadCacheData(dataDir)
	if err == nil && isCacheValid(cache, 2) {
		log.Println("Using cached data")
		Store.StepsData = filterDataByDays(cache.Steps, requestedDays)
		Store.CaloriesData = filterDataByDays(cache.Calories, requestedDays)
		Store.ElevationData = filterDataByDays(cache.Elevation, requestedDays)
		Store.HeartRateData = filterHeartDataByDays(cache.HeartRate, requestedDays)
		Store.ProfileData = cache.Profile
		return nil
	}

	downloader := NewFitbitDownloader(clientID, clientSecret, dataDir)

	// Check if we already have token information
	err = downloader.LoadTokenInfo()
	if err != nil {
		// First time authentication (only needed once)
		// This will open your browser for authorization
		fmt.Println("No token information found. Starting authorization flow...")
		err = downloader.StartAuthFlow()
		if err != nil {
			fmt.Println("Authorization failed:", err)
			return err
		}
	} else {
		// Refresh the access token if it exists
		fmt.Println("Refreshing access token...")
		err = downloader.RefreshAccessToken()
		if err != nil {
			fmt.Println("Failed to refresh access token:", err)
			return err
		}
	}
	// Check data refresh timestamp

	profileData, err := downloader.DownloadProfile()
	if err != nil {
		log.Fatal("Failed to download profile:", err)
	}
	if profileData != nil {
		Store.ProfileData = *profileData
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	errChan := make(chan error, 4) // Buffer size of 3 to hold potential errors from goroutines

	wg.Add(4)
	go func() {
		defer wg.Done()
		stepData, err := downloader.DownloadActivities("steps", MAX_DAYS)
		if err != nil {
			errChan <- fmt.Errorf("failed to download steps data: %w", err)
			return
		}
		if stepData != nil {
			processedData := stepData.ProcessData(strconv.Itoa(MAX_DAYS))
			mu.Lock()
			Store.StepsData = processedData
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		caloriesData, err := downloader.DownloadActivities("calories", MAX_DAYS)
		if err != nil {
			errChan <- fmt.Errorf("failed to download calories data: %w", err)
			return
		}
		if caloriesData != nil {
			processedData := caloriesData.ProcessData(strconv.Itoa(MAX_DAYS))
			mu.Lock()
			Store.CaloriesData = processedData
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		elevationData, err := downloader.DownloadActivities("elevation", MAX_DAYS)
		if err != nil {
			errChan <- fmt.Errorf("failed to download elevation data: %w", err)
			return
		}
		if elevationData != nil {
			processedData := elevationData.ProcessData(strconv.Itoa(MAX_DAYS))
			mu.Lock()
			Store.ElevationData = processedData
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		heartRateData, err := downloader.DownloadHeartRate(MAX_DAYS)
		if err != nil {
			errChan <- fmt.Errorf("failed to download heart rate data: %w", err)
			return
		}
		if heartRateData != nil {
			// Process and store heart rate data as needed
			processedData := heartRateData.ProcessData(strconv.Itoa(MAX_DAYS))
			mu.Lock()
			Store.HeartRateData = processedData
			mu.Unlock()
		}
	}()

	wg.Wait()
	close(errChan)

	// Check for errors from goroutines
	for err := range errChan {
		log.Println(err)
	}
	// populate data timestamp and write data to disk
	err = cacheData(downloader)
	if err != nil {
		log.Printf("Failed to cache data: %v", err)
	}

	return nil
}

func cacheData(downloader *FitbitDownloader) error {
	cache := CacheData{
		Timestamp: time.Now().Unix(),
		MaxDays:   MAX_DAYS,
		Steps:     Store.StepsData,
		Calories:  Store.CaloriesData,
		Elevation: Store.ElevationData,
		HeartRate: Store.HeartRateData,
		Profile:   Store.ProfileData,
	}

	// Marshal the data with indentation for readability
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	// Write to cache file
	cacheFile := filepath.Join(downloader.DataDir, "cache.json")
	err = os.WriteFile(cacheFile, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}
	return nil
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

func loadCacheData(dataDir string) (*CacheData, error) {
	cacheFile := filepath.Join(dataDir, "cache.json")

	// check if cache file exists
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("cache file does not exist")
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache CacheData
	err = json.Unmarshal(data, &cache)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache data: %w", err)
	}

	return &cache, nil
}

func isCacheValid(cache *CacheData, maxAgeHours int) bool {
	cacheTime := time.Unix(cache.Timestamp, 0)
	age := time.Since(cacheTime)
	return age.Hours() <= float64(maxAgeHours)
}

func filterDataByDays(data ChartData, days int) ChartData {
	if len(data.XAxis) <= days {
		return data
	}

	startIdx := len(data.XAxis) - days

	filtered := ChartData{
		Title:    data.Title,
		Subtitle: data.Subtitle,
		XAxis:    data.XAxis[startIdx:],
		Series:   make(map[string][]int),
	}

	for key, values := range data.Series {
		filtered.Series[key] = values[startIdx:]
	}
	return filtered
}

func filterHeartDataByDays(data HeartChartData, days int) HeartChartData {
	if len(data.XAxis) <= days {
		return data
	}

	startIdx := len(data.XAxis) - days

	filtered := HeartChartData{
		Title:    data.Title,
		Subtitle: data.Subtitle,
		XAxis:    data.XAxis[startIdx:],
		Series:   make(map[string][]HeartRateEntry),
	}

	for key, values := range data.Series {
		filtered.Series[key] = values[startIdx:]
	}
	return filtered
}
