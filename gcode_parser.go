package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// GCodeCommand represents a single G-code command
type GCodeCommand struct {
	Type        string  // G0, G1, G28, M104, etc.
	X, Y, Z     float64 // Position coordinates
	E           float64 // Extruder position
	F           float64 // Feed rate (speed)
	S           float64 // Spindle speed / Temperature
	T           int     // Tool number
	Comment     string  // Any comment after semicolon
	LineNumber  int     // Original line number
	RawLine     string  // Original raw line
	IsValid     bool    // Whether parsing was successful
}

// GCodePath represents a 3D path segment
type GCodePath struct {
	StartX, StartY, StartZ float64
	EndX, EndY, EndZ       float64
	ExtrusionAmount        float64
	Speed                  float64
	LayerIndex             int
	PathType               PathType
	LineNumber             int
}

// PathType defines the type of movement
type PathType int

const (
	PathTypeTravel PathType = iota // Non-extrusion move
	PathTypeExtrusion              // Extrusion move
	PathTypeRetraction             // Retraction/unretraction
	PathTypePerimeter              // Outer perimeter
	PathTypeInfill                 // Infill pattern
	PathTypeSupport                // Support material
)

// PathTypeNames for display
var PathTypeNames = map[PathType]string{
	PathTypeTravel:     "Travel",
	PathTypeExtrusion:  "Extrusion",
	PathTypeRetraction: "Retraction",
	PathTypePerimeter:  "Perimeter",
	PathTypeInfill:     "Infill",
	PathTypeSupport:    "Support",
}

// GCodeModel represents the complete parsed G-code
type GCodeModel struct {
	Commands     []GCodeCommand
	Paths        []GCodePath
	Layers       []GCodeLayer
	Bounds       GCodeBounds
	Metadata     GCodeMetadata
	TotalLines   int
	ParseErrors  []string
}

// GCodeLayer represents a single layer
type GCodeLayer struct {
	Index         int
	Z             float64
	StartLine     int
	EndLine       int
	Paths         []int // Indices into main Paths array
	LayerTime     float64
	FilamentUsed  float64
	BoundingBox   GCodeBounds
}

// GCodeBounds represents 3D bounding box
type GCodeBounds struct {
	MinX, MaxX float64
	MinY, MaxY float64
	MinZ, MaxZ float64
}

// GCodeMetadata contains print information
type GCodeMetadata struct {
	GeneratedBy    string
	PrintTime      float64  // Estimated print time in seconds
	FilamentUsed   float64  // Total filament used in mm
	LayerHeight    float64
	FirstLayerHeight float64
	InfillDensity  float64
	PrintSpeed     float64
	TotalLayers    int
	PrinterModel   string
	SlicerSettings map[string]string
}

// GCodeParser handles G-code parsing
type GCodeParser struct {
	currentX, currentY, currentZ float64
	currentE                     float64
	currentF                     float64
	absoluteMode                 bool
	absoluteEMode                bool
	currentLayer                 int
	layerZ                       float64
	lastExtrusionAmount          float64
}

// NewGCodeParser creates a new G-code parser
func NewGCodeParser() *GCodeParser {
	return &GCodeParser{
		absoluteMode:  true,
		absoluteEMode: true,
		currentF:      1500, // Default feed rate
	}
}

