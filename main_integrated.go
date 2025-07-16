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
	
	// Authentication
	authManager   *AuthManager
	loginUI       *LoginUI
	profileUI     *UserProfileUI
	tokenHandler  *TokenExpiredHandler
	
	// Backend integration
	backend       *BackendClient
	statusChan    chan PrinterStatus
	
	// Connection status
	connectionStatus *fyne.Container
	
	// Temperature monitoring
	temperatureUI *TemperatureUI
	
	// G-code viewer
	gcodeViewerUI *GCodeViewerUI
	
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
	isAuthenticated bool
}

func NewIntegratedApp() *IntegratedApp {
	a := app.New()
	a.Settings().SetTheme(&InnovateTheme{})
	
	w := a.NewWindow("Innovate OS - 3D Printer Control")
	w.Resize(fyne.NewSize(1024, 600))
	w.SetFullScreen(true)
	
	// Initialize authentication
	authManager := NewAuthManager("localhost:8080")
	
	// Initialize backend client
	backend := NewBackendClient("localhost:8080")
	
	app := &IntegratedApp{
		app:        a,
		window:     w,
		authManager: authManager,
		backend:    backend,
		statusChan: make(chan PrinterStatus, 100),
		isAuthenticated: authManager.IsAuthenticated(),
	}
	
	// Create auth UI components
	app.loginUI = NewLoginUI(w, authManager)
	app.loginUI.SetLoginSuccessCallback(func() {
		app.isAuthenticated = true
		app.updateAuthToken()
		app.setupUI()
		app.initializeBackend()
	})
	
	app.profileUI = NewUserProfileUI(w, authManager)
	app.profileUI.SetLogoutCallback(func() {
		app.isAuthenticated = false
		app.backend.CloseWebSocket()
		app.showLoginScreen()
	})
	
	app.tokenHandler = NewTokenExpiredHandler(w, authManager)
	app.tokenHandler.onReauth = func() {
		app.showLoginScreen()
	}
	
	// Set auth change callback
	authManager.SetAuthChangeCallback(func(authenticated bool) {
		app.isAuthenticated = authenticated
		if authenticated {
			app.updateAuthToken()
		}
	})
	
	return app
}

func (app *IntegratedApp) updateAuthToken() {
	// Update backend client with new token
	token := app.authManager.GetToken()
	app.backend.SetAuthToken(token)
}

func (app *IntegratedApp) showLoginScreen() {
	app.window.SetContent(app.loginUI.GetContent())
}

func (app *IntegratedApp) initializeBackend() {
	// Only initialize if authenticated
	if !app.isAuthenticated {
		return
	}
	
	// Update log
	if app.logEntry != nil {
		app.logEntry.SetText(app.logEntry.Text + "\nConnecting to backend WebSocket...")
	}
	
	// Connect to WebSocket for real-time updates
	err := app.backend.ConnectWebSocket()
	if err != nil {
		log.Printf("Failed to connect to WebSocket: %v", err)
		// Check if it's an auth error
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "unauthorized") {
			app.tokenHandler.HandleTokenExpired()
			return
		}
		if app.logEntry != nil {
			app.logEntry.SetText(app.logEntry.Text + fmt.Sprintf("\nWebSocket connection failed: %v", err))
		}
		// Don't show error dialog - the connection manager will handle reconnection
		return
	}
	
	if app.logEntry != nil {
		app.logEntry.SetText(app.logEntry.Text + "\nWebSocket connected successfully!")
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
		
		// Update temperature chart if available
		if app.temperatureUI != nil {
			// Temperature data is automatically updated via the TemperatureUI's own ticker
			// But we can also manually sync here if needed
		}
		
		// Sync G-code viewer with print progress if available
		if app.gcodeViewerUI != nil && status.CurrentLayer > 0 {
			// Estimate current line based on layer progress
			// This is a simplified approach - real implementation would need actual line tracking
			app.gcodeViewerUI.SyncWithPrintProgress(int(status.Progress * 1000))
		}
	}
}

