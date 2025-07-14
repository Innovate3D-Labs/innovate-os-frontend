package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/layout"
	"image/color"
	"fyne.io/fyne/v2/canvas"
	"log"
)

// InnovateTheme provides a clean, Apple-inspired theme for the touchscreen interface
type InnovateTheme struct{}

func (t InnovateTheme) Color(name theme.ColorName, variant theme.Variant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 248, G: 248, B: 248, A: 255} // Light gray background
	case theme.ColorNameForeground:
		return color.NRGBA{R: 28, G: 28, B: 30, A: 255} // Dark text
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0, G: 122, B: 255, A: 255} // iOS Blue
	case theme.ColorNameButton:
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255} // White buttons
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 174, G: 174, B: 178, A: 255} // Light gray
	case theme.ColorNameError:
		return color.NRGBA{R: 255, G: 69, B: 58, A: 255} // iOS Red
	case theme.ColorNameSuccess:
		return color.NRGBA{R: 52, G: 199, B: 89, A: 255} // iOS Green
	case theme.ColorNameWarning:
		return color.NRGBA{R: 255, G: 149, B: 0, A: 255} // iOS Orange
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

func (t InnovateTheme) Font(style theme.TextStyle) *theme.FontResource {
	return theme.DefaultTheme().Font(style)
}

func (t InnovateTheme) Icon(name theme.IconName) *theme.ThemedResource {
	return theme.DefaultTheme().Icon(name)
}

func (t InnovateTheme) Size(name theme.SizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 16 // Larger text for touch screens
	case theme.SizeNameCaptionText:
		return 14
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameSubHeadingText:
		return 18
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 12
	case theme.SizeNameScrollBar:
		return 24 // Larger scroll bars for touch
	case theme.SizeNameSeparator:
		return 2
	default:
		return theme.DefaultTheme().Size(name) * 1.2 // Make everything 20% larger for touch
	}
}

// MainApp represents the main application structure
type MainApp struct {
	app      app.App
	window   fyne.Window
	content  *container.Border
	sidebar  *container.VBox
	mainView *container.VBox
}

func NewMainApp() *MainApp {
	a := app.New()
	a.Settings().SetTheme(&InnovateTheme{})
	
	w := a.NewWindow("Innovate OS - 3D Printer Control")
	w.Resize(fyne.NewSize(1024, 600)) // 10-inch screen resolution
	w.SetFullScreen(true)
	
	return &MainApp{
		app:    a,
		window: w,
	}
}