// ParseGCode parses G-code from a reader
func (p *GCodeParser) ParseGCode(reader io.Reader) (*GCodeModel, error) {
	model := &GCodeModel{
		Commands:    make([]GCodeCommand, 0),
		Paths:       make([]GCodePath, 0),
		Layers:      make([]GCodeLayer, 0),
		ParseErrors: make([]string, 0),
		Metadata: GCodeMetadata{
			SlicerSettings: make(map[string]string),
		},
		Bounds: GCodeBounds{
			MinX: math.Inf(1), MaxX: math.Inf(-1),
			MinY: math.Inf(1), MaxY: math.Inf(-1),
			MinZ: math.Inf(1), MaxZ: math.Inf(-1),
		},
	}

	scanner := bufio.NewScanner(reader)
	lineNumber := 0

	var currentLayer *GCodeLayer

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		
		if line == "" {
			continue
		}

		// Parse command
		cmd := p.parseLine(line, lineNumber)
		model.Commands = append(model.Commands, cmd)

		if !cmd.IsValid {
			continue
		}

		// Extract metadata from comments
		p.extractMetadata(&model.Metadata, cmd)

		// Process movement commands
		if cmd.Type == "G0" || cmd.Type == "G1" {
			// Calculate new position
			newX, newY, newZ := p.calculateNewPosition(cmd)
			newE := p.calculateNewE(cmd)

			// Detect layer changes
			if newZ > p.layerZ+0.01 { // New layer detected
				if currentLayer != nil {
					currentLayer.EndLine = lineNumber - 1
					model.Layers = append(model.Layers, *currentLayer)
				}

				p.currentLayer++
				p.layerZ = newZ
				currentLayer = &GCodeLayer{
					Index:     p.currentLayer,
					Z:         newZ,
					StartLine: lineNumber,
					Paths:     make([]int, 0),
					BoundingBox: GCodeBounds{
						MinX: math.Inf(1), MaxX: math.Inf(-1),
						MinY: math.Inf(1), MaxY: math.Inf(-1),
						MinZ: newZ, MaxZ: newZ,
					},
				}
			}

			// Create path segment
			path := GCodePath{
				StartX:     p.currentX,
				StartY:     p.currentY,
				StartZ:     p.currentZ,
				EndX:       newX,
				EndY:       newY,
				EndZ:       newZ,
				Speed:      p.currentF,
				LayerIndex: p.currentLayer,
				LineNumber: lineNumber,
			}

			// Determine path type and extrusion
			extrusionDiff := newE - p.currentE
			path.ExtrusionAmount = extrusionDiff

			if extrusionDiff > 0.01 {
				path.PathType = p.determinePathType(cmd, extrusionDiff)
			} else if extrusionDiff < -0.01 {
				path.PathType = PathTypeRetraction
			} else {
				path.PathType = PathTypeTravel
			}

			model.Paths = append(model.Paths, path)

			// Update current layer
			if currentLayer != nil {
				currentLayer.Paths = append(currentLayer.Paths, len(model.Paths)-1)
				currentLayer.FilamentUsed += math.Max(0, extrusionDiff)
				p.updateBounds(&currentLayer.BoundingBox, newX, newY, newZ)
			}

			// Update global bounds
			p.updateBounds(&model.Bounds, newX, newY, newZ)

			// Update position
			p.currentX = newX
			p.currentY = newY
			p.currentZ = newZ
			p.currentE = newE
		}

		// Handle other G-codes
		p.processOtherCommands(cmd)
	}

	// Finalize last layer
	if currentLayer != nil {
		currentLayer.EndLine = lineNumber
		model.Layers = append(model.Layers, *currentLayer)
	}

	model.TotalLines = lineNumber
	model.Metadata.TotalLayers = len(model.Layers)

	// Post-process metadata
	p.finalizeMetadata(&model.Metadata, model)

	return model, scanner.Err()
}

// parseLine parses a single G-code line
func (p *GCodeParser) parseLine(line string, lineNumber int) GCodeCommand {
	cmd := GCodeCommand{
		LineNumber: lineNumber,
		RawLine:    line,
		X:          math.NaN(),
		Y:          math.NaN(),
		Z:          math.NaN(),
		E:          math.NaN(),
		F:          math.NaN(),
		S:          math.NaN(),
		T:          -1,
	}

	// Split line into command and comment
	parts := strings.SplitN(line, ";", 2)
	commandPart := strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		cmd.Comment = strings.TrimSpace(parts[1])
	}

	if commandPart == "" {
		return cmd
	}

	// Extract command type (G0, G1, M104, etc.)
	fields := strings.Fields(commandPart)
	if len(fields) == 0 {
		return cmd
	}

	cmd.Type = strings.ToUpper(fields[0])

	// Parse parameters
	for _, field := range fields[1:] {
		if len(field) < 2 {
			continue
		}

		param := field[0]
		valueStr := field[1:]
		value, err := strconv.ParseFloat(valueStr, 64)

		if err != nil {
			if param == 'T' {
				// Tool number might be integer
				if intVal, intErr := strconv.Atoi(valueStr); intErr == nil {
					cmd.T = intVal
				}
			}
			continue
		}

		switch param {
		case 'X':
			cmd.X = value
		case 'Y':
			cmd.Y = value
		case 'Z':
			cmd.Z = value
		case 'E':
			cmd.E = value
		case 'F':
			cmd.F = value
		case 'S':
			cmd.S = value
		}
	}

	cmd.IsValid = true
	return cmd
}

// calculateNewPosition calculates new XYZ position based on current mode
func (p *GCodeParser) calculateNewPosition(cmd GCodeCommand) (float64, float64, float64) {
	newX, newY, newZ := p.currentX, p.currentY, p.currentZ

	if !math.IsNaN(cmd.X) {
		if p.absoluteMode {
			newX = cmd.X
		} else {
			newX = p.currentX + cmd.X
		}
	}

	if !math.IsNaN(cmd.Y) {
		if p.absoluteMode {
			newY = cmd.Y
		} else {
			newY = p.currentY + cmd.Y
		}
	}

	if !math.IsNaN(cmd.Z) {
		if p.absoluteMode {
			newZ = cmd.Z
		} else {
			newZ = p.currentZ + cmd.Z
		}
	}

	return newX, newY, newZ
}

// calculateNewE calculates new extruder position
func (p *GCodeParser) calculateNewE(cmd GCodeCommand) float64 {
	if math.IsNaN(cmd.E) {
		return p.currentE
	}

	if p.absoluteEMode {
		return cmd.E
	} else {
		return p.currentE + cmd.E
	}
}

