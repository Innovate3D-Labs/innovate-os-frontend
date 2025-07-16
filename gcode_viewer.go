package main

import (
	"fmt"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// GCodeViewer displays 3D visualization of G-code
type GCodeViewer struct {
	widget.BaseWidget
	
	// Data
	model         *GCodeModel
	currentLayer  int
	currentLine   int
	visibleLayers []int
	
	// 3D view settings
	camera        Camera3D
	width         float32
	height        float32
	
	// Display options
	showTravelMoves   bool
	showSupports      bool
	pathColors        map[PathType]color.Color
	backgroundColor   color.Color
	
	// Animation
	animationSpeed    float64
	isAnimating       bool
	animationProgress float64
	
	// Interaction
	isDragging        bool
	lastDragPos       fyne.Position
	touchStartPos     fyne.Position
	touchStartTime    int64
}

// Camera3D represents the 3D view camera
type Camera3D struct {
	RotationX    float64 // Rotation around X axis (pitch)
	RotationY    float64 // Rotation around Y axis (yaw)
	RotationZ    float64 // Rotation around Z axis (roll)
	Zoom         float64 // Zoom level
	PanX, PanY   float64 // Pan offset
	Distance     float64 // Distance from object
}

// Point3D represents a 3D point
type Point3D struct {
	X, Y, Z float64
}

// Point2D represents a 2D screen point
type Point2D struct {
	X, Y float32
}

// NewGCodeViewer creates a new G-code viewer
func NewGCodeViewer() *GCodeViewer {
	viewer := &GCodeViewer{
		currentLayer:    0,
		visibleLayers:   make([]int, 0),
		showTravelMoves: false,
		showSupports:    true,
		animationSpeed:  1.0,
		backgroundColor: color.NRGBA{R: 20, G: 20, B: 25, A: 255},
		
		camera: Camera3D{
			RotationX: -30,
			RotationY: 45,
			Zoom:      1.0,
			Distance:  200,
		},
		
		pathColors: map[PathType]color.Color{
			PathTypeTravel:     color.NRGBA{R: 100, G: 100, B: 100, A: 128}, // Gray transparent
			PathTypeExtrusion:  color.NRGBA{R: 255, G: 255, B: 255, A: 255}, // White
			PathTypeRetraction: color.NRGBA{R: 255, G: 100, B: 100, A: 255}, // Light red
			PathTypePerimeter:  color.NRGBA{R: 0, G: 150, B: 255, A: 255},   // Blue
			PathTypeInfill:     color.NRGBA{R: 255, G: 200, B: 0, A: 255},   // Yellow
			PathTypeSupport:    color.NRGBA{R: 150, G: 75, B: 0, A: 255},    // Brown
		},
	}
	
	viewer.ExtendBaseWidget(viewer)
	return viewer
}

// LoadGCode loads a G-code model for visualization
func (v *GCodeViewer) LoadGCode(model *GCodeModel) {
	v.model = model
	v.currentLayer = 0
	v.currentLine = 0
	v.visibleLayers = make([]int, len(model.Layers))
	for i := range v.visibleLayers {
		v.visibleLayers[i] = i
	}
	
	// Auto-fit the view
	v.fitToView()
	v.Refresh()
}

// CreateRenderer creates the viewer renderer
func (v *GCodeViewer) CreateRenderer() fyne.WidgetRenderer {
	return &gcodeViewerRenderer{viewer: v}
}

// gcodeViewerRenderer renders the G-code viewer
type gcodeViewerRenderer struct {
	viewer *GCodeViewer
}

func (r *gcodeViewerRenderer) Layout(size fyne.Size) {
	r.viewer.width = size.Width
	r.viewer.height = size.Height
}

func (r *gcodeViewerRenderer) MinSize() fyne.Size {
	return fyne.NewSize(400, 300)
}

func (r *gcodeViewerRenderer) Refresh() {
	// Refresh handled by redrawing
}

func (r *gcodeViewerRenderer) Destroy() {
	// Nothing to destroy
}

func (r *gcodeViewerRenderer) Objects() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}
	
	if r.viewer.model == nil {
		// Show loading message
		text := canvas.NewText("No G-code loaded", color.White)
		text.Alignment = fyne.TextAlignCenter
		text.Move(fyne.NewPos(r.viewer.width/2-50, r.viewer.height/2))
		return []fyne.CanvasObject{text}
	}
	
	// Draw background
	bg := canvas.NewRectangle(r.viewer.backgroundColor)
	bg.Resize(fyne.NewSize(r.viewer.width, r.viewer.height))
	objects = append(objects, bg)
	
	// Draw build platform
	objects = append(objects, r.drawBuildPlatform()...)
	
	// Draw G-code paths
	objects = append(objects, r.drawGCodePaths()...)
	
	// Draw current position indicator
	objects = append(objects, r.drawCurrentPosition()...)
	
	// Draw UI overlay
	objects = append(objects, r.drawUIOverlay()...)
	
	return objects
}

