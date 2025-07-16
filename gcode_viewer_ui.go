package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// GCodeViewerUI manages the G-code viewer interface
type GCodeViewerUI struct {
	window     fyne.Window
	backend    *BackendClient
	
	// Viewer
	viewer     *GCodeViewer
	model      *GCodeModel
	
	// File management
	currentFile      string
	loadedFiles      []string
	
	// Layer controls
	layerSlider      *widget.Slider
	layerLabel       *widget.Label
	showAllBtn       *widget.Button
	showCurrentBtn   *widget.Button
	
	// Progress controls
	progressSlider   *widget.Slider
	progressLabel    *widget.Label
	playBtn          *widget.Button
	pauseBtn         *widget.Button
	resetBtn         *widget.Button
	speedSlider      *widget.Slider
	
	// Display options
	travelMovesCheck *widget.Check
	supportsCheck    *widget.Check
	fullscreenBtn    *widget.Button
	resetViewBtn     *widget.Button
	
	// File controls
	fileSelect       *widget.Select
	loadBtn          *widget.Button
	reloadBtn        *widget.Button
	
	// Information display
	metadataCard     *widget.Card
	layerInfoCard    *widget.Card
	
	// Animation
	animationTicker  *time.Ticker
	isPlaying        bool
	playbackSpeed    float64
	
	// Content
	content          *fyne.Container
}

// NewGCodeViewerUI creates a new G-code viewer interface
func NewGCodeViewerUI(window fyne.Window, backend *BackendClient) *GCodeViewerUI {
	ui := &GCodeViewerUI{
		window:      window,
		backend:     backend,
		viewer:      NewGCodeViewer(),
		loadedFiles: make([]string, 0),
		playbackSpeed: 1.0,
	}
	
	ui.createControls()
	ui.createLayout()
	ui.setupInteractions()
	
	return ui
}

// createControls creates all UI controls
func (ui *GCodeViewerUI) createControls() {
	// Layer controls
	ui.layerSlider = widget.NewSlider(0, 1)
	ui.layerSlider.Step = 1
	ui.layerSlider.OnChanged = func(value float64) {
		ui.setCurrentLayer(int(value))
	}
	
	ui.layerLabel = widget.NewLabel("Layer: 0/0")
	
	ui.showAllBtn = widget.NewButton("Show All", func() {
		ui.viewer.SetVisibleLayers(ui.getAllLayerIndices())
		ui.viewer.Refresh()
	})
	ui.showAllBtn.Resize(fyne.NewSize(100, 40))
	
	ui.showCurrentBtn = widget.NewButton("Show Current", func() {
		current := ui.viewer.currentLayer
		ui.viewer.ShowLayersUpTo(current)
	})
	ui.showCurrentBtn.Resize(fyne.NewSize(100, 40))
	
	// Progress controls
	ui.progressSlider = widget.NewSlider(0, 1)
	ui.progressSlider.OnChanged = func(value float64) {
		ui.setProgress(value)
	}
	
	ui.progressLabel = widget.NewLabel("Progress: 0%")
	
	ui.playBtn = widget.NewButton("▶", func() {
		ui.startAnimation()
	})
	ui.playBtn.Resize(fyne.NewSize(50, 40))
	
	ui.pauseBtn = widget.NewButton("⏸", func() {
		ui.pauseAnimation()
	})
	ui.pauseBtn.Resize(fyne.NewSize(50, 40))
	ui.pauseBtn.Hide()
	
	ui.resetBtn = widget.NewButton("⏹", func() {
		ui.resetAnimation()
	})
	ui.resetBtn.Resize(fyne.NewSize(50, 40))
	
	ui.speedSlider = widget.NewSlider(0.1, 5.0)
	ui.speedSlider.SetValue(1.0)
	ui.speedSlider.OnChanged = func(value float64) {
		ui.playbackSpeed = value
	}
	
	// Display options
	ui.travelMovesCheck = widget.NewCheck("Show Travel Moves", func(checked bool) {
		ui.viewer.showTravelMoves = checked
		ui.viewer.Refresh()
	})
	
	ui.supportsCheck = widget.NewCheck("Show Supports", func(checked bool) {
		ui.viewer.showSupports = checked
		ui.viewer.Refresh()
	})
	ui.supportsCheck.SetChecked(true)
	
	ui.fullscreenBtn = widget.NewButton("Fullscreen", func() {
		ui.toggleFullscreen()
	})
	ui.fullscreenBtn.Resize(fyne.NewSize(100, 40))
	
	ui.resetViewBtn = widget.NewButton("Reset View", func() {
		ui.viewer.ResetView()
	})
	ui.resetViewBtn.Resize(fyne.NewSize(100, 40))
	
	// File controls
	ui.fileSelect = widget.NewSelect([]string{}, func(selected string) {
		ui.currentFile = selected
	})
	ui.fileSelect.PlaceHolder = "Select G-code file..."
	
	ui.loadBtn = widget.NewButton("Load File", func() {
		ui.loadGCodeFile()
	})
	ui.loadBtn.Resize(fyne.NewSize(100, 40))
	
	ui.reloadBtn = widget.NewButton("Reload", func() {
		ui.reloadCurrentFile()
	})
	ui.reloadBtn.Resize(fyne.NewSize(80, 40))
	
	// Information cards
	ui.metadataCard = widget.NewCard("File Information", "", widget.NewLabel("No file loaded"))
	ui.layerInfoCard = widget.NewCard("Layer Information", "", widget.NewLabel("No layer selected"))
}

