package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// TemperatureUI manages the temperature interface
type TemperatureUI struct {
	window        fyne.Window
	backend       *BackendClient
	
	// Chart
	chart         *TemperatureChart
	
	// Controls
	hotendTarget  *widget.Entry
	bedTarget     *widget.Entry
	setHotendBtn  *widget.Button
	setBedBtn     *widget.Button
	
	// Current values display
	hotendActual  *widget.Label
	bedActual     *widget.Label
	statusLabel   *widget.Label
	
	// Time range controls
	timeRangeSelect *widget.Select
	
	// Chart controls
	zoomSlider    *widget.Slider
	resetZoomBtn  *widget.Button
	exportBtn     *widget.Button
	clearBtn      *widget.Button
	
	// Auto-update
	updateTicker  *time.Ticker
	stopUpdate    chan bool
	
	// Content
	content       *fyne.Container
}

// NewTemperatureUI creates a new temperature interface
func NewTemperatureUI(window fyne.Window, backend *BackendClient) *TemperatureUI {
	ui := &TemperatureUI{
		window:     window,
		backend:    backend,
		chart:      NewTemperatureChart(),
		stopUpdate: make(chan bool),
	}
	
	ui.createControls()
	ui.createLayout()
	ui.setupCallbacks()
	ui.startAutoUpdate()
	
	return ui
}

// createControls creates all the UI controls
func (ui *TemperatureUI) createControls() {
	// Current temperature displays
	ui.hotendActual = widget.NewLabel("0°C")
	ui.hotendActual.TextStyle = fyne.TextStyle{Bold: true}
	
	ui.bedActual = widget.NewLabel("0°C")
	ui.bedActual.TextStyle = fyne.TextStyle{Bold: true}
	
	ui.statusLabel = widget.NewLabel("Standby")
	
	// Temperature target inputs
	ui.hotendTarget = widget.NewEntry()
	ui.hotendTarget.SetPlaceHolder("200")
	ui.hotendTarget.Resize(fyne.NewSize(80, 40))
	
	ui.bedTarget = widget.NewEntry()
	ui.bedTarget.SetPlaceHolder("60")
	ui.bedTarget.Resize(fyne.NewSize(80, 40))
	
	// Set temperature buttons
	ui.setHotendBtn = widget.NewButton("Set Hotend", func() {
		ui.setHotendTemperature()
	})
	ui.setHotendBtn.Importance = widget.HighImportance
	ui.setHotendBtn.Resize(fyne.NewSize(120, 50))
	
	ui.setBedBtn = widget.NewButton("Set Bed", func() {
		ui.setBedTemperature()
	})
	ui.setBedBtn.Importance = widget.HighImportance
	ui.setBedBtn.Resize(fyne.NewSize(120, 50))
	
	// Time range selector
	ui.timeRangeSelect = widget.NewSelect(
		[]string{"5 min", "10 min", "30 min", "1 hour", "2 hours", "6 hours"},
		func(selected string) {
			ui.setTimeRange(selected)
		},
	)
	ui.timeRangeSelect.SetSelected("30 min")
	
	// Zoom controls
	ui.zoomSlider = widget.NewSlider(0.1, 5.0)
	ui.zoomSlider.SetValue(1.0)
	ui.zoomSlider.OnChanged = func(value float64) {
		ui.chart.SetZoom(value)
	}
	
	ui.resetZoomBtn = widget.NewButton("Reset Zoom", func() {
		ui.zoomSlider.SetValue(1.0)
		ui.chart.SetZoom(1.0)
	})
	
	// Export and clear buttons
	ui.exportBtn = widget.NewButton("Export CSV", func() {
		ui.exportTemperatureData()
	})
	ui.exportBtn.Resize(fyne.NewSize(120, 40))
	
	ui.clearBtn = widget.NewButton("Clear Chart", func() {
		dialog.ShowConfirm("Clear Chart", 
			"Are you sure you want to clear all temperature data?",
			func(confirmed bool) {
				if confirmed {
					ui.chart.Clear()
				}
			}, ui.window)
	})
	ui.clearBtn.Importance = widget.DangerImportance
	ui.clearBtn.Resize(fyne.NewSize(120, 40))
}

