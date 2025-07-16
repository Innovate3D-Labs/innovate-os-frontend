// +build ignore

package main

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Test G-code viewer with sample data
func main() {
	// Create test app
	testApp := app.New()
	testApp.Settings().SetTheme(&InnovateTheme{})
	
	window := testApp.NewWindow("G-code Viewer Test")
	window.Resize(fyne.NewSize(1400, 900))
	
	// Create mock backend
	backend := &MockBackend{}
	
	// Create G-code viewer UI
	viewerUI := NewGCodeViewerUI(window, backend)
	
	// Test controls
	generateSimpleBtn := widget.NewButton("Generate Simple Cube", func() {
		model := generateSimpleCube()
		viewerUI.loadModel(model, "simple_cube.gcode")
	})
	generateSimpleBtn.Resize(fyne.NewSize(150, 50))
	
	generateComplexBtn := widget.NewButton("Generate Complex Model", func() {
		model := generateComplexModel()
		viewerUI.loadModel(model, "complex_model.gcode")
	})
	generateComplexBtn.Resize(fyne.NewSize(150, 50))
	
	generateTowerBtn := widget.NewButton("Generate Tower", func() {
		model := generateTower()
		viewerUI.loadModel(model, "tower.gcode")
	})
	generateTowerBtn.Resize(fyne.NewSize(150, 50))
	
	animateBtn := widget.NewButton("Start Animation", func() {
		go simulateRealTimePrint(viewerUI)
	})
	animateBtn.Resize(fyne.NewSize(150, 50))
	
	// Test stats
	statsLabel := widget.NewLabel("Viewer Statistics:")
	
	// Update stats periodically
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			model := viewerUI.model
			if model != nil {
				stats := fmt.Sprintf(
					"Model: %s\nLayers: %d\nPaths: %d\nCommands: %d\nCurrent Layer: %d\nProgress: %.1f%%",
					viewerUI.currentFile,
					len(model.Layers),
					len(model.Paths),
					len(model.Commands),
					viewerUI.viewer.currentLayer+1,
					float64(viewerUI.viewer.currentLine)/float64(len(model.Commands))*100,
				)
				statsLabel.SetText(stats)
			} else {
				statsLabel.SetText("No model loaded")
			}
		}
	}()
	
	// Controls panel
	controls := container.NewVBox(
		widget.NewCard("Test Controls", "", container.NewVBox(
			generateSimpleBtn,
			generateComplexBtn,
			generateTowerBtn,
			animateBtn,
		)),
		widget.NewCard("Statistics", "", statsLabel),
	)
	
	// Layout
	content := container.NewHSplit(
		controls,
		viewerUI.GetContent(),
	)
	content.SetOffset(0.2) // 20% for controls, 80% for viewer
	
	window.SetContent(content)
	
	// Instructions
	log.Println("G-code Viewer Test Demo")
	log.Println("=======================")
	log.Println("This demo tests:")
	log.Println("1. G-code parsing and 3D visualization")
	log.Println("2. Layer navigation and progress scrubbing")
	log.Println("3. 3D camera controls and view manipulation")
	log.Println("4. Real-time print progress simulation")
	log.Println("5. Path type visualization and filtering")
	log.Println("")
	log.Println("Try these features:")
	log.Println("- Generate different test models")
	log.Println("- Navigate through layers with the slider")
	log.Println("- Use play/pause controls for animation")
	log.Println("- Toggle travel moves and supports")
	log.Println("- Try fullscreen mode")
	log.Println("- Reset view to fit model")
	
	window.ShowAndRun()
}

