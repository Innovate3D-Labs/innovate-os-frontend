package main

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// TemperatureDataPoint represents a single temperature measurement
type TemperatureDataPoint struct {
	Timestamp      time.Time
	HotendActual   float64
	HotendTarget   float64
	BedActual      float64
	BedTarget      float64
}

// TemperatureChart displays real-time temperature data
type TemperatureChart struct {
	widget.BaseWidget
	
	// Data
	dataPoints    []TemperatureDataPoint
	maxDataPoints int
	
	// Chart settings
	width         float32
	height        float32
	timeRange     time.Duration // How much time to show
	minTemp       float64
	maxTemp       float64
	
	// Zoom and pan
	zoomLevel     float64
	panOffsetX    float64
	panOffsetY    float64
	
	// Colors
	hotendActualColor color.Color
	hotendTargetColor color.Color
	bedActualColor    color.Color
	bedTargetColor    color.Color
	gridColor         color.Color
	textColor         color.Color
	
	// Interaction
	isDragging    bool
	lastDragPos   fyne.Position
	
	// Export callback
	onExport      func([]TemperatureDataPoint)
}

// NewTemperatureChart creates a new temperature chart
func NewTemperatureChart() *TemperatureChart {
	chart := &TemperatureChart{
		dataPoints:    make([]TemperatureDataPoint, 0),
		maxDataPoints: 1800, // 30 minutes at 1 second intervals
		timeRange:     30 * time.Minute,
		minTemp:       0,
		maxTemp:       300,
		zoomLevel:     1.0,
		
		// Colors
		hotendActualColor: color.NRGBA{R: 255, G: 69, B: 58, A: 255},   // Red
		hotendTargetColor: color.NRGBA{R: 255, G: 149, B: 0, A: 255},   // Orange
		bedActualColor:    color.NRGBA{R: 52, G: 199, B: 89, A: 255},   // Green
		bedTargetColor:    color.NRGBA{R: 48, G: 176, B: 199, A: 255},  // Blue
		gridColor:         color.NRGBA{R: 200, G: 200, B: 200, A: 128}, // Light gray
		textColor:         color.NRGBA{R: 28, G: 28, B: 30, A: 255},    // Dark
	}
	
	chart.ExtendBaseWidget(chart)
	return chart
}

// AddDataPoint adds a new temperature measurement
func (t *TemperatureChart) AddDataPoint(point TemperatureDataPoint) {
	t.dataPoints = append(t.dataPoints, point)
	
	// Remove old data points
	if len(t.dataPoints) > t.maxDataPoints {
		t.dataPoints = t.dataPoints[1:]
	}
	
	// Auto-scale Y axis
	t.updateScale()
	
	t.Refresh()
}

// updateScale automatically adjusts the temperature scale
func (t *TemperatureChart) updateScale() {
	if len(t.dataPoints) == 0 {
		return
	}
	
	minTemp := math.Inf(1)
	maxTemp := math.Inf(-1)
	
	for _, point := range t.dataPoints {
		temps := []float64{point.HotendActual, point.HotendTarget, point.BedActual, point.BedTarget}
		for _, temp := range temps {
			if temp > 0 && temp < minTemp {
				minTemp = temp
			}
			if temp > maxTemp {
				maxTemp = temp
			}
		}
	}
	
	// Add some padding
	padding := (maxTemp - minTemp) * 0.1
	t.minTemp = math.Max(0, minTemp-padding)
	t.maxTemp = maxTemp + padding
	
	// Ensure minimum range
	if t.maxTemp-t.minTemp < 50 {
		center := (t.maxTemp + t.minTemp) / 2
		t.minTemp = center - 25
		t.maxTemp = center + 25
	}
}

// CreateRenderer creates the chart renderer
func (t *TemperatureChart) CreateRenderer() fyne.WidgetRenderer {
	return &temperatureChartRenderer{chart: t}
}

// temperatureChartRenderer renders the temperature chart
type temperatureChartRenderer struct {
	chart *TemperatureChart
}

func (r *temperatureChartRenderer) Layout(size fyne.Size) {
	r.chart.width = size.Width
	r.chart.height = size.Height
}

func (r *temperatureChartRenderer) MinSize() fyne.Size {
	return fyne.NewSize(400, 300)
}

func (r *temperatureChartRenderer) Refresh() {
	// Refresh is handled by the canvas objects
}

func (r *temperatureChartRenderer) Destroy() {
	// Nothing to destroy
}

