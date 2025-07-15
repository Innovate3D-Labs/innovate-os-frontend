package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// PrinterDiscoveryUI handles the printer discovery interface
type PrinterDiscoveryUI struct {
	app            fyne.App
	window         fyne.Window
	client         *BackendClient
	printerList    *widget.List
	scanButton     *widget.Button
	statusLabel    *widget.Label
	progressBar    *widget.ProgressBarInfinite
	printers       []DiscoveredPrinter
	isScanning     bool
	onConnect      func(printer DiscoveredPrinter)
}

// NewPrinterDiscoveryUI creates a new printer discovery UI
func NewPrinterDiscoveryUI(app fyne.App, client *BackendClient) *PrinterDiscoveryUI {
	ui := &PrinterDiscoveryUI{
		app:      app,
		client:   client,
		printers: []DiscoveredPrinter{},
	}
	
	ui.window = app.NewWindow("Printer Discovery")
	ui.window.Resize(fyne.NewSize(800, 600))
	ui.window.CenterOnScreen()
	
	ui.setupUI()
	return ui
}

// setupUI creates the UI layout
func (ui *PrinterDiscoveryUI) setupUI() {
	// Header
	header := container.NewVBox(
		widget.NewLabelWithStyle("Discover 3D Printers", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
	)
	
	// Status and progress
	ui.statusLabel = widget.NewLabel("Ready to scan for printers")
	ui.progressBar = widget.NewProgressBarInfinite()
	ui.progressBar.Hide()
	
	statusContainer := container.NewVBox(
		ui.statusLabel,
		ui.progressBar,
	)
	
	// Scan button
	ui.scanButton = widget.NewButtonWithIcon("Scan for Printers", theme.SearchIcon(), func() {
		ui.startDiscovery()
	})
	ui.scanButton.Importance = widget.HighImportance
	
	// Printer list
	ui.printerList = widget.NewList(
		func() int { return len(ui.printers) },
		func() fyne.CanvasObject {
			return ui.createPrinterItem()
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			ui.updatePrinterItem(i, o)
		},
	)
	
	// Instructions
	instructions := widget.NewCard("Instructions", "", widget.NewLabel(
		"1. Make sure your printer is powered on and connected via USB\n"+
		"2. Click 'Scan for Printers' to start discovery\n"+
		"3. Select a printer from the list and click 'Connect'\n"+
		"4. Discovery will test common baud rates automatically",
	))
	
	// Layout
	content := container.NewBorder(
		container.NewVBox(
			header,
			statusContainer,
			ui.scanButton,
			widget.NewSeparator(),
		),
		instructions,
		nil,
		nil,
		container.NewPadded(ui.printerList),
	)
	
	ui.window.SetContent(content)
}

// createPrinterItem creates a list item for a printer
func (ui *PrinterDiscoveryUI) createPrinterItem() fyne.CanvasObject {
	icon := widget.NewIcon(theme.ComputerIcon())
	name := widget.NewLabelWithStyle("Printer Name", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	details := widget.NewLabel("Details")
	connectBtn := widget.NewButtonWithIcon("Connect", theme.NavigateNextIcon(), nil)
	connectBtn.Importance = widget.HighImportance
	
	info := container.NewVBox(name, details)
	
	return container.NewBorder(
		nil, nil,
		icon,
		connectBtn,
		info,
	)
}

// updatePrinterItem updates a printer list item with data
func (ui *PrinterDiscoveryUI) updatePrinterItem(i widget.ListItemID, o fyne.CanvasObject) {
	if i >= len(ui.printers) {
		return
	}
	
	printer := ui.printers[i]
	border := o.(*fyne.Container)
	
	// Update icon based on compatibility
	icon := border.Objects[0].(*widget.Icon)
	if printer.IsCompatible {
		icon.SetResource(theme.ConfirmIcon())
	} else {
		icon.SetResource(theme.WarningIcon())
	}
	
	// Update info
	infoContainer := border.Objects[2].(*fyne.Container)
	nameLabel := infoContainer.Objects[0].(*widget.Label)
	detailsLabel := infoContainer.Objects[1].(*widget.Label)
	
	nameLabel.SetText(printer.Name)
	
	// Build details string
	details := fmt.Sprintf("Port: %s | Baud: %d | Firmware: %s",
		printer.Port, printer.BaudRate, printer.Firmware)
	
	if printer.Identity != nil && printer.Identity.SerialNumber != "" {
		details = fmt.Sprintf("SN: %s | %s", printer.Identity.SerialNumber, details)
	}
	
	detailsLabel.SetText(details)
	
	// Update connect button
	connectBtn := border.Objects[1].(*widget.Button)
	connectBtn.OnTapped = func() {
		ui.connectToPrinter(printer)
	}
	
	if !printer.IsCompatible {
		connectBtn.Disable()
		connectBtn.SetText("Incompatible")
	} else {
		connectBtn.Enable()
		connectBtn.SetText("Connect")
	}
}

// startDiscovery starts the printer discovery process
func (ui *PrinterDiscoveryUI) startDiscovery() {
	if ui.isScanning {
		return
	}
	
	ui.isScanning = true
	ui.printers = []DiscoveredPrinter{}
	ui.printerList.Refresh()
	
	ui.scanButton.Disable()
	ui.progressBar.Show()
	ui.statusLabel.SetText("Starting printer discovery...")
	
	// Start discovery
	go func() {
		err := ui.client.StartPrinterDiscovery()
		if err != nil {
			ui.app.SendNotification(&fyne.Notification{
				Title:   "Discovery Error",
				Content: err.Error(),
			})
			ui.resetUI()
			return
		}
		
		// Poll for status
		ui.pollDiscoveryStatus()
	}()
}

// pollDiscoveryStatus polls the discovery status
func (ui *PrinterDiscoveryUI) pollDiscoveryStatus() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	timeout := time.After(30 * time.Second)
	
	for {
		select {
		case <-ticker.C:
			status, err := ui.client.GetDiscoveryStatus()
			if err != nil {
				ui.statusLabel.SetText("Error checking status")
				continue
			}
			
			// Update UI with discovered printers
			if len(status.Discovered) > len(ui.printers) {
				ui.printers = status.Discovered
				ui.printerList.Refresh()
				ui.statusLabel.SetText(fmt.Sprintf("Found %d printer(s)", len(ui.printers)))
			}
			
			// Check if scanning is complete
			if !status.IsScanning {
				ui.discoveryComplete()
				return
			}
			
		case <-timeout:
			ui.statusLabel.SetText("Discovery timeout")
			ui.discoveryComplete()
			return
		}
	}
}

// discoveryComplete handles discovery completion
func (ui *PrinterDiscoveryUI) discoveryComplete() {
	ui.isScanning = false
	ui.scanButton.Enable()
	ui.progressBar.Hide()
	
	if len(ui.printers) == 0 {
		ui.statusLabel.SetText("No printers found. Make sure your printer is connected and powered on.")
		
		dialog.ShowInformation("No Printers Found",
			"No 3D printers were detected.\n\n"+
			"Please check:\n"+
			"• Printer is powered on\n"+
			"• USB cable is connected\n"+
			"• You have permission to access serial ports",
			ui.window)
	} else {
		ui.statusLabel.SetText(fmt.Sprintf("Discovery complete. Found %d printer(s)", len(ui.printers)))
		
		// Auto-select first compatible printer
		for i, printer := range ui.printers {
			if printer.IsCompatible {
				ui.printerList.Select(i)
				break
			}
		}
	}
}

// connectToPrinter connects to the selected printer
func (ui *PrinterDiscoveryUI) connectToPrinter(printer DiscoveredPrinter) {
	// Check if it's an Innovate3D printer with profile
	isInnovate3D := false
	var modelID string
	if printer.Manufacturer != nil {
		if mid, ok := printer.Manufacturer["model_id"]; ok && strings.HasPrefix(mid, "INNOVATE3D") {
			isInnovate3D = true
			modelID = mid
		}
	}
	
	confirmMessage := fmt.Sprintf("Connect to %s on %s?", printer.Name, printer.Port)
	if isInnovate3D {
		confirmMessage += "\n\nThis is an Innovate3D printer with automatic configuration."
	}
	
	dialog.ShowConfirm("Connect to Printer", confirmMessage,
		func(confirmed bool) {
			if !confirmed {
				return
			}
			
			ui.statusLabel.SetText(fmt.Sprintf("Connecting to %s...", printer.Name))
			ui.progressBar.Show()
			
			// Connect via backend
			go func() {
				err := ui.client.ConnectPrinter(printer)
				
				ui.progressBar.Hide()
				
				if err != nil {
					ui.statusLabel.SetText("Connection failed")
					dialog.ShowError(err, ui.window)
					return
				}
				
				ui.statusLabel.SetText(fmt.Sprintf("Connected to %s", printer.Name))
				
				// Notify success
				ui.app.SendNotification(&fyne.Notification{
					Title:   "Printer Connected",
					Content: fmt.Sprintf("Successfully connected to %s", printer.Name),
				})
				
				// If it's an Innovate3D printer, show profile UI
				if isInnovate3D && printer.Manufacturer != nil {
					// Create profile from manufacturer data
					profile := &PrinterProfile{
						ModelID:       modelID,
						ModelName:     printer.Name,
						PrintHeadType: printer.Manufacturer["printhead_type"],
						NozzleCount:   1, // Default
						Capabilities:  []string{}, // Would be populated from actual data
						BuildVolume: map[string]float64{
							"x": 300,
							"y": 300,
							"z": 400,
						},
					}
					
					// Parse nozzle count
					if nc, ok := printer.Manufacturer["nozzle_count"]; ok {
						if count, err := strconv.Atoi(nc); err == nil {
							profile.NozzleCount = count
						}
					}
					
					// Show profile UI
					profileUI := NewPrinterProfileUI(ui.app, printer, profile)
					profileUI.SetOnConfigure(func(config map[string]interface{}) {
						// Handle configuration updates
						log.Printf("Configuration updated: %v", config)
					})
					profileUI.Show()
				}
				
				// Call callback if set
				if ui.onConnect != nil {
					ui.onConnect(printer)
				}
				
				// Close window after short delay
				time.Sleep(1 * time.Second)
				ui.window.Close()
			}()
		},
		ui.window)
}

// resetUI resets the UI to initial state
func (ui *PrinterDiscoveryUI) resetUI() {
	ui.isScanning = false
	ui.scanButton.Enable()
	ui.progressBar.Hide()
	ui.statusLabel.SetText("Ready to scan for printers")
}

// SetOnConnect sets the callback for when a printer is connected
func (ui *PrinterDiscoveryUI) SetOnConnect(callback func(printer DiscoveredPrinter)) {
	ui.onConnect = callback
}

// Show shows the discovery window
func (ui *PrinterDiscoveryUI) Show() {
	ui.window.Show()
} 