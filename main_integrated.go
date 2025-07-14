package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"image/color"
	"fyne.io/fyne/v2/canvas"
	"log"
	"fmt"
	"time"
	"strconv"
	"strings"
)

// IntegratedApp represents the main application with backend integration
type IntegratedApp struct {
	app           fyne.App
	window        fyne.Window
	content       *container.Border
	sidebar       *container.VBox
	mainView      *container.VBox
	
	// Backend integration
	backend       *BackendClient
	statusChan    chan PrinterStatus
	
	// UI Components for real-time updates
	tempLabel     *widget.Label
	progressBar   *widget.ProgressBar
	progressLabel *widget.Label
	positionLabel *widget.Label
	statusLabel   *widget.Label
	logEntry      *widget.Entry
	
	// Current state
	currentStatus PrinterStatus
	printJobs     []PrintJob
	selectedFile  string
}

func NewIntegratedApp() *IntegratedApp {
	a := app.New()
	a.Settings().SetTheme(&InnovateTheme{})
	
	w := a.NewWindow("Innovate OS - 3D Printer Control")
	w.Resize(fyne.NewSize(1024, 600))
	w.SetFullScreen(true)
	
	// Initialize backend client
	backend := NewBackendClient("localhost:8080")
	
	return &IntegratedApp{
		app:        a,
		window:     w,
		backend:    backend,
		statusChan: make(chan PrinterStatus, 100),
	}
}

func (app *IntegratedApp) initializeBackend() {
	// Connect to WebSocket for real-time updates
	err := app.backend.ConnectWebSocket()
	if err != nil {
		log.Printf("Failed to connect to WebSocket: %v", err)
		app.showError("Backend Connection Error", fmt.Sprintf("Failed to connect to printer backend:\n%v", err))
		return
	}
	
	// Start listening for updates
	go app.backend.ListenForUpdates(app.statusChan)
	
	// Start update handler
	go app.handleStatusUpdates()
	
	// Initial status fetch
	app.refreshStatus()
}

func (app *IntegratedApp) handleStatusUpdates() {
	for status := range app.statusChan {
		app.currentStatus = status
		app.updateUI()
	}
}

func (app *IntegratedApp) updateUI() {
	if app.tempLabel != nil {
		app.tempLabel.SetText(fmt.Sprintf("Hotend: %.1f°C | Bed: %.1f°C", 
			app.currentStatus.Temperature, app.currentStatus.BedTemp))
	}
	
	if app.progressBar != nil {
		app.progressBar.SetValue(app.currentStatus.Progress)
	}
	
	if app.progressLabel != nil {
		app.progressLabel.SetText(fmt.Sprintf("Layer %d/%d", 
			app.currentStatus.CurrentLayer, app.currentStatus.TotalLayers))
	}
	
	if app.positionLabel != nil {
		app.positionLabel.SetText(fmt.Sprintf("X: %.1f | Y: %.1f | Z: %.1f", 
			app.currentStatus.PositionX, app.currentStatus.PositionY, app.currentStatus.PositionZ))
	}
	
	if app.statusLabel != nil {
		app.statusLabel.SetText(fmt.Sprintf("Status: %s", app.currentStatus.Status))
	}
}

func (app *IntegratedApp) refreshStatus() {
	status, err := app.backend.GetPrinterStatus()
	if err != nil {
		log.Printf("Failed to get printer status: %v", err)
		return
	}
	
	app.currentStatus = *status
	app.updateUI()
}

func (app *IntegratedApp) refreshPrintJobs() {
	jobs, err := app.backend.GetPrintJobs()
	if err != nil {
		log.Printf("Failed to get print jobs: %v", err)
		return
	}
	
	app.printJobs = jobs
}

func (app *IntegratedApp) refreshLogs() {
	logs, err := app.backend.GetSystemLogs()
	if err != nil {
		log.Printf("Failed to get system logs: %v", err)
		return
	}
	
	if app.logEntry != nil {
		app.logEntry.SetText(strings.Join(logs, "\n"))
	}
}

func (app *IntegratedApp) showError(title, message string) {
	dialog.ShowError(fmt.Errorf(message), app.window)
}

func (app *IntegratedApp) showInfo(title, message string) {
	dialog.ShowInformation(title, message, app.window)
}

