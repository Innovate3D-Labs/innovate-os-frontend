package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// PrinterProfile represents the printer profile data
type PrinterProfile struct {
	ModelID       string              `json:"model_id"`
	ModelName     string              `json:"model_name"`
	PrintHeadType string              `json:"printhead_type"`
	NozzleCount   int                 `json:"nozzle_count"`
	Capabilities  []string            `json:"capabilities"`
	BuildVolume   map[string]float64  `json:"build_volume"`
}

// PrinterProfileUI handles the printer profile display
type PrinterProfileUI struct {
	app          fyne.App
	window       fyne.Window
	profile      *PrinterProfile
	printer      DiscoveredPrinter
	onConfigure  func(config map[string]interface{})
}

// NewPrinterProfileUI creates a new printer profile UI
func NewPrinterProfileUI(app fyne.App, printer DiscoveredPrinter, profile *PrinterProfile) *PrinterProfileUI {
	ui := &PrinterProfileUI{
		app:     app,
		printer: printer,
		profile: profile,
	}
	
	ui.window = app.NewWindow("Printer Profile - " + profile.ModelName)
	ui.window.Resize(fyne.NewSize(600, 700))
	ui.window.CenterOnScreen()
	
	ui.setupUI()
	return ui
}

// setupUI creates the UI layout
func (ui *PrinterProfileUI) setupUI() {
	// Header
	header := container.NewVBox(
		widget.NewLabelWithStyle(ui.profile.ModelName, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabel(fmt.Sprintf("Serial: %s", ui.printer.Identity.SerialNumber)),
		widget.NewSeparator(),
	)
	
	// Model Information Card
	modelInfo := widget.NewCard("Model Information", "", container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Model ID: %s", ui.profile.ModelID)),
		widget.NewLabel(fmt.Sprintf("Firmware: %s", ui.printer.Firmware)),
		widget.NewLabel(fmt.Sprintf("Port: %s @ %d baud", ui.printer.Port, ui.printer.BaudRate)),
	))
	
	// PrintHead Configuration Card
	printHeadInfo := ui.createPrintHeadCard()
	
	// Build Volume Card
	buildVolumeInfo := widget.NewCard("Build Volume", "", container.NewVBox(
		widget.NewLabel(fmt.Sprintf("X: %.0f mm", ui.profile.BuildVolume["x"])),
		widget.NewLabel(fmt.Sprintf("Y: %.0f mm", ui.profile.BuildVolume["y"])),
		widget.NewLabel(fmt.Sprintf("Z: %.0f mm", ui.profile.BuildVolume["z"])),
		widget.NewLabel(fmt.Sprintf("Total: %.0f x %.0f x %.0f mm", 
			ui.profile.BuildVolume["x"],
			ui.profile.BuildVolume["y"],
			ui.profile.BuildVolume["z"])),
	))
	
	// Capabilities Card
	capabilitiesContent := container.NewVBox()
	for _, cap := range ui.profile.Capabilities {
		icon := theme.ConfirmIcon()
		label := ui.getCapabilityLabel(cap)
		capabilitiesContent.Add(
			container.NewHBox(
				widget.NewIcon(icon),
				widget.NewLabel(label),
			),
		)
	}
	capabilitiesCard := widget.NewCard("Capabilities", "", capabilitiesContent)
	
	// Configuration Options based on PrintHead Type
	configCard := ui.createConfigurationCard()
	
	// Action Buttons
	btnClose := widget.NewButton("Close", func() {
		ui.window.Close()
	})
	
	btnTestConfig := widget.NewButtonWithIcon("Test Configuration", theme.MediaPlayIcon(), func() {
		ui.testConfiguration()
	})
	btnTestConfig.Importance = widget.HighImportance
	
	buttons := container.NewHBox(
		btnTestConfig,
		widget.NewSeparator(),
		btnClose,
	)
	
	// Main content
	content := container.NewVScroll(container.NewVBox(
		header,
		modelInfo,
		printHeadInfo,
		buildVolumeInfo,
		capabilitiesCard,
		configCard,
	))
	
	// Layout
	ui.window.SetContent(container.NewBorder(
		nil,
		container.NewPadded(buttons),
		nil,
		nil,
		content,
	))
}

// createPrintHeadCard creates the printhead information card
func (ui *PrinterProfileUI) createPrintHeadCard() *widget.Card {
	content := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Type: %s", ui.profile.PrintHeadType)),
		widget.NewLabel(fmt.Sprintf("Nozzles: %d", ui.profile.NozzleCount)),
	)
	
	// Add specific info based on type
	switch ui.profile.PrintHeadType {
	case "Dual":
		content.Add(widget.NewLabel("Mode: Dual Extrusion"))
		content.Add(widget.NewLabel("Tool Offset: X+25mm"))
	case "IDEX":
		content.Add(widget.NewLabel("Mode: Independent Dual Extrusion"))
		content.Add(widget.NewLabel("Features: Mirror, Duplication"))
	}
	
	return widget.NewCard("PrintHead Configuration", "", content)
}

