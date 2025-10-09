package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