// createLayout creates the UI layout
func (ui *GCodeViewerUI) createLayout() {
	// Left panel with controls
	leftPanel := container.NewVBox(
		// File controls
		widget.NewCard("File", "", container.NewVBox(
			ui.fileSelect,
			container.NewGridWithColumns(2, ui.loadBtn, ui.reloadBtn),
		)),
		
		// Layer controls
		widget.NewCard("Layers", "", container.NewVBox(
			ui.layerLabel,
			ui.layerSlider,
			container.NewGridWithColumns(2, ui.showAllBtn, ui.showCurrentBtn),
		)),
		
		// Progress controls
		widget.NewCard("Progress", "", container.NewVBox(
			ui.progressLabel,
			ui.progressSlider,
			container.NewGridWithColumns(3, ui.playBtn, ui.pauseBtn, ui.resetBtn),
			container.NewHBox(
				widget.NewLabel("Speed:"),
				ui.speedSlider,
			),
		)),
		
		// Display options
		widget.NewCard("Display", "", container.NewVBox(
			ui.travelMovesCheck,
			ui.supportsCheck,
			container.NewGridWithColumns(2, ui.fullscreenBtn, ui.resetViewBtn),
		)),
		
		// Information
		ui.metadataCard,
		ui.layerInfoCard,
	)
	
	// Right panel with viewer
	viewerContainer := container.NewMax(ui.viewer)
	
	// Main layout
	ui.content = container.NewHSplit(
		container.NewScroll(leftPanel),
		viewerContainer,
	)
	ui.content.SetOffset(0.25) // 25% for controls, 75% for viewer
}

// setupInteractions sets up touch and mouse interactions
func (ui *GCodeViewerUI) setupInteractions() {
	// TODO: Add touch gesture handling when Fyne supports it better
	// For now, we'll use keyboard shortcuts and buttons
}

// loadGCodeFile loads a G-code file for viewing
func (ui *GCodeViewerUI) loadGCodeFile() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()
		
		// Show loading dialog
		progressDialog := dialog.NewProgressInfinite("Loading G-code", "Parsing file...", ui.window)
		progressDialog.Show()
		
		go func() {
			// Parse G-code
			parser := NewGCodeParser()
			model, parseErr := parser.ParseGCode(reader)
			
			// Close progress dialog
			progressDialog.Hide()
			
			if parseErr != nil {
				dialog.ShowError(fmt.Errorf("failed to parse G-code: %v", parseErr), ui.window)
				return
			}
			
			// Update UI on main thread
			ui.loadModel(model, reader.URI().Name())
		}()
		
	}, ui.window)
}

// loadModel loads a parsed G-code model
func (ui *GCodeViewerUI) loadModel(model *GCodeModel, filename string) {
	ui.model = model
	ui.currentFile = filename
	
	// Update viewer
	ui.viewer.LoadGCode(model)
	
	// Update controls
	ui.updateLayerControls()
	ui.updateProgressControls()
	ui.updateInformation()
	
	// Add to loaded files list
	baseName := filepath.Base(filename)
	if !ui.containsString(ui.loadedFiles, baseName) {
		ui.loadedFiles = append(ui.loadedFiles, baseName)
		ui.fileSelect.Options = ui.loadedFiles
	}
	ui.fileSelect.SetSelected(baseName)
}

// updateLayerControls updates layer-related controls
func (ui *GCodeViewerUI) updateLayerControls() {
	if ui.model == nil || len(ui.model.Layers) == 0 {
		ui.layerSlider.SetValue(0)
		ui.layerSlider.Max = 1
		ui.layerLabel.SetText("Layer: 0/0")
		return
	}
	
	layerCount := len(ui.model.Layers)
	ui.layerSlider.Max = float64(layerCount - 1)
	ui.layerSlider.SetValue(0)
	ui.layerLabel.SetText(fmt.Sprintf("Layer: 1/%d", layerCount))
}

