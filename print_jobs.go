package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// PrintJob represents a print job
type PrintJob struct {
	ID            uint      `json:"id"`
	Name          string    `json:"name"`
	FileName      string    `json:"file_name"`
	Status        string    `json:"status"`
	Progress      int       `json:"progress"`
	TimeElapsed   int       `json:"time_elapsed"`
	TimeRemaining int       `json:"time_remaining"`
	StartedAt     time.Time `json:"started_at"`
	PrinterID     uint      `json:"printer_id"`
	PrinterName   string    `json:"printer_name"`
}

// GCodeFile represents an uploaded G-code file
type GCodeFile struct {
	ID           uint      `json:"id"`
	Name         string    `json:"name"`
	FileName     string    `json:"file_name"`
	FileSize     int64     `json:"file_size"`
	PrintTime    int       `json:"print_time"`
	FilamentUsed float64   `json:"filament_used"`
	LayerCount   int       `json:"layer_count"`
	UploadedAt   time.Time `json:"uploaded_at"`
}

// PrintJobsUI handles the print job interface
type PrintJobsUI struct {
	app           fyne.App
	window        fyne.Window
	backendURL    string
	authToken     string
	currentPrinter *Printer
	
	// UI elements
	fileList      *widget.List
	jobList       *widget.List
	uploadButton  *widget.Button
	printButton   *widget.Button
	progressBar   *widget.ProgressBar
	statusLabel   *widget.Label
	
	// Data
	gcodeFiles    []GCodeFile
	printJobs     []PrintJob
	selectedFile  *GCodeFile
	currentJob    *PrintJob
}

// NewPrintJobsUI creates a new print jobs interface
func NewPrintJobsUI(app fyne.App, window fyne.Window, backendURL, authToken string, printer *Printer) *PrintJobsUI {
	ui := &PrintJobsUI{
		app:            app,
		window:         window,
		backendURL:     backendURL,
		authToken:      authToken,
		currentPrinter: printer,
		gcodeFiles:     []GCodeFile{},
		printJobs:      []PrintJob{},
	}
	
	return ui
}

// CreateUI creates the print jobs interface
func (ui *PrintJobsUI) CreateUI() fyne.CanvasObject {
	// Header
	title := widget.NewLabelWithStyle(
		fmt.Sprintf("Print Jobs - %s", ui.currentPrinter.Name),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	
	backBtn := widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), func() {
		// Return to printer control
	})
	
	header := container.NewBorder(nil, nil, backBtn, nil, title)
	
	// File management section
	fileSection := ui.createFileSection()
	
	// Active job section
	activeJobSection := ui.createActiveJobSection()
	
	// Job history section
	historySection := ui.createHistorySection()
	
	// Main content with tabs
	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Files", theme.FolderIcon(), fileSection),
		container.NewTabItemWithIcon("Active Job", theme.MediaPlayIcon(), activeJobSection),
		container.NewTabItemWithIcon("History", theme.DocumentIcon(), historySection),
	)
	
	// Status bar
	ui.statusLabel = widget.NewLabel("Ready")
	statusBar := container.NewBorder(
		widget.NewSeparator(), nil, nil, nil,
		container.NewPadded(ui.statusLabel),
	)
	
	// Load initial data
	ui.loadGCodeFiles()
	ui.loadPrintJobs()
	
	// Start status updates
	go ui.startStatusUpdates()
	
	return container.NewBorder(
		header,
		statusBar,
		nil, nil,
		tabs,
	)
}