// drawBuildPlatform draws the build platform grid
func (r *gcodeViewerRenderer) drawBuildPlatform() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}
	
	if r.viewer.model == nil {
		return objects
	}
	
	bounds := r.viewer.model.Bounds
	centerX := (bounds.MinX + bounds.MaxX) / 2
	centerY := (bounds.MinY + bounds.MaxY) / 2
	
	// Draw grid lines
	gridSize := 10.0
	gridColor := color.NRGBA{R: 60, G: 60, B: 60, A: 255}
	
	// Vertical lines
	for x := math.Floor(bounds.MinX/gridSize)*gridSize; x <= bounds.MaxX; x += gridSize {
		start := r.viewer.project3DTo2D(Point3D{X: x, Y: bounds.MinY, Z: bounds.MinZ})
		end := r.viewer.project3DTo2D(Point3D{X: x, Y: bounds.MaxY, Z: bounds.MinZ})
		
		line := canvas.NewLine(gridColor)
		line.Position1 = fyne.NewPos(start.X, start.Y)
		line.Position2 = fyne.NewPos(end.X, end.Y)
		line.StrokeWidth = 1
		objects = append(objects, line)
	}
	
	// Horizontal lines
	for y := math.Floor(bounds.MinY/gridSize)*gridSize; y <= bounds.MaxY; y += gridSize {
		start := r.viewer.project3DTo2D(Point3D{X: bounds.MinX, Y: y, Z: bounds.MinZ})
		end := r.viewer.project3DTo2D(Point3D{X: bounds.MaxX, Y: y, Z: bounds.MinZ})
		
		line := canvas.NewLine(gridColor)
		line.Position1 = fyne.NewPos(start.X, start.Y)
		line.Position2 = fyne.NewPos(end.X, end.Y)
		line.StrokeWidth = 1
		objects = append(objects, line)
	}
	
	// Center axes
	axisColor := color.NRGBA{R: 255, G: 255, B: 255, A: 128}
	
	// X axis
	xStart := r.viewer.project3DTo2D(Point3D{X: centerX - 20, Y: centerY, Z: bounds.MinZ})
	xEnd := r.viewer.project3DTo2D(Point3D{X: centerX + 20, Y: centerY, Z: bounds.MinZ})
	xAxis := canvas.NewLine(color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	xAxis.Position1 = fyne.NewPos(xStart.X, xStart.Y)
	xAxis.Position2 = fyne.NewPos(xEnd.X, xEnd.Y)
	xAxis.StrokeWidth = 2
	objects = append(objects, xAxis)
	
	// Y axis
	yStart := r.viewer.project3DTo2D(Point3D{X: centerX, Y: centerY - 20, Z: bounds.MinZ})
	yEnd := r.viewer.project3DTo2D(Point3D{X: centerX, Y: centerY + 20, Z: bounds.MinZ})
	yAxis := canvas.NewLine(color.NRGBA{R: 0, G: 255, B: 0, A: 255})
	yAxis.Position1 = fyne.NewPos(yStart.X, yStart.Y)
	yAxis.Position2 = fyne.NewPos(yEnd.X, yEnd.Y)
	yAxis.StrokeWidth = 2
	objects = append(objects, yAxis)
	
	return objects
}