func (r *temperatureChartRenderer) Objects() []fyne.CanvasObject {
	if len(r.chart.dataPoints) == 0 {
		// Show "No data" message
		text := canvas.NewText("No temperature data available", r.chart.textColor)
		text.Alignment = fyne.TextAlignCenter
		text.Move(fyne.NewPos(r.chart.width/2-100, r.chart.height/2))
		return []fyne.CanvasObject{text}
	}
	
	objects := []fyne.CanvasObject{}
	
	// Draw grid
	objects = append(objects, r.drawGrid()...)
	
	// Draw temperature lines
	objects = append(objects, r.drawTemperatureLines()...)
	
	// Draw legend
	objects = append(objects, r.drawLegend()...)
	
	// Draw axes labels
	objects = append(objects, r.drawAxesLabels()...)
	
	return objects
}

// drawGrid draws the background grid
func (r *temperatureChartRenderer) drawGrid() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}
	
	chartArea := r.getChartArea()
	
	// Vertical grid lines (time)
	numVerticalLines := 6
	for i := 0; i <= numVerticalLines; i++ {
		x := chartArea.Min.X + float32(i)*chartArea.Size().Width/float32(numVerticalLines)
		line := canvas.NewLine(r.chart.gridColor)
		line.Position1 = fyne.NewPos(x, chartArea.Min.Y)
		line.Position2 = fyne.NewPos(x, chartArea.Max.Y)
		line.StrokeWidth = 1
		objects = append(objects, line)
	}
	
	// Horizontal grid lines (temperature)
	numHorizontalLines := 6
	for i := 0; i <= numHorizontalLines; i++ {
		y := chartArea.Min.Y + float32(i)*chartArea.Size().Height/float32(numHorizontalLines)
		line := canvas.NewLine(r.chart.gridColor)
		line.Position1 = fyne.NewPos(chartArea.Min.X, y)
		line.Position2 = fyne.NewPos(chartArea.Max.X, y)
		line.StrokeWidth = 1
		objects = append(objects, line)
	}
	
	return objects
}

// drawTemperatureLines draws the temperature data lines
func (r *temperatureChartRenderer) drawTemperatureLines() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}
	chartArea := r.getChartArea()
	
	if len(r.chart.dataPoints) < 2 {
		return objects
	}
	
	// Calculate time range to display
	now := time.Now()
	startTime := now.Add(-r.chart.timeRange)
	
	// Filter data points within time range
	visiblePoints := []TemperatureDataPoint{}
	for _, point := range r.chart.dataPoints {
		if point.Timestamp.After(startTime) {
			visiblePoints = append(visiblePoints, point)
		}
	}
	
	if len(visiblePoints) < 2 {
		return objects
	}
	
	// Helper function to convert data point to screen coordinates
	pointToScreen := func(timestamp time.Time, temp float64) fyne.Position {
		// X: time position
		timeDiff := timestamp.Sub(startTime).Seconds()
		totalTime := r.chart.timeRange.Seconds()
		x := chartArea.Min.X + float32(timeDiff/totalTime)*chartArea.Size().Width
		
		// Y: temperature position (inverted because screen Y grows downward)
		tempRatio := (temp - r.chart.minTemp) / (r.chart.maxTemp - r.chart.minTemp)
		y := chartArea.Max.Y - float32(tempRatio)*chartArea.Size().Height
		
		return fyne.NewPos(x, y)
	}
	
	// Draw lines for each temperature type
	lines := []struct {
		getValue func(TemperatureDataPoint) float64
		color    color.Color
		width    float32
	}{
		{func(p TemperatureDataPoint) float64 { return p.HotendActual }, r.chart.hotendActualColor, 2},
		{func(p TemperatureDataPoint) float64 { return p.HotendTarget }, r.chart.hotendTargetColor, 1},
		{func(p TemperatureDataPoint) float64 { return p.BedActual }, r.chart.bedActualColor, 2},
		{func(p TemperatureDataPoint) float64 { return p.BedTarget }, r.chart.bedTargetColor, 1},
	}
	
	for _, lineConfig := range lines {
		for i := 0; i < len(visiblePoints)-1; i++ {
			point1 := visiblePoints[i]
			point2 := visiblePoints[i+1]
			
			temp1 := lineConfig.getValue(point1)
			temp2 := lineConfig.getValue(point2)
			
			// Skip if either temperature is 0 (not set)
			if temp1 <= 0 || temp2 <= 0 {
				continue
			}
			
			pos1 := pointToScreen(point1.Timestamp, temp1)
			pos2 := pointToScreen(point2.Timestamp, temp2)
			
			line := canvas.NewLine(lineConfig.color)
			line.Position1 = pos1
			line.Position2 = pos2
			line.StrokeWidth = lineConfig.width
			objects = append(objects, line)
		}
	}
	
	return objects
}