func (app *IntegratedApp) createIntegratedSidebar() *container.VBox {
	// Navigation buttons
	btnDashboard := widget.NewButton("Dashboard", func() {
		app.showIntegratedDashboard()
	})
	btnDashboard.Resize(fyne.NewSize(200, 60))
	btnDashboard.Importance = widget.HighImportance
	
	btnPrint := widget.NewButton("Print Control", func() {
		app.showIntegratedPrintControl()
	})
	btnPrint.Resize(fyne.NewSize(200, 60))
	
	btnFiles := widget.NewButton("Files", func() {
		app.showIntegratedFiles()
	})
	btnFiles.Resize(fyne.NewSize(200, 60))
	
	btnSettings := widget.NewButton("Settings", func() {
		app.showIntegratedSettings()
	})
	btnSettings.Resize(fyne.NewSize(200, 60))
	
	// Emergency stop button
	btnEmergencyStop := widget.NewButton("EMERGENCY STOP", func() {
		app.handleEmergencyStop()
	})
	btnEmergencyStop.Resize(fyne.NewSize(200, 80))
	btnEmergencyStop.Importance = widget.DangerImportance
	
	// Connection status
	connectionStatus := widget.NewLabel("Connected to Backend")
	if !app.currentStatus.IsConnected {
		connectionStatus.SetText("Disconnected")
	}
	
	sidebar := container.NewVBox(
		widget.NewCard("", "", canvas.NewText("Innovate OS", color.NRGBA{R: 28, G: 28, B: 30, A: 255})),
		connectionStatus,
		widget.NewSeparator(),
		btnDashboard,
		btnPrint,
		btnFiles,
		btnSettings,
		layout.NewSpacer(),
		btnEmergencyStop,
	)
	
	return sidebar
}

func (app *IntegratedApp) showIntegratedDashboard() {
	// Real-time temperature card
	app.tempLabel = widget.NewLabel(fmt.Sprintf("Hotend: %.1f°C | Bed: %.1f°C", 
		app.currentStatus.Temperature, app.currentStatus.BedTemp))
	tempCard := widget.NewCard("Temperature", "", app.tempLabel)
	
	// Real-time progress card
	app.progressBar = widget.NewProgressBar()
	app.progressBar.SetValue(app.currentStatus.Progress)
	app.progressLabel = widget.NewLabel(fmt.Sprintf("Layer %d/%d", 
		app.currentStatus.CurrentLayer, app.currentStatus.TotalLayers))
	progressCard := widget.NewCard("Print Progress", "", 
		container.NewVBox(app.progressBar, app.progressLabel))
	
	// Real-time position card
	app.positionLabel = widget.NewLabel(fmt.Sprintf("X: %.1f | Y: %.1f | Z: %.1f", 
		app.currentStatus.PositionX, app.currentStatus.PositionY, app.currentStatus.PositionZ))
	app.statusLabel = widget.NewLabel(fmt.Sprintf("Status: %s", app.currentStatus.Status))
	positionCard := widget.NewCard("Position & Status", "", 
		container.NewVBox(app.positionLabel, app.statusLabel))
	
	topRow := container.NewGridWithColumns(3, tempCard, progressCard, positionCard)
	
	// Real-time system log
	app.logEntry = widget.NewEntry()
	app.logEntry.MultiLine = true
	app.refreshLogs()
	
	logCard := widget.NewCard("System Log", "", app.logEntry)
	
	// Add refresh button
	refreshBtn := widget.NewButton("Refresh", func() {
		app.refreshStatus()
		app.refreshLogs()
	})
	
	app.mainView = container.NewVBox(
		topRow,
		refreshBtn,
		logCard,
	)
	
	app.updateMainContent()
}