// drawGCodePaths draws the 3D printing paths
func (r *gcodeViewerRenderer) drawGCodePaths() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}
	
	if r.viewer.model == nil || len(r.viewer.model.Paths) == 0 {
		return objects
	}
	
	// Draw paths for visible layers
	for _, layerIndex := range r.viewer.visibleLayers {
		if layerIndex >= len(r.viewer.model.Layers) {
			continue
		}
		
		layer := r.viewer.model.Layers[layerIndex]
		
		for _, pathIndex := range layer.Paths {
			if pathIndex >= len(r.viewer.model.Paths) {
				continue
			}
			
			path := r.viewer.model.Paths[pathIndex]
			
			// Skip travel moves if disabled
			if !r.viewer.showTravelMoves && path.PathType == PathTypeTravel {
				continue
			}
			
			// Skip supports if disabled
			if !r.viewer.showSupports && path.PathType == PathTypeSupport {
				continue
			}
			
			// Determine line color and thickness
			pathColor := r.viewer.pathColors[path.PathType]
			lineWidth := float32(1)
			
			// Highlight current and completed paths
			if path.LineNumber <= r.viewer.currentLine {
				// Already printed - make slightly dimmer
				if path.PathType != PathTypeTravel {
					pathColor = r.dimColor(pathColor, 0.8)
				}
			} else {
				// Not yet printed - make much dimmer
				pathColor = r.dimColor(pathColor, 0.3)
			}
			
			// Highlight current path
			if path.LineNumber == r.viewer.currentLine {
				pathColor = color.NRGBA{R: 255, G: 0, B: 255, A: 255} // Magenta for current
				lineWidth = 3
			}
			
			// Adjust line width based on path type
			switch path.PathType {
			case PathTypePerimeter:
				lineWidth += 1
			case PathTypeTravel:
				lineWidth = 1
			case PathTypeRetraction:
				lineWidth = 2
			}
			
			// Project to 2D and draw line
			start := r.viewer.project3DTo2D(Point3D{X: path.StartX, Y: path.StartY, Z: path.StartZ})
			end := r.viewer.project3DTo2D(Point3D{X: path.EndX, Y: path.EndY, Z: path.EndZ})
			
			line := canvas.NewLine(pathColor)
			line.Position1 = fyne.NewPos(start.X, start.Y)
			line.Position2 = fyne.NewPos(end.X, end.Y)
			line.StrokeWidth = lineWidth
			objects = append(objects, line)
		}
	}
	
	return objects
}

