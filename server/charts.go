package server

import (
	"bytes"
	"html/template"
	"log"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/gofit/models"
	"github.com/gofit/templates"
)

func LineChartHandler(w http.ResponseWriter, r *http.Request) {
	chartType := r.URL.Query().Get("type")

	var data models.ChartData
	switch chartType {
	case "steps":
		data = models.Store.StepsData
	case "calories":
		data = models.Store.CaloriesData
	case "elevation":
		data = models.Store.ElevationData
	default:
		// data = Store.GetHeartRateData()
	}
	line := generateLineChart(data, chartType)
	log.Println("Generating line chart for type:", chartType)

	var buf bytes.Buffer
	err := line.Render(&buf)
	if err != nil {
		http.Error(w, "Failed to render chart", http.StatusInternalServerError)
		return
	}

	component := templates.LineChart(template.HTML(buf.String()), data.Title)
	templ.Handler(component).ServeHTTP(w, r)
}

// TODO move to separate file
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
