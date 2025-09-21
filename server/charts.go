package server

import (
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