// createFileSection creates the file management section
func (ui *PrintJobsUI) createFileSection() fyne.CanvasObject {
	// Upload button
	ui.uploadButton = widget.NewButtonWithIcon("Upload G-code", theme.UploadIcon(), func() {
		ui.showUploadDialog()
	})
	ui.uploadButton.Importance = widget.HighImportance
	
	// File list
	ui.fileList = widget.NewList(
		func() int { return len(ui.gcodeFiles) },
		func() fyne.CanvasObject {
			return container.NewBorder(
				nil, nil, nil,
				widget.NewButtonWithIcon("", theme.DeleteIcon(), nil),
				container.NewVBox(
					widget.NewLabel("File name"),
					widget.NewLabel("Details"),
				),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(ui.gcodeFiles) {
				return
			}
			
			file := ui.gcodeFiles[id]
			content := obj.(*fyne.Container).Objects[0].(*fyne.Container)
			labels := content.Objects[0].(*fyne.Container)
			
			// Update labels
			labels.Objects[0].(*widget.Label).SetText(file.Name)
			labels.Objects[1].(*widget.Label).SetText(fmt.Sprintf(
				"%.1f MB | %s | %d layers",
				float64(file.FileSize)/(1024*1024),
				ui.formatDuration(file.PrintTime),
				file.LayerCount,
			))
			
			// Update delete button
			deleteBtn := obj.(*fyne.Container).Objects[1].(*widget.Button)
			deleteBtn.OnTapped = func() {
				ui.deleteFile(&file)
			}
		},
	)
	
	ui.fileList.OnSelected = func(id widget.ListItemID) {
		if id < len(ui.gcodeFiles) {
			ui.selectedFile = &ui.gcodeFiles[id]
			ui.updatePrintButton()
		}
	}
	
	// Print button
	ui.printButton = widget.NewButtonWithIcon("Start Print", theme.MediaPlayIcon(), func() {
		if ui.selectedFile != nil {
			ui.startPrint(ui.selectedFile)
		}
	})
	ui.printButton.Importance = widget.HighImportance
	ui.printButton.Disable()
	
	// File info panel
	fileInfo := widget.NewCard("Selected File", "", widget.NewLabel("No file selected"))
	
	// Layout
	topButtons := container.NewGridWithColumns(2,
		ui.uploadButton,
		ui.printButton,
	)
	
	return container.NewBorder(
		topButtons,
		fileInfo,
		nil, nil,
		ui.fileList,
	)
}

// createActiveJobSection creates the active print job section
func (ui *PrintJobsUI) createActiveJobSection() fyne.CanvasObject {
	// Job info card
	jobInfoCard := widget.NewCard("No Active Print", "", 
		widget.NewLabel("Start a print from the Files tab"),
	)
	
	// Progress section
	ui.progressBar = widget.NewProgressBar()
	progressLabel := widget.NewLabel("0%")
	progressSection := container.NewBorder(
		nil, nil, nil,
		progressLabel,
		ui.progressBar,
	)
	
	// Time info
	elapsedLabel := widget.NewLabel("Elapsed: --:--:--")
	remainingLabel := widget.NewLabel("Remaining: --:--:--")
	timeInfo := container.NewGridWithColumns(2, elapsedLabel, remainingLabel)
	
	// Control buttons
	pauseBtn := widget.NewButtonWithIcon("Pause", theme.MediaPauseIcon(), func() {
		if ui.currentJob != nil {
			ui.pauseJob(ui.currentJob)
		}
	})
	
	cancelBtn := widget.NewButtonWithIcon("Cancel", theme.MediaStopIcon(), func() {
		if ui.currentJob != nil {
			ui.showCancelConfirmation(ui.currentJob)
		}
	})
	cancelBtn.Importance = widget.DangerImportance
	
	controls := container.NewGridWithColumns(2, pauseBtn, cancelBtn)
	
	// G-code viewer
	gcodeViewer := widget.NewMultiLineEntry()
	gcodeViewer.Disable()
	gcodeViewer.SetPlaceHolder("G-code preview will appear here during printing...")
	
	gcodeCard := widget.NewCard("Current G-code", "", 
		container.NewScroll(gcodeViewer),
	)
	
	// Layout
	topSection := container.NewVBox(
		jobInfoCard,
		widget.NewSeparator(),
		progressSection,
		timeInfo,
		controls,
	)
	
	split := container.NewVSplit(topSection, gcodeCard)
	split.SetOffset(0.4)
	
	return container.NewPadded(split)
}

// createHistorySection creates the job history section
func (ui *PrintJobsUI) createHistorySection() fyne.CanvasObject {
	// History list
	ui.jobList = widget.NewList(
		func() int { return len(ui.printJobs) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.DocumentIcon()),
				container.NewVBox(
					widget.NewLabel("Job name"),
					widget.NewLabel("Details"),
				),
				layout.NewSpacer(),
				widget.NewLabel("Status"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(ui.printJobs) {
				return
			}
			
			job := ui.printJobs[id]
			hbox := obj.(*fyne.Container)
			
			// Update icon based on status
			icon := hbox.Objects[0].(*widget.Icon)
			switch job.Status {
			case "completed":
				icon.SetResource(theme.ConfirmIcon())
			case "failed":
				icon.SetResource(theme.ErrorIcon())
			case "cancelled":
				icon.SetResource(theme.CancelIcon())
			default:
				icon.SetResource(theme.DocumentIcon())
			}
			
			// Update labels
			info := hbox.Objects[1].(*fyne.Container)
			info.Objects[0].(*widget.Label).SetText(job.Name)
			info.Objects[1].(*widget.Label).SetText(fmt.Sprintf(
				"Started: %s | Duration: %s",
				job.StartedAt.Format("Jan 2 15:04"),
				ui.formatDuration(job.TimeElapsed),
			))
			
			// Update status
			statusLabel := hbox.Objects[3].(*widget.Label)
			statusLabel.SetText(strings.Title(job.Status))
		},
	)
	
	// Stats card
	statsCard := widget.NewCard("Statistics", "",
		container.NewVBox(
			widget.NewLabel("Total prints: 0"),
			widget.NewLabel("Success rate: --%"),
			widget.NewLabel("Total print time: --:--:--"),
		),
	)
	
	// Clear history button
	clearBtn := widget.NewButtonWithIcon("Clear History", theme.DeleteIcon(), func() {
		dialog.ShowConfirm("Clear History", 
			"Are you sure you want to clear the print history?",
			func(ok bool) {
				if ok {
					ui.clearHistory()
				}
			},
			ui.window,
		)
	})
	
	// Layout
	return container.NewBorder(
		statsCard,
		container.NewPadded(clearBtn),
		nil, nil,
		ui.jobList,
	)
}

// showUploadDialog shows the file upload dialog
func (ui *PrintJobsUI) showUploadDialog() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()
		
		// Check file extension
		if !strings.HasSuffix(strings.ToLower(reader.URI().Name()), ".gcode") &&
		   !strings.HasSuffix(strings.ToLower(reader.URI().Name()), ".gco") {
			dialog.ShowError(fmt.Errorf("Please select a G-code file (.gcode or .gco)"), ui.window)
			return
		}
		
		// Show upload progress
		progress := dialog.NewProgressInfinite("Uploading file...", "Please wait")
		progress.Show()
		
		// Upload file
		go func() {
			err := ui.uploadGCodeFile(reader)
			progress.Hide()
			
			if err != nil {
				dialog.ShowError(err, ui.window)
			} else {
				ui.statusLabel.SetText("File uploaded successfully")
				ui.loadGCodeFiles()
			}
		}()
		
	}, ui.window)
}