// drawCurrentPosition draws the current print head position
func (r *gcodeViewerRenderer) drawCurrentPosition() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}
	
	if r.viewer.model == nil || r.viewer.currentLine >= len(r.viewer.model.Commands) {
		return objects
	}
	
	// Find current position from executed commands
	var currentX, currentY, currentZ float64
	
	for i := 0; i <= r.viewer.currentLine && i < len(r.viewer.model.Commands); i++ {
		cmd := r.viewer.model.Commands[i]
		if cmd.Type == "G0" || cmd.Type == "G1" {
			if !math.IsNaN(cmd.X) {
				currentX = cmd.X
			}
			if !math.IsNaN(cmd.Y) {
				currentY = cmd.Y
			}
			if !math.IsNaN(cmd.Z) {
				currentZ = cmd.Z
			}
		}
	}
	
	// Draw print head indicator
	pos := r.viewer.project3DTo2D(Point3D{X: currentX, Y: currentY, Z: currentZ})
	
	// Outer circle
	outerCircle := canvas.NewCircle(color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	outerCircle.Resize(fyne.NewSize(12, 12))
	outerCircle.Move(fyne.NewPos(pos.X-6, pos.Y-6))
	objects = append(objects, outerCircle)
	
	// Inner circle
	innerCircle := canvas.NewCircle(color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	innerCircle.Resize(fyne.NewSize(6, 6))
	innerCircle.Move(fyne.NewPos(pos.X-3, pos.Y-3))
	objects = append(objects, innerCircle)
	
	return objects
}

// drawUIOverlay draws UI information overlay
func (r *gcodeViewerRenderer) drawUIOverlay() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{}
	
	if r.viewer.model == nil {
		return objects
	}
	
	// Layer info
	layerText := fmt.Sprintf("Layer: %d/%d", r.viewer.currentLayer+1, len(r.viewer.model.Layers))
	layerLabel := canvas.NewText(layerText, color.White)
	layerLabel.Move(fyne.NewPos(10, 10))
	layerLabel.TextSize = 14
	objects = append(objects, layerLabel)
	
	// Progress info
	progressPercent := float64(r.viewer.currentLine) / float64(len(r.viewer.model.Commands)) * 100
	progressText := fmt.Sprintf("Progress: %.1f%%", progressPercent)
	progressLabel := canvas.NewText(progressText, color.White)
	progressLabel.Move(fyne.NewPos(10, 30))
	progressLabel.TextSize = 14
	objects = append(objects, progressLabel)
	
	// Current line info
	if r.viewer.currentLine < len(r.viewer.model.Commands) {
		cmd := r.viewer.model.Commands[r.viewer.currentLine]
		lineText := fmt.Sprintf("Line: %d - %s", cmd.LineNumber, cmd.Type)
		lineLabel := canvas.NewText(lineText, color.White)
		lineLabel.Move(fyne.NewPos(10, 50))
		lineLabel.TextSize = 12
		objects = append(objects, lineLabel)
	}
	
	// View controls hint
	hintText := "Touch: Rotate | Pinch: Zoom | Double-tap: Reset"
	hintLabel := canvas.NewText(hintText, color.NRGBA{R: 200, G: 200, B: 200, A: 255})
	hintLabel.Move(fyne.NewPos(10, r.viewer.height-25))
	hintLabel.TextSize = 10
	objects = append(objects, hintLabel)
	
	return objects
}

// project3DTo2D projects 3D coordinates to 2D screen coordinates
func (v *GCodeViewer) project3DTo2D(point Point3D) Point2D {
	// Apply camera transformations
	
	// 1. Translate to origin (center the model)
	bounds := v.model.Bounds
	centerX := (bounds.MinX + bounds.MaxX) / 2
	centerY := (bounds.MinY + bounds.MaxY) / 2
	centerZ := (bounds.MinZ + bounds.MaxZ) / 2
	
	x := point.X - centerX
	y := point.Y - centerY
	z := point.Z - centerZ
	
	// 2. Apply rotations
	// Rotate around X axis (pitch)
	radX := v.camera.RotationX * math.Pi / 180
	y1 := y*math.Cos(radX) - z*math.Sin(radX)
	z1 := y*math.Sin(radX) + z*math.Cos(radX)
	y = y1
	z = z1
	
	// Rotate around Y axis (yaw)
	radY := v.camera.RotationY * math.Pi / 180
	x1 := x*math.Cos(radY) + z*math.Sin(radY)
	z1 = -x*math.Sin(radY) + z*math.Cos(radY)
	x = x1
	z = z1
	
	// 3. Apply perspective projection
	distance := v.camera.Distance
	scale := v.camera.Zoom * 100 / (distance + z)
	
	// 4. Convert to screen coordinates
	screenX := float32(x*scale + float64(v.width)/2 + v.camera.PanX)
	screenY := float32(-y*scale + float64(v.height)/2 + v.camera.PanY) // Flip Y axis
	
	return Point2D{X: screenX, Y: screenY}
}