func (app *IntegratedApp) showIntegratedPrintControl() {
	// Print control buttons with backend integration
	btnStart := widget.NewButton("Start Print", func() {
		if app.selectedFile == "" {
			app.showError("No File Selected", "Please select a file to print first")
			return
		}
		
		err := app.backend.StartPrint(app.selectedFile)
		if err != nil {
			app.showError("Print Start Error", fmt.Sprintf("Failed to start print: %v", err))
		} else {
			app.showInfo("Print Started", fmt.Sprintf("Started printing %s", app.selectedFile))
		}
	})
	btnStart.Resize(fyne.NewSize(180, 80))
	btnStart.Importance = widget.HighImportance
	
	btnPause := widget.NewButton("Pause", func() {
		err := app.backend.PausePrint()
		if err != nil {
			app.showError("Pause Error", fmt.Sprintf("Failed to pause print: %v", err))
		}
	})
	btnPause.Resize(fyne.NewSize(180, 80))
	btnPause.Importance = widget.MediumImportance
	
	btnResume := widget.NewButton("Resume", func() {
		err := app.backend.ResumePrint()
		if err != nil {
			app.showError("Resume Error", fmt.Sprintf("Failed to resume print: %v", err))
		}
	})
	btnResume.Resize(fyne.NewSize(180, 80))
	btnResume.Importance = widget.HighImportance
	
	btnStop := widget.NewButton("Stop Print", func() {
		err := app.backend.StopPrint()
		if err != nil {
			app.showError("Stop Error", fmt.Sprintf("Failed to stop print: %v", err))
		}
	})
	btnStop.Resize(fyne.NewSize(180, 80))
	btnStop.Importance = widget.DangerImportance
	
	controlRow := container.NewGridWithColumns(2, btnStart, btnPause)
	controlRow2 := container.NewGridWithColumns(2, btnResume, btnStop)
	
	// Manual control with backend integration
	manualMoves := container.NewVBox(
		widget.NewLabel("Manual Control:"),
		container.NewGridWithColumns(3,
			widget.NewButton("↑", func() { app.backend.MoveAxis("Y", 10) }),
			widget.NewButton("Home", func() { app.backend.HomeAxes() }),
			widget.NewButton("↓", func() { app.backend.MoveAxis("Y", -10) }),
		),
		container.NewGridWithColumns(3,
			widget.NewButton("←", func() { app.backend.MoveAxis("X", -10) }),
			widget.NewButton("Z+", func() { app.backend.MoveAxis("Z", 1) }),
			widget.NewButton("→", func() { app.backend.MoveAxis("X", 10) }),
		),
	)
	
	// Temperature control
	tempControl := container.NewVBox(
		widget.NewLabel("Temperature Control:"),
		container.NewHBox(
			widget.NewLabel("Hotend:"),
			widget.NewButton("180°C", func() { app.backend.SetTemperature("hotend", 180) }),
			widget.NewButton("200°C", func() { app.backend.SetTemperature("hotend", 200) }),
			widget.NewButton("220°C", func() { app.backend.SetTemperature("hotend", 220) }),
		),
		container.NewHBox(
			widget.NewLabel("Bed:"),
			widget.NewButton("50°C", func() { app.backend.SetTemperature("bed", 50) }),
			widget.NewButton("60°C", func() { app.backend.SetTemperature("bed", 60) }),
			widget.NewButton("70°C", func() { app.backend.SetTemperature("bed", 70) }),
		),
	)
	
	manualCard := widget.NewCard("Manual Control", "", 
		container.NewVBox(manualMoves, tempControl))
	
	app.mainView = container.NewVBox(
		widget.NewCard("Print Control", "", container.NewVBox(controlRow, controlRow2)),
		manualCard,
	)
	
	app.updateMainContent()
}

func (app *IntegratedApp) showIntegratedFiles() {
	// Refresh print jobs from backend
	app.refreshPrintJobs()
	
	// File list with real data
	fileList := widget.NewList(
		func() int { return len(app.printJobs) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Template"),
				layout.NewSpacer(),
				widget.NewLabel("Status"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(app.printJobs) {
				job := app.printJobs[id]
				container := obj.(*container.Container)
				nameLabel := container.Objects[0].(*widget.Label)
				statusLabel := container.Objects[2].(*widget.Label)
				
				nameLabel.SetText(job.Filename)
				statusLabel.SetText(job.Status)
			}
		},
	)
	
	// File selection
	fileList.OnSelected = func(id widget.ListItemID) {
		if id < len(app.printJobs) {
			app.selectedFile = app.printJobs[id].Filename
		}
	}
	
	// File management buttons
	btnUpload := widget.NewButton("Upload File", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			
			// Read file content
			content := make([]byte, 1024*1024) // Max 1MB
			n, err := reader.Read(content)
			if err != nil && err.Error() != "EOF" {
				app.showError("File Read Error", fmt.Sprintf("Failed to read file: %v", err))
				return
			}
			
			// Upload to backend
			filename := reader.URI().Name()
			err = app.backend.UploadFile(filename, content[:n])
			if err != nil {
				app.showError("Upload Error", fmt.Sprintf("Failed to upload file: %v", err))
			} else {
				app.showInfo("Upload Success", fmt.Sprintf("File %s uploaded successfully", filename))
				app.refreshPrintJobs()
			}
		}, app.window)
	})
	btnUpload.Resize(fyne.NewSize(150, 50))
	
	btnRefresh := widget.NewButton("Refresh", func() {
		app.refreshPrintJobs()
	})
	btnRefresh.Resize(fyne.NewSize(150, 50))
	
	btnDelete := widget.NewButton("Delete Selected", func() {
		if app.selectedFile == "" {
			app.showError("No File Selected", "Please select a file to delete")
			return
		}
		
		dialog.ShowConfirm("Delete File", 
			fmt.Sprintf("Are you sure you want to delete %s?", app.selectedFile),
			func(confirmed bool) {
				if confirmed {
					err := app.backend.DeleteFile(app.selectedFile)
					if err != nil {
						app.showError("Delete Error", fmt.Sprintf("Failed to delete file: %v", err))
					} else {
						app.showInfo("Delete Success", fmt.Sprintf("File %s deleted successfully", app.selectedFile))
						app.selectedFile = ""
						app.refreshPrintJobs()
					}
				}
			}, app.window)
	})
	btnDelete.Resize(fyne.NewSize(150, 50))
	btnDelete.Importance = widget.DangerImportance
	
	buttonRow := container.NewHBox(btnUpload, btnRefresh, btnDelete)
	
	app.mainView = container.NewVBox(
		widget.NewCard("File Manager", "", container.NewVBox(
			buttonRow,
			fileList,
		)),
	)
	
	app.updateMainContent()
}

