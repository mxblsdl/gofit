package models

type ChartData struct {
	Title    string
	Subtitle string
	XAxis    []string
	Series   map[string][]int
}

type DataStore struct {
	StepsData     ChartData
	CaloriesData  ChartData
	ElevationData ChartData
	ProfileData   ProfileData
}