// createLayout creates the UI layout
func (ui *TemperatureUI) createLayout() {
	// Current temperature card
	currentTempCard := widget.NewCard("Current Temperatures", "", container.NewVBox(
		container.NewGridWithColumns(4,
			widget.NewLabel("Hotend:"),
			ui.hotendActual,
			widget.NewLabel("Bed:"),
			ui.bedActual,
		),
		ui.statusLabel,
	))
	
	// Temperature control card
	controlCard := widget.NewCard("Temperature Control", "", container.NewVBox(
		// Hotend controls
		container.NewGridWithColumns(3,
			widget.NewLabel("Hotend Target:"),
			ui.hotendTarget,
			ui.setHotendBtn,
		),
		// Bed controls
		container.NewGridWithColumns(3,
			widget.NewLabel("Bed Target:"),
			ui.bedTarget,
			ui.setBedBtn,
		),
		// Quick preset buttons
		ui.createPresetButtons(),
	))
	
	// Chart controls card
	chartControlCard := widget.NewCard("Chart Controls", "", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Time Range:"),
			ui.timeRangeSelect,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("Zoom:"),
			ui.zoomSlider,
		),
		container.NewGridWithColumns(3,
			ui.resetZoomBtn,
			ui.exportBtn,
			ui.clearBtn,
		),
	))
	
	// Top controls
	topControls := container.NewGridWithColumns(3,
		currentTempCard,
		controlCard,
		chartControlCard,
	)
	
	// Chart takes up most of the space
	chartContainer := container.NewMax(ui.chart)
	
	ui.content = container.NewVBox(
		topControls,
		widget.NewCard("Temperature Chart", "", chartContainer),
	)
}

// createPresetButtons creates quick preset temperature buttons
func (ui *TemperatureUI) createPresetButtons() *fyne.Container {
	presets := []struct {
		name   string
		hotend float64
		bed    float64
	}{
		{"PLA", 200, 60},
		{"ABS", 240, 80},
		{"PETG", 230, 70},
		{"Cool Down", 0, 0},
	}
	
	buttons := make([]fyne.CanvasObject, len(presets))
	for i, preset := range presets {
		p := preset // Capture for closure
		btn := widget.NewButton(p.name, func() {
			ui.setPresetTemperatures(p.hotend, p.bed)
		})
		btn.Resize(fyne.NewSize(80, 40))
		buttons[i] = btn
	}
	
	return container.NewGridWithColumns(len(presets), buttons...)
}

// setupCallbacks sets up chart callbacks
func (ui *TemperatureUI) setupCallbacks() {
	ui.chart.SetExportCallback(func(data []TemperatureDataPoint) {
		ui.saveTemperatureDataToFile(data)
	})
}

// startAutoUpdate starts automatic temperature data updates
func (ui *TemperatureUI) startAutoUpdate() {
	ui.updateTicker = time.NewTicker(1 * time.Second)
	
	go func() {
		for {
			select {
			case <-ui.updateTicker.C:
				ui.updateTemperatureData()
				
			case <-ui.stopUpdate:
				ui.updateTicker.Stop()
				return
			}
		}
	}()
}

// updateTemperatureData fetches and updates temperature data
func (ui *TemperatureUI) updateTemperatureData() {
	// Get status from backend
	status, err := ui.backend.GetPrinterStatus()
	if err != nil {
		// Don't show error for every failed update
		return
	}
	
	// Update current temperature displays
	ui.hotendActual.SetText(fmt.Sprintf("%.1f°C", status.Temperature))
	ui.bedActual.SetText(fmt.Sprintf("%.1f°C", status.BedTemp))
	ui.statusLabel.SetText(status.Status)
	
	// Add data point to chart
	dataPoint := TemperatureDataPoint{
		Timestamp:      time.Now(),
		HotendActual:   status.Temperature,
		HotendTarget:   status.Temperature, // TODO: Get actual target from backend
		BedActual:      status.BedTemp,
		BedTarget:      status.BedTemp,     // TODO: Get actual target from backend
	}
	
	ui.chart.AddDataPoint(dataPoint)
}

// setHotendTemperature sets the hotend target temperature
func (ui *TemperatureUI) setHotendTemperature() {
	tempStr := ui.hotendTarget.Text
	if tempStr == "" {
		return
	}
	
	temp, err := strconv.ParseFloat(tempStr, 64)
	if err != nil {
		dialog.ShowError(fmt.Errorf("invalid temperature: %s", tempStr), ui.window)
		return
	}
	
	if temp < 0 || temp > 300 {
		dialog.ShowError(fmt.Errorf("temperature out of range (0-300°C): %.1f", temp), ui.window)
		return
	}
	
	err = ui.backend.SetTemperature("hotend", temp)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to set hotend temperature: %v", err), ui.window)
		return
	}
	
	ui.statusLabel.SetText(fmt.Sprintf("Setting hotend to %.1f°C", temp))
}