func (app *IntegratedApp) showIntegratedSettings() {
	// Settings with backend integration
	printerName := widget.NewEntry()
	printerName.SetText("Innovate 3D Printer")
	
	bedTempSlider := widget.NewSlider(0, 100)
	bedTempSlider.SetValue(60)
	bedTempSlider.Step = 5
	bedTempLabel := widget.NewLabel("60°C")
	
	bedTempSlider.OnChanged = func(value float64) {
		bedTempLabel.SetText(fmt.Sprintf("%.0f°C", value))
	}
	
	hotendTempSlider := widget.NewSlider(0, 300)
	hotendTempSlider.SetValue(200)
	hotendTempSlider.Step = 5
	hotendTempLabel := widget.NewLabel("200°C")
	
	hotendTempSlider.OnChanged = func(value float64) {
		hotendTempLabel.SetText(fmt.Sprintf("%.0f°C", value))
	}
	
	settingsForm := container.NewVBox(
		widget.NewLabel("Printer Name:"),
		printerName,
		widget.NewLabel("Default Bed Temperature:"),
		container.NewHBox(bedTempSlider, bedTempLabel),
		widget.NewLabel("Default Hotend Temperature:"),
		container.NewHBox(hotendTempSlider, hotendTempLabel),
		widget.NewButton("Apply Temperatures", func() {
			bedTemp := bedTempSlider.Value
			hotendTemp := hotendTempSlider.Value
			
			err1 := app.backend.SetTemperature("bed", bedTemp)
			err2 := app.backend.SetTemperature("hotend", hotendTemp)
			
			if err1 != nil || err2 != nil {
				app.showError("Temperature Error", "Failed to set temperatures")
			} else {
				app.showInfo("Settings Applied", "Temperature settings applied successfully")
			}
		}),
	)
	
	app.mainView = container.NewVBox(
		widget.NewCard("Settings", "", settingsForm),
	)
	
	app.updateMainContent()
}

func (app *IntegratedApp) handleEmergencyStop() {
	dialog.ShowConfirm("Emergency Stop", 
		"Are you sure you want to perform an emergency stop?",
		func(confirmed bool) {
			if confirmed {
				err := app.backend.EmergencyStop()
				if err != nil {
					app.showError("Emergency Stop Error", fmt.Sprintf("Failed to execute emergency stop: %v", err))
				} else {
					app.showInfo("Emergency Stop", "Emergency stop executed successfully")
				}
			}
		}, app.window)
}

func (app *IntegratedApp) updateMainContent() {
	app.content.Objects[1] = container.NewScroll(app.mainView)
	app.content.Refresh()
}

func (app *IntegratedApp) setupUI() {
	app.sidebar = app.createIntegratedSidebar()
	app.showIntegratedDashboard()
	
	app.content = container.NewBorder(
		nil, nil,
		app.sidebar, nil,
		container.NewScroll(app.mainView),
	)
	
	app.window.SetContent(app.content)
}

func (app *IntegratedApp) run() {
	app.setupUI()
	app.initializeBackend()
	app.window.ShowAndRun()
}

// Alternative main function for integrated version
func mainIntegrated() {
	app := NewIntegratedApp()
	app.run()
} 