// updateProgressControls updates progress-related controls
func (ui *GCodeViewerUI) updateProgressControls() {
	if ui.model == nil || len(ui.model.Commands) == 0 {
		ui.progressSlider.SetValue(0)
		ui.progressSlider.Max = 1
		ui.progressLabel.SetText("Progress: 0%")
		return
	}
	
	commandCount := len(ui.model.Commands)
	ui.progressSlider.Max = float64(commandCount - 1)
	ui.progressSlider.SetValue(0)
	ui.progressLabel.SetText("Progress: 0.0%")
}

// updateInformation updates information display cards
func (ui *GCodeViewerUI) updateInformation() {
	if ui.model == nil {
		ui.metadataCard.SetContent(widget.NewLabel("No file loaded"))
		ui.layerInfoCard.SetContent(widget.NewLabel("No layer selected"))
		return
	}
	
	// Update metadata
	metadata := ui.model.Metadata
	metadataText := fmt.Sprintf(
		"Generated by: %s\n"+
		"Total layers: %d\n"+
		"Print time: %.1f hours\n"+
		"Filament used: %.2f mm\n"+
		"Layer height: %.2f mm\n"+
		"Infill density: %.1f%%\n"+
		"Bounds: X=%.1f-%.1f, Y=%.1f-%.1f, Z=%.1f-%.1f",
		metadata.GeneratedBy,
		metadata.TotalLayers,
		metadata.PrintTime/3600,
		metadata.FilamentUsed,
		metadata.LayerHeight,
		metadata.InfillDensity,
		ui.model.Bounds.MinX, ui.model.Bounds.MaxX,
		ui.model.Bounds.MinY, ui.model.Bounds.MaxY,
		ui.model.Bounds.MinZ, ui.model.Bounds.MaxZ,
	)
	ui.metadataCard.SetContent(widget.NewLabel(metadataText))
	
	// Update layer info for current layer
	ui.updateCurrentLayerInfo()
}

// updateCurrentLayerInfo updates information for the current layer
func (ui *GCodeViewerUI) updateCurrentLayerInfo() {
	if ui.model == nil || ui.viewer.currentLayer >= len(ui.model.Layers) {
		ui.layerInfoCard.SetContent(widget.NewLabel("No layer selected"))
		return
	}
	
	layer := ui.model.Layers[ui.viewer.currentLayer]
	layerText := fmt.Sprintf(
		"Layer %d\n"+
		"Z height: %.2f mm\n"+
		"Paths: %d\n"+
		"Filament used: %.2f mm\n"+
		"Lines: %d - %d\n"+
		"Bounds: X=%.1f-%.1f, Y=%.1f-%.1f",
		layer.Index+1,
		layer.Z,
		len(layer.Paths),
		layer.FilamentUsed,
		layer.StartLine, layer.EndLine,
		layer.BoundingBox.MinX, layer.BoundingBox.MaxX,
		layer.BoundingBox.MinY, layer.BoundingBox.MaxY,
	)
	ui.layerInfoCard.SetContent(widget.NewLabel(layerText))
}

// setCurrentLayer sets the current layer
func (ui *GCodeViewerUI) setCurrentLayer(layer int) {
	if ui.model == nil {
		return
	}
	
	ui.viewer.SetCurrentLayer(layer)
	ui.layerLabel.SetText(fmt.Sprintf("Layer: %d/%d", layer+1, len(ui.model.Layers)))
	ui.updateCurrentLayerInfo()
}

// setProgress sets the current progress
func (ui *GCodeViewerUI) setProgress(progress float64) {
	if ui.model == nil {
		return
	}
	
	line := int(progress)
	ui.viewer.SetCurrentLine(line)
	
	progressPercent := progress / float64(len(ui.model.Commands)-1) * 100
	ui.progressLabel.SetText(fmt.Sprintf("Progress: %.1f%%", progressPercent))
	
	// Update layer based on current line
	if line < len(ui.model.Commands) {
		cmd := ui.model.Commands[line]
		for i, layer := range ui.model.Layers {
			if cmd.LineNumber >= layer.StartLine && cmd.LineNumber <= layer.EndLine {
				if i != ui.viewer.currentLayer {
					ui.layerSlider.SetValue(float64(i))
					ui.setCurrentLayer(i)
				}
				break
			}
		}
	}
}