func (m *MainApp) createSidebar() *container.VBox {
	// Create navigation buttons with touch-optimized sizing
	btnDashboard := widget.NewButton("Dashboard", func() {
		m.showDashboard()
	})
	btnDashboard.Resize(fyne.NewSize(200, 60))
	btnDashboard.Importance = widget.HighImportance
	
	btnPrint := widget.NewButton("Print Control", func() {
		m.showPrintControl()
	})
	btnPrint.Resize(fyne.NewSize(200, 60))
	
	btnFiles := widget.NewButton("Files", func() {
		m.showFiles()
	})
	btnFiles.Resize(fyne.NewSize(200, 60))
	
	btnSettings := widget.NewButton("Settings", func() {
		m.showSettings()
	})
	btnSettings.Resize(fyne.NewSize(200, 60))
	
	// Emergency stop button
	btnEmergencyStop := widget.NewButton("EMERGENCY STOP", func() {
		m.emergencyStop()
	})
	btnEmergencyStop.Resize(fyne.NewSize(200, 80))
	btnEmergencyStop.Importance = widget.DangerImportance
	
	// Create sidebar with proper spacing
	sidebar := container.NewVBox(
		widget.NewCard("", "", canvas.NewText("Innovate OS", color.NRGBA{R: 28, G: 28, B: 30, A: 255})),
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

func (m *MainApp) showDashboard() {
	// Status cards for temperature, progress, etc.
	tempCard := widget.NewCard("Temperature", "Hotend: 200°C | Bed: 60°C", 
		widget.NewProgressBar())
	
	progressCard := widget.NewCard("Print Progress", "Layer 45/200", 
		widget.NewProgressBar())
	progressBar := progressCard.Objects[1].(*widget.ProgressBar)
	progressBar.SetValue(0.225)
	
	positionCard := widget.NewCard("Position", "X: 125.5 | Y: 87.2 | Z: 12.3", 
		widget.NewLabel("Printing..."))
	
	// Create main dashboard layout
	topRow := container.NewGridWithColumns(3, tempCard, progressCard, positionCard)
	
	// Real-time log
	logCard := widget.NewCard("System Log", "", 
		widget.NewEntry())
	logEntry := logCard.Objects[1].(*widget.Entry)
	logEntry.MultiLine = true
	logEntry.SetText("System initialized...\nHeating bed to 60°C...\nPrint started successfully...")
	
	m.mainView = container.NewVBox(
		topRow,
		logCard,
	)
	
	m.updateMainContent()
}

func (m *MainApp) showPrintControl() {
	// Large touch-optimized control buttons
	btnStart := widget.NewButton("Start Print", func() {
		log.Println("Starting print...")
	})
	btnStart.Resize(fyne.NewSize(180, 80))
	btnStart.Importance = widget.HighImportance
	
	btnPause := widget.NewButton("Pause", func() {
		log.Println("Pausing print...")
	})
	btnPause.Resize(fyne.NewSize(180, 80))
	btnPause.Importance = widget.MediumImportance
	
	btnResume := widget.NewButton("Resume", func() {
		log.Println("Resuming print...")
	})
	btnResume.Resize(fyne.NewSize(180, 80))
	btnResume.Importance = widget.HighImportance
	
	btnStop := widget.NewButton("Stop Print", func() {
		log.Println("Stopping print...")
	})
	btnStop.Resize(fyne.NewSize(180, 80))
	btnStop.Importance = widget.DangerImportance
	
	// Control buttons layout
	controlRow := container.NewGridWithColumns(2, btnStart, btnPause)
	controlRow2 := container.NewGridWithColumns(2, btnResume, btnStop)
	
	// Manual control section
	manualCard := widget.NewCard("Manual Control", "", 
		container.NewVBox(
			widget.NewLabel("Move Extruder:"),
			container.NewGridWithColumns(3,
				widget.NewButton("↑", func() { log.Println("Move Y+") }),
				widget.NewButton("Home", func() { log.Println("Home all") }),
				widget.NewButton("↓", func() { log.Println("Move Y-") }),
			),
			container.NewGridWithColumns(3,
				widget.NewButton("←", func() { log.Println("Move X-") }),
				widget.NewButton("Z+", func() { log.Println("Move Z+") }),
				widget.NewButton("→", func() { log.Println("Move X+") }),
			),
		))
	
	m.mainView = container.NewVBox(
		widget.NewCard("Print Control", "", container.NewVBox(controlRow, controlRow2)),
		manualCard,
	)
	
	m.updateMainContent()
}

func (m *MainApp) showFiles() {
	// File list with touch-optimized list
	fileList := widget.NewList(
		func() int { return 5 }, // Number of files
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			files := []string{
				"sample_model.gcode",
				"phone_case.gcode", 
				"miniature.gcode",
				"bracket.gcode",
				"test_print.gcode",
			}
			obj.(*widget.Label).SetText(files[id])
		},
	)
	
	// File management buttons
	btnUpload := widget.NewButton("Upload File", func() {
		log.Println("Opening file dialog...")
	})
	btnUpload.Resize(fyne.NewSize(150, 50))
	
	btnDownload := widget.NewButton("Download from Cloud", func() {
		log.Println("Downloading from cloud...")
	})
	btnDownload.Resize(fyne.NewSize(150, 50))
	
	btnDelete := widget.NewButton("Delete Selected", func() {
		log.Println("Deleting file...")
	})
	btnDelete.Resize(fyne.NewSize(150, 50))
	btnDelete.Importance = widget.DangerImportance
	
	buttonRow := container.NewHBox(btnUpload, btnDownload, btnDelete)
	
	m.mainView = container.NewVBox(
		widget.NewCard("File Manager", "", container.NewVBox(
			buttonRow,
			fileList,
		)),
	)
	
	m.updateMainContent()
}

func (m *MainApp) showSettings() {
	// Settings form with touch-optimized inputs
	printerName := widget.NewEntry()
	printerName.SetText("Innovate 3D Printer")
	
	bedTemp := widget.NewSlider(0, 100)
	bedTemp.SetValue(60)
	bedTemp.Step = 5
	
	hotendTemp := widget.NewSlider(0, 300)
	hotendTemp.SetValue(200)
	hotendTemp.Step = 5
	
	printSpeed := widget.NewSlider(10, 200)
	printSpeed.SetValue(100)
	printSpeed.Step = 10
	
	settingsForm := container.NewVBox(
		widget.NewLabel("Printer Name:"),
		printerName,
		widget.NewLabel("Bed Temperature:"),
		bedTemp,
		widget.NewLabel("Hotend Temperature:"),
		hotendTemp,
		widget.NewLabel("Print Speed (%):"),
		printSpeed,
		widget.NewButton("Save Settings", func() {
			log.Println("Settings saved")
		}),
	)
	
	m.mainView = container.NewVBox(
		widget.NewCard("Settings", "", settingsForm),
	)
	
	m.updateMainContent()
}

func (m *MainApp) emergencyStop() {
	log.Println("EMERGENCY STOP ACTIVATED!")
	// TODO: Implement emergency stop functionality
}

func (m *MainApp) updateMainContent() {
	m.content.Objects[1] = container.NewScroll(m.mainView)
	m.content.Refresh()
}

func (m *MainApp) setupUI() {
	m.sidebar = m.createSidebar()
	m.showDashboard() // Show dashboard by default
	
	// Create main layout with sidebar
	m.content = container.NewBorder(
		nil, nil, // top, bottom
		m.sidebar, nil, // left, right
		container.NewScroll(m.mainView), // center
	)
	
	m.window.SetContent(m.content)
}

func (m *MainApp) run() {
	m.setupUI()
	m.window.ShowAndRun()
}

func main() {
	// Use integrated version with backend connection
	app := NewIntegratedApp()
	app.run()
} 