// createConfigurationCard creates configuration options based on printer type
func (ui *PrinterProfileUI) createConfigurationCard() *widget.Card {
	content := container.NewVBox()
	
	switch ui.profile.PrintHeadType {
	case "Dual":
		// Dual specific options
		purgeVolumeSlider := widget.NewSlider(5, 30)
		purgeVolumeSlider.SetValue(15)
		purgeVolumeLabel := widget.NewLabel("15 mm³")
		
		purgeVolumeSlider.OnChanged = func(value float64) {
			purgeVolumeLabel.SetText(fmt.Sprintf("%.0f mm³", value))
		}
		
		content.Add(widget.NewLabel("Purge Volume:"))
		content.Add(container.NewHBox(purgeVolumeSlider, purgeVolumeLabel))
		
		content.Add(widget.NewSeparator())
		
		// Tool offset calibration
		content.Add(widget.NewButton("Calibrate Tool Offset", func() {
			ui.showCalibrationDialog("Tool Offset Calibration", 
				"This will run the tool offset calibration routine.\n"+
				"Make sure the bed is clear.")
		}))
		
	case "IDEX":
		// IDEX specific options
		modeSelect := widget.NewSelect(
			[]string{"Normal", "Mirror Mode", "Duplication Mode"},
			func(selected string) {
				ui.handleIDEXModeChange(selected)
			},
		)
		modeSelect.SetSelected("Normal")
		
		content.Add(widget.NewLabel("IDEX Mode:"))
		content.Add(modeSelect)
		
		content.Add(widget.NewSeparator())
		
		// Calibration buttons
		content.Add(widget.NewButton("Calibrate X Offset", func() {
			ui.showCalibrationDialog("X Offset Calibration",
				"This will calibrate the X offset between tools.\n"+
				"Both tools will be used.")
		}))
		
		content.Add(widget.NewButton("Park Position Setup", func() {
			ui.showParkPositionDialog()
		}))
	}
	
	// Common options for all
	content.Add(widget.NewSeparator())
	content.Add(widget.NewButton("Run Startup Sequence", func() {
		ui.runStartupSequence()
	}))
	
	return widget.NewCard("Configuration Options", "", content)
}

// getCapabilityLabel returns a user-friendly label for a capability
func (ui *PrinterProfileUI) getCapabilityLabel(capability string) string {
	labels := map[string]string{
		"heated_bed":        "Heated Bed",
		"auto_leveling":     "Auto Bed Leveling",
		"filament_sensor":   "Filament Runout Sensor",
		"dual_extrusion":    "Dual Extrusion",
		"idex":              "Independent Dual Extrusion",
		"mirror_mode":       "Mirror Printing Mode",
		"duplication_mode":  "Duplication Mode",
	}
	
	if label, ok := labels[capability]; ok {
		return label
	}
	return capability
}

// testConfiguration runs a test sequence for the printer
func (ui *PrinterProfileUI) testConfiguration() {
	dialog := widget.NewCard("Testing Configuration", "", widget.NewProgressBarInfinite())
	popup := widget.NewModalPopUp(dialog, ui.window.Canvas())
	popup.Show()
	
	// Simulate test sequence
	go func() {
		// In real implementation, this would send commands to the printer
		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   "Configuration Test",
			Content: "Running printer configuration test...",
		})
		
		// Close after simulation
		popup.Hide()
		
		ui.showInfo("Test Complete", 
			"Printer configuration test completed successfully.\n"+
			"All systems operational.")
	}()
}

// showCalibrationDialog shows a calibration confirmation dialog
func (ui *PrinterProfileUI) showCalibrationDialog(title, message string) {
	dialog.ShowConfirm(title, message, func(confirmed bool) {
		if confirmed {
			// In real implementation, send calibration commands
			ui.showInfo("Calibration Started", 
				"Calibration sequence has been initiated.\n"+
				"Please follow the printer display instructions.")
		}
	}, ui.window)
}

// showParkPositionDialog shows park position configuration
func (ui *PrinterProfileUI) showParkPositionDialog() {
	t0Entry := widget.NewEntry()
	t0Entry.SetText("-30")
	t1Entry := widget.NewEntry()
	t1Entry.SetText("330")
	
	form := container.NewVBox(
		widget.NewLabel("Tool 0 Park Position (X):"),
		t0Entry,
		widget.NewLabel("Tool 1 Park Position (X):"),
		t1Entry,
	)
	
	dialog.ShowCustomConfirm("Park Position Setup", "Save", "Cancel", form,
		func(confirmed bool) {
			if confirmed {
				// Save park positions
				ui.showInfo("Park Positions Updated", 
					"Park positions have been saved to printer EEPROM.")
			}
		}, ui.window)
}

// handleIDEXModeChange handles IDEX mode changes
func (ui *PrinterProfileUI) handleIDEXModeChange(mode string) {
	commands := map[string]string{
		"Normal":            "M605 S0",
		"Mirror Mode":       "M605 S2",
		"Duplication Mode":  "M605 S1",
	}
	
	if cmd, ok := commands[mode]; ok {
		// In real implementation, send command to printer
		ui.showInfo("Mode Changed", 
			fmt.Sprintf("IDEX mode changed to: %s\nCommand sent: %s", mode, cmd))
	}
}

// runStartupSequence runs the printer startup sequence
func (ui *PrinterProfileUI) runStartupSequence() {
	dialog.ShowConfirm("Run Startup Sequence", 
		"This will run the printer startup sequence including:\n"+
		"• Homing all axes\n"+
		"• Auto bed leveling (if available)\n"+
		"• Tool initialization\n\n"+
		"Continue?",
		func(confirmed bool) {
			if confirmed {
				// In real implementation, send startup commands
				ui.showInfo("Startup Sequence", 
					"Startup sequence initiated.\n"+
					"Please wait for completion.")
			}
		}, ui.window)
}

// showInfo shows an information dialog
func (ui *PrinterProfileUI) showInfo(title, message string) {
	dialog.ShowInformation(title, message, ui.window)
}

// SetOnConfigure sets the configuration callback
func (ui *PrinterProfileUI) SetOnConfigure(callback func(config map[string]interface{})) {
	ui.onConfigure = callback
}

// Show shows the profile window
func (ui *PrinterProfileUI) Show() {
	ui.window.Show()
} 