func (app *IntegratedApp) updateUI() {
	if app.tempLabel != nil {
		tempData := fmt.Sprintf("Hotend: %.1f°C | Bed: %.1f°C", 
			app.currentStatus.Temperature, app.currentStatus.BedTemp)
		app.tempLabel.SetText(tempData)
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
		app.statusLabel.SetText(app.currentStatus.Status)
	}
	
	// Update log with important events
	if app.logEntry != nil {
		currentText := app.logEntry.Text
		
		// Add significant temperature changes
		if app.currentStatus.Temperature > 0 {
			lastLine := ""
			lines := strings.Split(currentText, "\n")
			if len(lines) > 0 {
				lastLine = lines[len(lines)-1]
			}
			
			// Only log if temperature changed significantly or status changed
			if !strings.Contains(lastLine, fmt.Sprintf("%.0f°C", app.currentStatus.Temperature)) {
				timestamp := time.Now().Format("15:04:05")
				logEntry := fmt.Sprintf("\n[%s] Temp: Hotend %.1f°C, Bed %.1f°C - %s", 
					timestamp, app.currentStatus.Temperature, app.currentStatus.BedTemp, app.currentStatus.Status)
				app.logEntry.SetText(currentText + logEntry)
			}
		}
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

func (app *IntegratedApp) createSidebar() *container.VBox {
	// Add profile button at the top
	profileButton := widget.NewButton("Profile", func() {
		app.showProfile()
	})
	profileButton.Resize(fyne.NewSize(200, 60))
	
	// Show user info if authenticated
	var userInfo fyne.CanvasObject
	if user := app.authManager.GetUser(); user != nil {
		userLabel := widget.NewLabel(fmt.Sprintf("👤 %s", user.Username))
		userLabel.Alignment = fyne.TextAlignCenter
		userInfo = userLabel
	} else {
		userInfo = widget.NewLabel("Not logged in")
	}
	
	// Create compact connection status indicator
	app.connectionStatus = CreateCompactStatusIndicator(app.backend)
	
	// Create navigation buttons with touch-optimized sizing
	btnDashboard := widget.NewButton("Dashboard", func() {
		app.showDashboard()
	})
	btnDashboard.Resize(fyne.NewSize(200, 60))
	btnDashboard.Importance = widget.HighImportance
	
	btnPrint := widget.NewButton("Print Control", func() {
		app.showPrintControl()
	})
	btnPrint.Resize(fyne.NewSize(200, 60))
	
	btnTemperature := widget.NewButton("Temperature", func() {
		app.showTemperature()
	})
	btnTemperature.Resize(fyne.NewSize(200, 60))
	
	btnGCodeViewer := widget.NewButton("G-Code Viewer", func() {
		app.showGCodeViewer()
	})
	btnGCodeViewer.Resize(fyne.NewSize(200, 60))
	
	btnFiles := widget.NewButton("Files", func() {
		app.showFiles()
	})
	btnFiles.Resize(fyne.NewSize(200, 60))
	
	btnSettings := widget.NewButton("Settings", func() {
		app.showSettings()
	})
	btnSettings.Resize(fyne.NewSize(200, 60))
	
	// Printer discovery button
	btnPrinterDiscovery := widget.NewButton("Printer Discovery", func() {
		app.showPrinterDiscovery()
	})
	btnPrinterDiscovery.Resize(fyne.NewSize(200, 60))
	
	// Print jobs button  
	btnPrintJobs := widget.NewButton("Print Jobs", func() {
		app.showPrintJobs()
	})
	btnPrintJobs.Resize(fyne.NewSize(200, 60))
	
	// Emergency stop button
	btnEmergencyStop := widget.NewButton("EMERGENCY STOP", func() {
		app.emergencyStop()
	})
	btnEmergencyStop.Resize(fyne.NewSize(200, 80))
	btnEmergencyStop.Importance = widget.DangerImportance
	
	// Create sidebar with proper spacing
	sidebar := container.NewVBox(
		widget.NewCard("", "", container.NewVBox(
			canvas.NewText("Innovate OS", color.NRGBA{R: 28, G: 28, B: 30, A: 255}),
			userInfo,
			profileButton,
			widget.NewSeparator(),
			app.connectionStatus,
		)),
		widget.NewSeparator(),
		btnDashboard,
		btnPrint,
		btnTemperature,
		btnGCodeViewer,
		btnFiles,
		btnPrintJobs,
		btnPrinterDiscovery,
		btnSettings,
		layout.NewSpacer(),
		btnEmergencyStop,
	)
	
	return sidebar
}

func (app *IntegratedApp) showProfile() {
	app.profileUI.Refresh()
	app.mainView = container.NewVBox(
		widget.NewCard("User Profile", "", app.profileUI.GetContent()),
	)
	app.updateMainContent()
}

func (app *IntegratedApp) showDashboard() {
	// Create connection status card
	connectionCard := NewConnectionStatusCard(app.backend)
	
	// Real-time temperature card with mini chart
	tempData := ""
	if app.temperatureUI != nil && app.temperatureUI.GetChart() != nil {
		current := app.temperatureUI.GetChart().GetCurrentTemperatures()
		if current != nil {
			tempData = fmt.Sprintf("Hotend: %.1f°C | Bed: %.1f°C", 
				current.HotendActual, current.BedActual)
		}
	}
	if tempData == "" {
		tempData = fmt.Sprintf("Hotend: %.1f°C | Bed: %.1f°C", 
			app.currentStatus.Temperature, app.currentStatus.BedTemp)
	}
	
	app.tempLabel = widget.NewLabel(tempData)
	tempProgressBar := widget.NewProgressBar()
	tempProgressBar.SetValue(app.currentStatus.Temperature / 250.0) // Scale to 250°C max
	
	tempCard := widget.NewCard("Temperature", "", 
		container.NewVBox(
			app.tempLabel, 
			tempProgressBar,
			widget.NewButton("View Chart", func() {
				app.showTemperature()
			}),
		))
	
	// Progress card
	app.progressLabel = widget.NewLabel(fmt.Sprintf("Layer %d/%d", 
		app.currentStatus.CurrentLayer, app.currentStatus.TotalLayers))
	app.progressBar = widget.NewProgressBar()
	app.progressBar.SetValue(app.currentStatus.Progress)
	progressCard := widget.NewCard("Print Progress", "", 
		container.NewVBox(app.progressLabel, app.progressBar))
	
	// Position card
	app.positionLabel = widget.NewLabel(fmt.Sprintf("X: %.1f | Y: %.1f | Z: %.1f", 
		app.currentStatus.PositionX, app.currentStatus.PositionY, app.currentStatus.PositionZ))
	app.statusLabel = widget.NewLabel(app.currentStatus.Status)
	positionCard := widget.NewCard("Position", "", 
		container.NewVBox(app.positionLabel, app.statusLabel))
	
	// Create dashboard layout with connection status at the top
	topRow := container.NewGridWithColumns(3, tempCard, progressCard, positionCard)
	
	// Real-time log
	app.logEntry = widget.NewEntry()
	app.logEntry.MultiLine = true
	app.logEntry.SetText("System initialized...\nWaiting for printer connection...")
	logCard := widget.NewCard("System Log", "", 
		container.NewMax(app.logEntry))
	
	app.mainView = container.NewVBox(
		connectionCard.GetCard(),
		topRow,
		logCard,
	)
	
	app.updateMainContent()
}

func (app *IntegratedApp) showTemperature() {
	// Initialize temperature UI if not already done
	if app.temperatureUI == nil {
		app.temperatureUI = NewTemperatureUI(app.window, app.backend)
	}
	
	app.mainView = container.NewVBox(
		app.temperatureUI.GetContent(),
	)
	
	app.updateMainContent()
}

func (app *IntegratedApp) showPrintControl() {
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

func (app *IntegratedApp) showFiles() {
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

func (app *IntegratedApp) showSettings() {
	// Settings with backend integration
	printerName := widget.NewEntry()
	printerName.SetText("Innovate 3D Printer")
	
	// Printer Discovery Button
	btnDiscoverPrinters := widget.NewButtonWithIcon("Discover Printers", theme.SearchIcon(), func() {
		discoveryUI := NewPrinterDiscoveryUI(app.app, app.backend)
		discoveryUI.SetOnConnect(func(printer DiscoveredPrinter) {
			// Update printer name from discovery
			if printer.Name != "" {
				printerName.SetText(printer.Name)
			}
			// Refresh status after connection
			app.refreshStatus()
		})
		discoveryUI.Show()
	})
	btnDiscoverPrinters.Importance = widget.HighImportance
	
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
	
	// Printer connection info card
	connectionInfo := widget.NewCard("Printer Connection", "", container.NewVBox(
		widget.NewLabel("Printer Name:"),
		printerName,
		btnDiscoverPrinters,
		widget.NewSeparator(),
	))
	
	// Temperature settings card
	temperatureSettings := widget.NewCard("Default Temperatures", "", container.NewVBox(
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
	))
	
	app.mainView = container.NewVBox(
		connectionInfo,
		temperatureSettings,
	)
	
	app.updateMainContent()
}

func (app *IntegratedApp) showPrinterDiscovery() {
	discoveryUI := NewPrinterDiscoveryUI(app.app, app.backend)
	discoveryUI.SetOnConnect(func(printer DiscoveredPrinter) {
		// Refresh status after connection
		app.refreshStatus()
	})
	discoveryUI.Show()
}

func (app *IntegratedApp) showPrintJobs() {
	// Refresh print jobs from backend
	app.refreshPrintJobs()
	
	// Print jobs list with real data
	jobList := widget.NewList(
		func() int { return len(app.printJobs) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Filename"),
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
	
	// Job selection
	jobList.OnSelected = func(id widget.ListItemID) {
		if id < len(app.printJobs) {
			job := app.printJobs[id]
			dialog.ShowInformation("Job Details", 
				fmt.Sprintf("Filename: %s\nStatus: %s\nProgress: %.1f%%\nLayer: %d/%d\nPosition: X=%.1f, Y=%.1f, Z=%.1f",
					job.Filename, job.Status, job.Progress*100, job.CurrentLayer, job.TotalLayers, job.PositionX, job.PositionY, job.PositionZ),
				app.window)
		}
	}
	
	// Job management buttons
	btnCancel := widget.NewButton("Cancel Job", func() {
		if app.selectedFile == "" {
			app.showError("No File Selected", "Please select a file to cancel")
			return
		}
		
		dialog.ShowConfirm("Cancel Job", 
			fmt.Sprintf("Are you sure you want to cancel the print job for %s?", app.selectedFile),
			func(confirmed bool) {
				if confirmed {
					err := app.backend.CancelPrintJob(app.selectedFile)
					if err != nil {
						app.showError("Cancel Error", fmt.Sprintf("Failed to cancel job: %v", err))
					} else {
						app.showInfo("Job Cancelled", fmt.Sprintf("Print job for %s cancelled", app.selectedFile))
						app.selectedFile = ""
						app.refreshPrintJobs()
					}
				}
			}, app.window)
	})
	btnCancel.Resize(fyne.NewSize(180, 80))
	btnCancel.Importance = widget.DangerImportance
	
	btnDelete := widget.NewButton("Delete Job", func() {
		if app.selectedFile == "" {
			app.showError("No File Selected", "Please select a file to delete")
			return
		}
		
		dialog.ShowConfirm("Delete Job", 
			fmt.Sprintf("Are you sure you want to delete the print job for %s?", app.selectedFile),
			func(confirmed bool) {
				if confirmed {
					err := app.backend.DeletePrintJob(app.selectedFile)
					if err != nil {
						app.showError("Delete Error", fmt.Sprintf("Failed to delete job: %v", err))
					} else {
						app.showInfo("Job Deleted", fmt.Sprintf("Print job for %s deleted", app.selectedFile))
						app.selectedFile = ""
						app.refreshPrintJobs()
					}
				}
			}, app.window)
	})
	btnDelete.Resize(fyne.NewSize(180, 80))
	btnDelete.Importance = widget.DangerImportance
	
	buttonRow := container.NewHBox(btnCancel, btnDelete)
	
	app.mainView = container.NewVBox(
		widget.NewCard("Print Jobs", "", container.NewVBox(
			buttonRow,
			jobList,
		)),
	)
	
	app.updateMainContent()
}

func (app *IntegratedApp) emergencyStop() {
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

func (app *IntegratedApp) showGCodeViewer() {
	// Initialize G-code viewer UI if not already done
	if app.gcodeViewerUI == nil {
		app.gcodeViewerUI = NewGCodeViewerUI(app.window, app.backend)
	}
	
	app.mainView = container.NewVBox(
		app.gcodeViewerUI.GetContent(),
	)
	
	app.updateMainContent()
}

func (app *IntegratedApp) updateMainContent() {
	app.content.Objects[1] = container.NewScroll(app.mainView)
	app.content.Refresh()
}

func (app *IntegratedApp) setupUI() {
	app.sidebar = app.createSidebar()
	app.showDashboard() // Show dashboard by default
	
	// Create main layout with sidebar
	app.content = container.NewBorder(
		nil, nil, // top, bottom
		app.sidebar, nil, // left, right
		container.NewScroll(app.mainView), // center
	)
	
	app.window.SetContent(app.content)
}

func (app *IntegratedApp) run() {
	// Check if already authenticated
	if app.isAuthenticated {
		app.setupUI()
		app.initializeBackend()
	} else {
		app.showLoginScreen()
	}
	
	app.window.ShowAndRun()
	
	// Cleanup on exit
	if app.temperatureUI != nil {
		app.temperatureUI.Stop()
	}
	if app.gcodeViewerUI != nil {
		app.gcodeViewerUI.Stop()
	}
}

// Alternative main function for integrated version
func mainIntegrated() {
	app := NewIntegratedApp()
	app.run()
} 