package server

import (
	"math"
	"strconv"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/gofit/models"
)

func generateLineChart(data models.ChartData, chartType string) *charts.Line {
	line := charts.NewLine()

	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: "macarons"}),
		charts.WithTitleOpts(opts.Title{
			Title:    data.Title,
			Subtitle: data.Subtitle,
		}),
		charts.WithXAxisOpts(opts.XAxis{
			AxisLabel: &opts.AxisLabel{
				Rotate: 45,
			},
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:            opts.Bool(true),
			Trigger:         "item",
			BackgroundColor: "#f5f5f5",
			BorderColor:     "#ccc",
			AxisPointer: &opts.AxisPointer{
				Type: "cross",
			}}),
	)

	// X-axis data
	line.SetXAxis(data.XAxis)

	if chartType == "elevation" {
		line.SetGlobalOptions(
			charts.WithYAxisOpts(opts.YAxis{
				Name:         "Elevation (m)",
				NameLocation: "middle",
				NameGap:      50,
				AxisLabel: &opts.AxisLabel{
					FontSize: 18,
				},
			}),
		)
	}

	// Add each series from the data
	for name, values := range data.Series {
		line.AddSeries(name, generateLineItems(values))
	}

	line.SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: opts.Bool(true)}))

	return line
}

// generateLineItems converts int slice to LineData slice
func generateLineItems(data []int) []opts.LineData {
	items := make([]opts.LineData, 0)
	for _, v := range data {
		items = append(items, opts.LineData{Value: v})
	}
	return items
}
func generateHeartRateChart(data models.HeartChartData) *charts.Bar {
	bar := charts.NewBar()

	bar.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: "macarons"}),
		charts.WithTitleOpts(opts.Title{
			Title:    data.Title,
			Subtitle: data.Subtitle,
		}),
		charts.WithLegendOpts(opts.Legend{
			Bottom:     "bottom",
			Padding:    8,
			ItemHeight: 20,
			Show:       opts.Bool(true),
			Selected: map[string]bool{
				"Out of Range (0 - 86 bpm)": false,
				"Fat Burn (86 - 121 bpm)":   true,
				"Cardio (121 - 147 bpm)":    true,
				"Peak (147 - 220 bpm)":      true,
			},
		}),
		charts.WithGridOpts(opts.Grid{
			Bottom: "20%", // Reserves space at bottom for legend
		}),
		charts.WithXAxisOpts(opts.XAxis{
			AxisLabel: &opts.AxisLabel{
				Rotate: 45,
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name:         "Percentage of Day",
			NameLocation: "middle",
			NameGap:      50,
			AxisLabel: &opts.AxisLabel{
				Formatter: "{value}%",
			},
			// Max:   100,
			// Scale: opts.Bool(true)
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger: "axis",
			AxisPointer: &opts.AxisPointer{
				Type: "shadow",
			},
			// Formatter:       "{b}: {c}%",
			BackgroundColor: "rgba(255, 255, 255, 0.9)", // Increased opacity to 0.9
			BorderColor:     "#ccc",
		}),
	)

	// Set X-axis data (dates)
	bar.SetXAxis(data.XAxis)

	// Define zone order (bottom to top of stack)
	zones := []string{"Out of Range", "Fat Burn", "Cardio", "Peak"}

	zoneRanges := map[string][2]int{
		"Out of Range": {0, 86},
		"Fat Burn":     {86, 121},
		"Cardio":       {121, 147},
		"Peak":         {147, 220},
	}

	// Add each zone as a series
	const minutesInDay float64 = 1440.0
	for _, zone := range zones {
		zoneData := make([]opts.BarData, len(data.Series["Heart Rate"]))

		for i, entry := range data.Series["Heart Rate"] {
			percentage := (float64(entry.Zones[zone]) / minutesInDay) * 100
			percentage = math.Round(percentage*10) / 10 // Round to 2 decimal places
			zoneData[i] = opts.BarData{Value: percentage}
		}
		// selected := true
		// if zone == "Out of Range" {
		// 	selected = false
		// }

		legendName := zone + " (" +
			strconv.Itoa(zoneRanges[zone][0]) + " - " +
			strconv.Itoa(zoneRanges[zone][1]) + " bpm)"
		bar.AddSeries(legendName, zoneData).
			SetSeriesOptions(
				charts.WithBarChartOpts(opts.BarChart{
					Stack: "total",
				}),
				// charts.WithLabelOpts(opts.Label{
				// 	Show: &selected,
				// }),
			)
	}

	return bar
}
