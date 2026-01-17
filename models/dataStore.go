package models

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
)

const MAX_DAYS = 28

type DataStore struct {
	StepsData     ChartData
	CaloriesData  ChartData
	ElevationData ChartData
	ProfileData   ProfileData
	HeartRateData HeartChartData
}

var Store = DataStore{
	StepsData:     ChartData{},
	CaloriesData:  ChartData{},
	ProfileData:   ProfileData{},
	ElevationData: ChartData{},
	HeartRateData: HeartChartData{},
}

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

	downloader := newFitbitDownloader(clientID, clientSecret, dataDir)

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
		stepData, err := downloader.DownloadActivities("steps", requestedDays)
		if err != nil {
			errChan <- fmt.Errorf("failed to download steps data: %w", err)
			return
		}
		if stepData != nil {
			processedData := stepData.ProcessData(strconv.Itoa(requestedDays))
			mu.Lock()
			Store.StepsData = processedData
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		caloriesData, err := downloader.DownloadActivities("calories", requestedDays)
		if err != nil {
			errChan <- fmt.Errorf("failed to download calories data: %w", err)
			return
		}
		if caloriesData != nil {
			processedData := caloriesData.ProcessData(strconv.Itoa(requestedDays))
			mu.Lock()
			Store.CaloriesData = processedData
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		elevationData, err := downloader.DownloadActivities("elevation", requestedDays)
		if err != nil {
			errChan <- fmt.Errorf("failed to download elevation data: %w", err)
			return
		}
		if elevationData != nil {
			processedData := elevationData.ProcessData(strconv.Itoa(requestedDays))
			mu.Lock()
			Store.ElevationData = processedData
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		heartRateData, err := downloader.DownloadHeartRate(requestedDays)
		if err != nil {
			errChan <- fmt.Errorf("failed to download heart rate data: %w", err)
			return
		}
		if heartRateData != nil {
			// Process and store heart rate data as needed
			processedData := heartRateData.ProcessData(strconv.Itoa(requestedDays))
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

// newFitbitDownloader creates a new downloader instance
func newFitbitDownloader(clientID, clientSecret, dataDir string) *FitbitDownloader {
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
			RedirectURI:  "http://fitbit-pi.local",
			RedirectPort: "8080",
		},
		DataDir: dataDir,
	}
}
