package models

import "sync"

type ChartData struct {
	Title    string
	Subtitle string
	XAxis    []string
	Series   map[string][]int
}

type DataStore struct {
	StepsData ChartData
	mu        sync.RWMutex
}

// UpdateStepsData safely updates steps chart data
func (ds *DataStore) UpdateStepsData(data ChartData) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.StepsData = data
}

// GetStepsData safely retrieves steps chart data
func (ds *DataStore) GetStepsData() ChartData {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.StepsData
}