// setBedTemperature sets the bed target temperature
func (ui *TemperatureUI) setBedTemperature() {
	tempStr := ui.bedTarget.Text
	if tempStr == "" {
		return
	}
	
	temp, err := strconv.ParseFloat(tempStr, 64)
	if err != nil {
		dialog.ShowError(fmt.Errorf("invalid temperature: %s", tempStr), ui.window)
		return
	}
	
	if temp < 0 || temp > 120 {
		dialog.ShowError(fmt.Errorf("bed temperature out of range (0-120°C): %.1f", temp), ui.window)
		return
	}
	
	err = ui.backend.SetTemperature("bed", temp)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to set bed temperature: %v", err), ui.window)
		return
	}
	
	ui.statusLabel.SetText(fmt.Sprintf("Setting bed to %.1f°C", temp))
}

// setPresetTemperatures sets both hotend and bed temperatures
func (ui *TemperatureUI) setPresetTemperatures(hotend, bed float64) {
	ui.hotendTarget.SetText(fmt.Sprintf("%.0f", hotend))
	ui.bedTarget.SetText(fmt.Sprintf("%.0f", bed))
	
	// Set hotend first
	if hotend > 0 {
		err := ui.backend.SetTemperature("hotend", hotend)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to set hotend temperature: %v", err), ui.window)
			return
		}
	}
	
	// Then set bed
	if bed > 0 {
		err := ui.backend.SetTemperature("bed", bed)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to set bed temperature: %v", err), ui.window)
			return
		}
	}
	
	if hotend == 0 && bed == 0 {
		ui.statusLabel.SetText("Cooling down...")
	} else {
		ui.statusLabel.SetText(fmt.Sprintf("Setting preset: Hotend %.0f°C, Bed %.0f°C", hotend, bed))
	}
}

// setTimeRange sets the chart time range
func (ui *TemperatureUI) setTimeRange(rangeStr string) {
	var duration time.Duration
	
	switch rangeStr {
	case "5 min":
		duration = 5 * time.Minute
	case "10 min":
		duration = 10 * time.Minute
	case "30 min":
		duration = 30 * time.Minute
	case "1 hour":
		duration = 1 * time.Hour
	case "2 hours":
		duration = 2 * time.Hour
	case "6 hours":
		duration = 6 * time.Hour
	default:
		duration = 30 * time.Minute
	}
	
	ui.chart.SetTimeRange(duration)
}

// exportTemperatureData exports temperature data to CSV
func (ui *TemperatureUI) exportTemperatureData() {
	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()
		
		ui.chart.ExportData()
	}, ui.window)
}

// saveTemperatureDataToFile saves temperature data to a CSV file
func (ui *TemperatureUI) saveTemperatureDataToFile(data []TemperatureDataPoint) {
	// Create default filename
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("temperature_log_%s.csv", timestamp)
	
	// Get downloads directory
	homeDir, _ := os.UserHomeDir()
	downloadsDir := filepath.Join(homeDir, "Downloads")
	fullPath := filepath.Join(downloadsDir, filename)
	
	// Ensure downloads directory exists
	os.MkdirAll(downloadsDir, 0755)
	
	// Create and write CSV file
	file, err := os.Create(fullPath)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to create file: %v", err), ui.window)
		return
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	// Write header
	header := []string{
		"Timestamp",
		"Hotend Actual (°C)",
		"Hotend Target (°C)",
		"Bed Actual (°C)",
		"Bed Target (°C)",
	}
	writer.Write(header)
	
	// Write data
	for _, point := range data {
		record := []string{
			point.Timestamp.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("%.2f", point.HotendActual),
			fmt.Sprintf("%.2f", point.HotendTarget),
			fmt.Sprintf("%.2f", point.BedActual),
			fmt.Sprintf("%.2f", point.BedTarget),
		}
		writer.Write(record)
	}
	
	// Show success message
	dialog.ShowInformation("Export Complete", 
		fmt.Sprintf("Temperature data exported to:\n%s\n\n%d data points saved.", 
			fullPath, len(data)), ui.window)
}

// GetContent returns the UI content
func (ui *TemperatureUI) GetContent() *fyne.Container {
	return ui.content
}

// Stop stops the automatic updates
func (ui *TemperatureUI) Stop() {
	close(ui.stopUpdate)
}

// GetChart returns the temperature chart for external access
func (ui *TemperatureUI) GetChart() *TemperatureChart {
	return ui.chart
}

// AddTemperatureReading manually adds a temperature reading (for testing)
func (ui *TemperatureUI) AddTemperatureReading(hotendActual, hotendTarget, bedActual, bedTarget float64) {
	dataPoint := TemperatureDataPoint{
		Timestamp:      time.Now(),
		HotendActual:   hotendActual,
		HotendTarget:   hotendTarget,
		BedActual:      bedActual,
		BedTarget:      bedTarget,
	}
	
	ui.chart.AddDataPoint(dataPoint)
	
	// Update current displays
	ui.hotendActual.SetText(fmt.Sprintf("%.1f°C", hotendActual))
	ui.bedActual.SetText(fmt.Sprintf("%.1f°C", bedActual))
} 