// fitToView adjusts camera to fit the entire model
func (v *GCodeViewer) fitToView() {
	if v.model == nil {
		return
	}
	
	bounds := v.model.Bounds
	
	// Calculate model size
	sizeX := bounds.MaxX - bounds.MinX
	sizeY := bounds.MaxY - bounds.MinY
	sizeZ := bounds.MaxZ - bounds.MinZ
	maxSize := math.Max(math.Max(sizeX, sizeY), sizeZ)
	
	// Adjust zoom and distance
	v.camera.Zoom = 1.0
	v.camera.Distance = maxSize * 2
	v.camera.PanX = 0
	v.camera.PanY = 0
}

// dimColor reduces the brightness of a color
func (r *gcodeViewerRenderer) dimColor(c color.Color, factor float64) color.Color {
	r, g, b, a := c.RGBA()
	return color.NRGBA{
		R: uint8(float64(r>>8) * factor),
		G: uint8(float64(g>>8) * factor),
		B: uint8(float64(b>>8) * factor),
		A: uint8(a >> 8),
	}
}

// SetCurrentLayer sets the currently visible layer
func (v *GCodeViewer) SetCurrentLayer(layer int) {
	if v.model == nil || layer < 0 || layer >= len(v.model.Layers) {
		return
	}
	v.currentLayer = layer
	v.Refresh()
}

// SetCurrentLine sets the current line for progress visualization
func (v *GCodeViewer) SetCurrentLine(line int) {
	if v.model == nil || line < 0 {
		return
	}
	v.currentLine = line
	v.Refresh()
}

// SetVisibleLayers sets which layers to display
func (v *GCodeViewer) SetVisibleLayers(layers []int) {
	v.visibleLayers = make([]int, len(layers))
	copy(v.visibleLayers, layers)
	v.Refresh()
}

// ShowLayersUpTo shows all layers up to the specified layer
func (v *GCodeViewer) ShowLayersUpTo(maxLayer int) {
	if v.model == nil {
		return
	}
	
	v.visibleLayers = make([]int, 0)
	for i := 0; i <= maxLayer && i < len(v.model.Layers); i++ {
		v.visibleLayers = append(v.visibleLayers, i)
	}
	v.Refresh()
}

// Rotate rotates the camera view
func (v *GCodeViewer) Rotate(deltaX, deltaY float64) {
	v.camera.RotationY += deltaX * 0.5
	v.camera.RotationX += deltaY * 0.5
	
	// Clamp rotation
	v.camera.RotationX = math.Max(-90, math.Min(90, v.camera.RotationX))
	
	v.Refresh()
}

// Zoom adjusts the zoom level
func (v *GCodeViewer) Zoom(delta float64) {
	v.camera.Zoom *= (1.0 + delta*0.1)
	v.camera.Zoom = math.Max(0.1, math.Min(10.0, v.camera.Zoom))
	v.Refresh()
}

// Pan adjusts the pan offset
func (v *GCodeViewer) Pan(deltaX, deltaY float64) {
	v.camera.PanX += deltaX
	v.camera.PanY += deltaY
	v.Refresh()
}

// ResetView resets the camera to default position
func (v *GCodeViewer) ResetView() {
	v.camera.RotationX = -30
	v.camera.RotationY = 45
	v.camera.RotationZ = 0
	v.fitToView()
	v.Refresh()
}

// ToggleTravelMoves toggles display of travel moves
func (v *GCodeViewer) ToggleTravelMoves() {
	v.showTravelMoves = !v.showTravelMoves
	v.Refresh()
}

// ToggleSupports toggles display of support material
func (v *GCodeViewer) ToggleSupports() {
	v.showSupports = !v.showSupports
	v.Refresh()
} 