// generateSimpleCube creates a simple cube G-code model
func generateSimpleCube() *GCodeModel {
	model := &GCodeModel{
		Commands: make([]GCodeCommand, 0),
		Paths:    make([]GCodePath, 0),
		Layers:   make([]GCodeLayer, 0),
		Bounds: GCodeBounds{
			MinX: 0, MaxX: 20,
			MinY: 0, MaxY: 20,
			MinZ: 0, MaxZ: 20,
		},
		Metadata: GCodeMetadata{
			GeneratedBy:   "Test Generator",
			TotalLayers:   20,
			LayerHeight:   1.0,
			PrintTime:     3600, // 1 hour
			FilamentUsed:  100.0,
			InfillDensity: 20.0,
		},
	}
	
	// Generate layers
	for layer := 0; layer < 20; layer++ {
		z := float64(layer) * 1.0
		
		layerObj := GCodeLayer{
			Index:     layer,
			Z:         z,
			StartLine: len(model.Commands),
			Paths:     make([]int, 0),
		}
		
		// Outer perimeter (square)
		perimeter := []struct{ x, y float64 }{
			{0, 0}, {20, 0}, {20, 20}, {0, 20}, {0, 0},
		}
		
		for i := 0; i < len(perimeter)-1; i++ {
			start := perimeter[i]
			end := perimeter[i+1]
			
			// Create command
			cmd := GCodeCommand{
				Type:       "G1",
				X:          end.x,
				Y:          end.y,
				Z:          z,
				E:          float64(len(model.Commands)) * 0.1,
				F:          1500,
				LineNumber: len(model.Commands) + 1,
				IsValid:    true,
			}
			model.Commands = append(model.Commands, cmd)
			
			// Create path
			path := GCodePath{
				StartX:          start.x,
				StartY:          start.y,
				StartZ:          z,
				EndX:            end.x,
				EndY:            end.y,
				EndZ:            z,
				ExtrusionAmount: 0.1,
				Speed:           1500,
				LayerIndex:      layer,
				PathType:        PathTypePerimeter,
				LineNumber:      len(model.Commands),
			}
			model.Paths = append(model.Paths, path)
			layerObj.Paths = append(layerObj.Paths, len(model.Paths)-1)
		}
		
		// Infill (simple lines)
		for y := 2.0; y < 18.0; y += 4.0 {
			// Create infill line
			cmd := GCodeCommand{
				Type:       "G1",
				X:          18,
				Y:          y,
				Z:          z,
				E:          float64(len(model.Commands)) * 0.1,
				F:          3000,
				LineNumber: len(model.Commands) + 1,
				IsValid:    true,
			}
			model.Commands = append(model.Commands, cmd)
			
			path := GCodePath{
				StartX:          2,
				StartY:          y,
				StartZ:          z,
				EndX:            18,
				EndY:            y,
				EndZ:            z,
				ExtrusionAmount: 0.1,
				Speed:           3000,
				LayerIndex:      layer,
				PathType:        PathTypeInfill,
				LineNumber:      len(model.Commands),
			}
			model.Paths = append(model.Paths, path)
			layerObj.Paths = append(layerObj.Paths, len(model.Paths)-1)
		}
		
		layerObj.EndLine = len(model.Commands)
		model.Layers = append(model.Layers, layerObj)
	}
	
	model.TotalLines = len(model.Commands)
	return model
}