// startAnimation starts progress animation
func (ui *GCodeViewerUI) startAnimation() {
	if ui.isPlaying || ui.model == nil {
		return
	}
	
	ui.isPlaying = true
	ui.playBtn.Hide()
	ui.pauseBtn.Show()
	
	// Start animation ticker
	interval := time.Duration(50.0/ui.playbackSpeed) * time.Millisecond
	ui.animationTicker = time.NewTicker(interval)
	
	go func() {
		for ui.isPlaying {
			select {
			case <-ui.animationTicker.C:
				currentProgress := ui.progressSlider.Value
				maxProgress := ui.progressSlider.Max
				
				if currentProgress >= maxProgress {
					ui.pauseAnimation()
					return
				}
				
				// Increment progress
				newProgress := currentProgress + 1
				ui.progressSlider.SetValue(newProgress)
				ui.setProgress(newProgress)
				
				// Update ticker interval if speed changed
				newInterval := time.Duration(50.0/ui.playbackSpeed) * time.Millisecond
				ui.animationTicker.Reset(newInterval)
			}
		}
	}()
}

// pauseAnimation pauses progress animation
func (ui *GCodeViewerUI) pauseAnimation() {
	ui.isPlaying = false
	ui.pauseBtn.Hide()
	ui.playBtn.Show()
	
	if ui.animationTicker != nil {
		ui.animationTicker.Stop()
	}
}

// resetAnimation resets progress to beginning
func (ui *GCodeViewerUI) resetAnimation() {
	ui.pauseAnimation()
	ui.progressSlider.SetValue(0)
	ui.setProgress(0)
	ui.layerSlider.SetValue(0)
	ui.setCurrentLayer(0)
}

// reloadCurrentFile reloads the current file
func (ui *GCodeViewerUI) reloadCurrentFile() {
	if ui.currentFile == "" {
		ui.loadGCodeFile()
		return
	}
	
	// Try to reload from filesystem
	file, err := os.Open(ui.currentFile)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to reload file: %v", err), ui.window)
		return
	}
	defer file.Close()
	
	// Parse G-code
	parser := NewGCodeParser()
	model, err := parser.ParseGCode(file)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to parse G-code: %v", err), ui.window)
		return
	}
	
	ui.loadModel(model, ui.currentFile)
}

// toggleFullscreen toggles fullscreen mode
func (ui *GCodeViewerUI) toggleFullscreen() {
	// Create new fullscreen window
	fullscreenWindow := ui.window.App().NewWindow("G-code Viewer - Fullscreen")
	fullscreenWindow.SetFullScreen(true)
	
	// Create viewer copy for fullscreen
	fullscreenViewer := NewGCodeViewer()
	if ui.model != nil {
		fullscreenViewer.LoadGCode(ui.model)
		fullscreenViewer.SetCurrentLayer(ui.viewer.currentLayer)
		fullscreenViewer.SetCurrentLine(ui.viewer.currentLine)
	}
	
	// Simple controls overlay
	exitBtn := widget.NewButton("Exit Fullscreen", func() {
		fullscreenWindow.Close()
	})
	
	overlay := container.NewBorder(
		nil, container.NewHBox(layout.NewSpacer(), exitBtn), // bottom
		nil, nil, // left, right
		fullscreenViewer, // center
	)
	
	fullscreenWindow.SetContent(overlay)
	fullscreenWindow.Show()
}

// getAllLayerIndices returns all layer indices
func (ui *GCodeViewerUI) getAllLayerIndices() []int {
	if ui.model == nil {
		return []int{}
	}
	
	indices := make([]int, len(ui.model.Layers))
	for i := range indices {
		indices[i] = i
	}
	return indices
}

// containsString checks if a string slice contains a string
func (ui *GCodeViewerUI) containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetContent returns the UI content
func (ui *GCodeViewerUI) GetContent() *fyne.Container {
	return ui.content
}

// Stop stops any running animations
func (ui *GCodeViewerUI) Stop() {
	ui.pauseAnimation()
}

// LoadGCodeFromFile loads G-code from a file path
func (ui *GCodeViewerUI) LoadGCodeFromFile(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	parser := NewGCodeParser()
	model, err := parser.ParseGCode(file)
	if err != nil {
		return err
	}
	
	ui.loadModel(model, filepath)
	return nil
}

// SyncWithPrintProgress syncs viewer with actual print progress
func (ui *GCodeViewerUI) SyncWithPrintProgress(currentLine int) {
	if ui.model == nil {
		return
	}
	
	// Don't sync if user is manually controlling
	if ui.isPlaying {
		return
	}
	
	ui.progressSlider.SetValue(float64(currentLine))
	ui.setProgress(float64(currentLine))
} 