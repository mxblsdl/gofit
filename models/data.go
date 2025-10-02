package models

// TODO add chart type into chart struct
type ChartData struct {
	Title    string
	Subtitle string
	XAxis    []string
	Series   map[string][]int
}

type HeartChartData struct {
	Title    string
	Subtitle string
	XAxis    []string
	Series   map[string][]HeartRateEntry
}

type HeartRateEntry struct {
	Zones       map[string]int  // Maps zone name to minutes spent in that zone
	RestingRate int
}

type DataStore struct {
	StepsData     ChartData
	CaloriesData  ChartData
	ElevationData ChartData
	ProfileData   ProfileData
	HeartRateData HeartChartData
}