// generateComplexModel creates a more complex model with various path types
func generateComplexModel() *GCodeModel {
	model := &GCodeModel{
		Commands: make([]GCodeCommand, 0),
		Paths:    make([]GCodePath, 0),
		Layers:   make([]GCodeLayer, 0),
		Bounds: GCodeBounds{
			MinX: -15, MaxX: 15,
			MinY: -15, MaxY: 15,
			MinZ: 0, MaxZ: 30,
		},
		Metadata: GCodeMetadata{
			GeneratedBy:   "Test Generator Complex",
			TotalLayers:   30,
			LayerHeight:   1.0,
			PrintTime:     7200, // 2 hours
			FilamentUsed:  250.0,
			InfillDensity: 40.0,
		},
	}
	
	// Generate layers with varying complexity
	for layer := 0; layer < 30; layer++ {
		z := float64(layer) * 1.0
		
		layerObj := GCodeLayer{
			Index:     layer,
			Z:         z,
			StartLine: len(model.Commands),
			Paths:     make([]int, 0),
		}
		
		// Create circular perimeter
		radius := 15.0 - float64(layer)*0.3 // Tapered cone
		segments := 32
		
		for i := 0; i < segments; i++ {
			angle1 := float64(i) * 2 * math.Pi / float64(segments)
			angle2 := float64(i+1) * 2 * math.Pi / float64(segments)
			
			x1 := radius * math.Cos(angle1)
			y1 := radius * math.Sin(angle1)
			x2 := radius * math.Cos(angle2)
			y2 := radius * math.Sin(angle2)
			
			// Create command
			cmd := GCodeCommand{
				Type:       "G1",
				X:          x2,
				Y:          y2,
				Z:          z,
				E:          float64(len(model.Commands)) * 0.05,
				F:          1200,
				LineNumber: len(model.Commands) + 1,
				IsValid:    true,
			}
			model.Commands = append(model.Commands, cmd)
			
			// Create path
			path := GCodePath{
				StartX:          x1,
				StartY:          y1,
				StartZ:          z,
				EndX:            x2,
				EndY:            y2,
				EndZ:            z,
				ExtrusionAmount: 0.05,
				Speed:           1200,
				LayerIndex:      layer,
				PathType:        PathTypePerimeter,
				LineNumber:      len(model.Commands),
			}
			model.Paths = append(model.Paths, path)
			layerObj.Paths = append(layerObj.Paths, len(model.Paths)-1)
		}
		
		// Add infill pattern (honeycomb-like)
		if layer%2 == 0 {
			// Horizontal infill
			for y := -radius + 2; y < radius-2; y += 3 {
				x1 := -math.Sqrt(radius*radius-y*y) + 2
				x2 := math.Sqrt(radius*radius-y*y) - 2
				
				cmd := GCodeCommand{
					Type:       "G1",
					X:          x2,
					Y:          y,
					Z:          z,
					E:          float64(len(model.Commands)) * 0.03,
					F:          2400,
					LineNumber: len(model.Commands) + 1,
					IsValid:    true,
				}
				model.Commands = append(model.Commands, cmd)
				
				path := GCodePath{
					StartX:          x1,
					StartY:          y,
					StartZ:          z,
					EndX:            x2,
					EndY:            y,
					EndZ:            z,
					ExtrusionAmount: 0.03,
					Speed:           2400,
					LayerIndex:      layer,
					PathType:        PathTypeInfill,
					LineNumber:      len(model.Commands),
				}
				model.Paths = append(model.Paths, path)
				layerObj.Paths = append(layerObj.Paths, len(model.Paths)-1)
			}
		}
		
		// Add travel moves between sections
		if layer > 0 {
			cmd := GCodeCommand{
				Type:       "G0",
				X:          0,
				Y:          0,
				Z:          z + 0.2, // Lift for travel
				LineNumber: len(model.Commands) + 1,
				IsValid:    true,
			}
			model.Commands = append(model.Commands, cmd)
			
			path := GCodePath{
				StartX:          radius,
				StartY:          0,
				StartZ:          z,
				EndX:            0,
				EndY:            0,
				EndZ:            z + 0.2,
				ExtrusionAmount: 0,
				Speed:           4800,
				LayerIndex:      layer,
				PathType:        PathTypeTravel,
				LineNumber:      len(model.Commands),
			}
			model.Paths = append(model.Paths, path)
			layerObj.Paths = append(layerObj.Paths, len(model.Paths)-1)
		}
		
		layerObj.EndLine = len(model.Commands)
		model.Layers = append(model.Layers, layerObj)
	}
	
	model.TotalLines = len(model.Commands)
	return model
}

