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
	default:
		// data = Store.GetHeartRateData()
	}
	line := generateLineChart(data)
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
func generateLineChart(data models.ChartData) *charts.Line {
	line := charts.NewLine()

	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: "macarons"}),
		charts.WithTitleOpts(opts.Title{
			Title:    data.Title,
			Subtitle: data.Subtitle,
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:            opts.Bool(true),
			Trigger:         "item",
			BackgroundColor: "#f5f5f5",
			BorderColor:     "#ccc",
			AxisPointer: &opts.AxisPointer{
				Type: "cross",
			}}),
		// charts.WithXAxisOpts(opts.XAxis{Data: data.XAxis}),
	)

	// X-axis data
	line.SetXAxis(data.XAxis)

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