// determinePathType determines the type of extrusion path
func (p *GCodeParser) determinePathType(cmd GCodeCommand, extrusionAmount float64) PathType {
	// Use comment hints if available
	comment := strings.ToLower(cmd.Comment)
	
	if strings.Contains(comment, "perimeter") || strings.Contains(comment, "outer") {
		return PathTypePerimeter
	}
	if strings.Contains(comment, "infill") || strings.Contains(comment, "fill") {
		return PathTypeInfill
	}
	if strings.Contains(comment, "support") {
		return PathTypeSupport
	}

	// Fall back to generic extrusion
	return PathTypeExtrusion
}

// extractMetadata extracts metadata from comments
func (p *GCodeParser) extractMetadata(metadata *GCodeMetadata, cmd GCodeCommand) {
	comment := cmd.Comment
	if comment == "" {
		return
	}

	// Common slicer metadata patterns
	patterns := map[string]*regexp.Regexp{
		"generated_by":     regexp.MustCompile(`generated by (.+)`),
		"layer_height":     regexp.MustCompile(`layer_height = ([0-9.]+)`),
		"infill_density":   regexp.MustCompile(`fill_density = ([0-9.]+)`),
		"print_speed":      regexp.MustCompile(`perimeter_speed = ([0-9.]+)`),
		"estimated_time":   regexp.MustCompile(`estimated printing time.*?([0-9]+)h ([0-9]+)m`),
		"filament_used":    regexp.MustCompile(`filament used = ([0-9.]+)mm`),
	}

	lowerComment := strings.ToLower(comment)

	// Extract generator
	if match := patterns["generated_by"].FindStringSubmatch(lowerComment); len(match) > 1 {
		metadata.GeneratedBy = strings.TrimSpace(match[1])
	}

	// Extract layer height
	if match := patterns["layer_height"].FindStringSubmatch(lowerComment); len(match) > 1 {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			metadata.LayerHeight = val
		}
	}

	// Extract infill density
	if match := patterns["infill_density"].FindStringSubmatch(lowerComment); len(match) > 1 {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			metadata.InfillDensity = val
		}
	}

	// Extract estimated time (basic pattern)
	if strings.Contains(lowerComment, "estimated") && strings.Contains(lowerComment, "time") {
		// Store in slicer settings for now
		metadata.SlicerSettings["estimated_time"] = comment
	}

	// Store any other key=value patterns
	if strings.Contains(comment, "=") {
		parts := strings.SplitN(comment, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			metadata.SlicerSettings[key] = value
		}
	}
}

// processOtherCommands handles non-movement G-codes
func (p *GCodeParser) processOtherCommands(cmd GCodeCommand) {
	switch cmd.Type {
	case "G90": // Absolute positioning
		p.absoluteMode = true
	case "G91": // Relative positioning
		p.absoluteMode = false
	case "M82": // Absolute extruder mode
		p.absoluteEMode = true
	case "M83": // Relative extruder mode
		p.absoluteEMode = false
	case "G92": // Set position
		if !math.IsNaN(cmd.E) {
			p.currentE = cmd.E
		}
	}

	// Update feed rate
	if !math.IsNaN(cmd.F) {
		p.currentF = cmd.F
	}
}

// updateBounds updates bounding box with new coordinates
func (p *GCodeParser) updateBounds(bounds *GCodeBounds, x, y, z float64) {
	if x < bounds.MinX {
		bounds.MinX = x
	}
	if x > bounds.MaxX {
		bounds.MaxX = x
	}
	if y < bounds.MinY {
		bounds.MinY = y
	}
	if y > bounds.MaxY {
		bounds.MaxY = y
	}
	if z < bounds.MinZ {
		bounds.MinZ = z
	}
	if z > bounds.MaxZ {
		bounds.MaxZ = z
	}
}

// finalizeMetadata calculates final metadata values
func (p *GCodeParser) finalizeMetadata(metadata *GCodeMetadata, model *GCodeModel) {
	// Calculate total filament used
	totalFilament := 0.0
	for _, path := range model.Paths {
		if path.ExtrusionAmount > 0 {
			totalFilament += path.ExtrusionAmount
		}
	}
	metadata.FilamentUsed = totalFilament

	// Estimate print time based on path speeds and distances
	totalTime := 0.0
	for _, path := range model.Paths {
		distance := math.Sqrt(
			math.Pow(path.EndX-path.StartX, 2) +
				math.Pow(path.EndY-path.StartY, 2) +
				math.Pow(path.EndZ-path.StartZ, 2),
		)
		if path.Speed > 0 {
			totalTime += distance / (path.Speed / 60.0) // Convert mm/min to mm/s
		}
	}
	metadata.PrintTime = totalTime

	// Set first layer height from first layer if available
	if len(model.Layers) > 0 {
		metadata.FirstLayerHeight = model.Layers[0].Z
	}
} 