// generateTower creates a tower model with supports
func generateTower() *GCodeModel {
	model := &GCodeModel{
		Commands: make([]GCodeCommand, 0),
		Paths:    make([]GCodePath, 0),
		Layers:   make([]GCodeLayer, 0),
		Bounds: GCodeBounds{
			MinX: -10, MaxX: 10,
			MinY: -10, MaxY: 10,
			MinZ: 0, MaxZ: 50,
		},
		Metadata: GCodeMetadata{
			GeneratedBy:   "Test Generator Tower",
			TotalLayers:   50,
			LayerHeight:   1.0,
			PrintTime:     5400, // 1.5 hours
			FilamentUsed:  180.0,
			InfillDensity: 15.0,
		},
	}
	
	// Generate tower layers
	for layer := 0; layer < 50; layer++ {
		z := float64(layer) * 1.0
		
		layerObj := GCodeLayer{
			Index:     layer,
			Z:         z,
			StartLine: len(model.Commands),
			Paths:     make([]int, 0),
		}
		
		// Main tower (getting smaller as it goes up)
		size := 10.0 - float64(layer)*0.15
		if size < 2.0 {
			size = 2.0
		}
		
		// Tower perimeter
		corners := []struct{ x, y float64 }{
			{-size, -size}, {size, -size}, {size, size}, {-size, size}, {-size, -size},
		}
		
		for i := 0; i < len(corners)-1; i++ {
			start := corners[i]
			end := corners[i+1]
			
			cmd := GCodeCommand{
				Type:       "G1",
				X:          end.x,
				Y:          end.y,
				Z:          z,
				E:          float64(len(model.Commands)) * 0.08,
				F:          1000,
				LineNumber: len(model.Commands) + 1,
				IsValid:    true,
			}
			model.Commands = append(model.Commands, cmd)
			
			path := GCodePath{
				StartX:          start.x,
				StartY:          start.y,
				StartZ:          z,
				EndX:            end.x,
				EndY:            end.y,
				EndZ:            z,
				ExtrusionAmount: 0.08,
				Speed:           1000,
				LayerIndex:      layer,
				PathType:        PathTypePerimeter,
				LineNumber:      len(model.Commands),
			}
			model.Paths = append(model.Paths, path)
			layerObj.Paths = append(layerObj.Paths, len(model.Paths)-1)
		}
		
		// Support columns (only for first 20 layers)
		if layer < 20 {
			supports := []struct{ x, y float64 }{
				{-8, -8}, {8, -8}, {8, 8}, {-8, 8},
			}
			
			for _, support := range supports {
				// Small support pillar
				for dx := -0.5; dx <= 0.5; dx += 1.0 {
					cmd := GCodeCommand{
						Type:       "G1",
						X:          support.x + dx,
						Y:          support.y + 0.5,
						Z:          z,
						E:          float64(len(model.Commands)) * 0.02,
						F:          800,
						LineNumber: len(model.Commands) + 1,
						IsValid:    true,
					}
					model.Commands = append(model.Commands, cmd)
					
					path := GCodePath{
						StartX:          support.x + dx,
						StartY:          support.y - 0.5,
						StartZ:          z,
						EndX:            support.x + dx,
						EndY:            support.y + 0.5,
						EndZ:            z,
						ExtrusionAmount: 0.02,
						Speed:           800,
						LayerIndex:      layer,
						PathType:        PathTypeSupport,
						LineNumber:      len(model.Commands),
					}
					model.Paths = append(model.Paths, path)
					layerObj.Paths = append(layerObj.Paths, len(model.Paths)-1)
				}
			}
		}
		
		layerObj.EndLine = len(model.Commands)
		model.Layers = append(model.Layers, layerObj)
	}
	
	model.TotalLines = len(model.Commands)
	return model
}

// simulateRealTimePrint simulates real-time printing progress
func simulateRealTimePrint(viewerUI *GCodeViewerUI) {
	if viewerUI.model == nil {
		return
	}
	
	log.Println("Starting real-time print simulation...")
	
	totalCommands := len(viewerUI.model.Commands)
	
	for i := 0; i < totalCommands; i++ {
		// Update viewer progress
		viewerUI.SyncWithPrintProgress(i)
		
		// Simulate print speed (faster for demo)
		time.Sleep(20 * time.Millisecond)
		
		// Add some variation in speed
		if i%100 == 0 {
			log.Printf("Print progress: %.1f%%", float64(i)/float64(totalCommands)*100)
		}
	}
	
	log.Println("Print simulation complete")
} 