// drawLegend draws the temperature legend
func (r *temperatureChartRenderer) drawLegend() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}
	
	legendItems := []struct {
		label string
		color color.Color
		width float32
	}{
		{"Hotend Actual", r.chart.hotendActualColor, 2},
		{"Hotend Target", r.chart.hotendTargetColor, 1},
		{"Bed Actual", r.chart.bedActualColor, 2},
		{"Bed Target", r.chart.bedTargetColor, 1},
	}
	
	startY := float32(10)
	lineHeight := float32(20)
	
	for i, item := range legendItems {
		y := startY + float32(i)*lineHeight
		
		// Color indicator line
		line := canvas.NewLine(item.color)
		line.Position1 = fyne.NewPos(10, y)
		line.Position2 = fyne.NewPos(30, y)
		line.StrokeWidth = item.width
		objects = append(objects, line)
		
		// Label text
		text := canvas.NewText(item.label, r.chart.textColor)
		text.Move(fyne.NewPos(35, y-8))
		text.TextSize = 12
		objects = append(objects, text)
	}
	
	return objects
}

// drawAxesLabels draws the temperature and time labels
func (r *temperatureChartRenderer) drawAxesLabels() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}
	chartArea := r.getChartArea()
	
	// Temperature labels (Y-axis)
	numTempLabels := 6
	for i := 0; i <= numTempLabels; i++ {
		ratio := float64(i) / float64(numTempLabels)
		temp := r.chart.minTemp + ratio*(r.chart.maxTemp-r.chart.minTemp)
		y := chartArea.Max.Y - float32(ratio)*chartArea.Size().Height
		
		text := canvas.NewText(fmt.Sprintf("%.0f°C", temp), r.chart.textColor)
		text.Move(fyne.NewPos(chartArea.Min.X-40, y-8))
		text.TextSize = 10
		objects = append(objects, text)
	}
	
	// Time labels (X-axis)
	numTimeLabels := 6
	now := time.Now()
	for i := 0; i <= numTimeLabels; i++ {
		ratio := float64(i) / float64(numTimeLabels)
		timeOffset := -r.chart.timeRange.Seconds() + ratio*r.chart.timeRange.Seconds()
		timestamp := now.Add(time.Duration(timeOffset) * time.Second)
		x := chartArea.Min.X + float32(ratio)*chartArea.Size().Width
		
		text := canvas.NewText(timestamp.Format("15:04"), r.chart.textColor)
		text.Move(fyne.NewPos(x-15, chartArea.Max.Y+5))
		text.TextSize = 10
		objects = append(objects, text)
	}
	
	return objects
}

// getChartArea returns the area available for drawing the chart (excluding margins)
func (r *temperatureChartRenderer) getChartArea() fyne.Rectangle {
	margin := float32(50)
	legendHeight := float32(100)
	
	return fyne.NewRectangle(
		fyne.NewPos(margin, legendHeight),
		fyne.NewSize(r.chart.width-2*margin, r.chart.height-legendHeight-margin),
	)
}

// SetTimeRange sets the time range to display
func (t *TemperatureChart) SetTimeRange(duration time.Duration) {
	t.timeRange = duration
	t.Refresh()
}

// SetZoom sets the zoom level
func (t *TemperatureChart) SetZoom(zoom float64) {
	t.zoomLevel = math.Max(0.1, math.Min(10.0, zoom))
	t.Refresh()
}

// Pan adjusts the pan offset
func (t *TemperatureChart) Pan(deltaX, deltaY float64) {
	t.panOffsetX += deltaX
	t.panOffsetY += deltaY
	t.Refresh()
}

// GetCurrentTemperatures returns the latest temperature readings
func (t *TemperatureChart) GetCurrentTemperatures() *TemperatureDataPoint {
	if len(t.dataPoints) == 0 {
		return nil
	}
	latest := t.dataPoints[len(t.dataPoints)-1]
	return &latest
}

// ExportData triggers the export callback with current data
func (t *TemperatureChart) ExportData() {
	if t.onExport != nil {
		t.onExport(t.dataPoints)
	}
}

// SetExportCallback sets the callback for data export
func (t *TemperatureChart) SetExportCallback(callback func([]TemperatureDataPoint)) {
	t.onExport = callback
}

// Clear removes all data points
func (t *TemperatureChart) Clear() {
	t.dataPoints = make([]TemperatureDataPoint, 0)
	t.Refresh()
} 