// uploadGCodeFile uploads a G-code file to the backend
func (ui *PrintJobsUI) uploadGCodeFile(reader fyne.URIReadCloser) error {
	// TODO: Implement actual file upload to backend
	// For now, simulate upload
	time.Sleep(2 * time.Second)
	
	// Add to list (temporary simulation)
	ui.gcodeFiles = append(ui.gcodeFiles, GCodeFile{
		ID:           uint(len(ui.gcodeFiles) + 1),
		Name:         filepath.Base(reader.URI().Name()),
		FileName:     reader.URI().Name(),
		FileSize:     1024 * 1024 * 5, // 5MB dummy
		PrintTime:    7200,             // 2 hours dummy
		FilamentUsed: 12.5,
		LayerCount:   150,
		UploadedAt:   time.Now(),
	})
	
	return nil
}

// Other helper methods...
func (ui *PrintJobsUI) formatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm %ds", minutes, secs)
}

func (ui *PrintJobsUI) updatePrintButton() {
	if ui.selectedFile != nil && ui.currentJob == nil {
		ui.printButton.Enable()
	} else {
		ui.printButton.Disable()
	}
}

func (ui *PrintJobsUI) loadGCodeFiles() {
	// TODO: Load from backend API
	ui.fileList.Refresh()
}

func (ui *PrintJobsUI) loadPrintJobs() {
	// TODO: Load from backend API
	ui.jobList.Refresh()
}

func (ui *PrintJobsUI) startPrint(file *GCodeFile) {
	// TODO: Send start print command to backend
	ui.statusLabel.SetText(fmt.Sprintf("Starting print: %s", file.Name))
}

func (ui *PrintJobsUI) pauseJob(job *PrintJob) {
	// TODO: Send pause command to backend
}

func (ui *PrintJobsUI) showCancelConfirmation(job *PrintJob) {
	dialog.ShowConfirm("Cancel Print",
		fmt.Sprintf("Are you sure you want to cancel '%s'?", job.Name),
		func(ok bool) {
			if ok {
				ui.cancelJob(job)
			}
		},
		ui.window,
	)
}

func (ui *PrintJobsUI) cancelJob(job *PrintJob) {
	// TODO: Send cancel command to backend
}

func (ui *PrintJobsUI) deleteFile(file *GCodeFile) {
	dialog.ShowConfirm("Delete File",
		fmt.Sprintf("Are you sure you want to delete '%s'?", file.Name),
		func(ok bool) {
			if ok {
				// TODO: Delete file via API
				ui.loadGCodeFiles()
			}
		},
		ui.window,
	)
}

func (ui *PrintJobsUI) clearHistory() {
	// TODO: Clear history via API
}

func (ui *PrintJobsUI) startStatusUpdates() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		// TODO: Get current job status from backend
		// Update UI accordingly
	}
} 