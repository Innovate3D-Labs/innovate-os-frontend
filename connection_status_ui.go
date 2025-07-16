package main

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ConnectionStatusUI displays the WebSocket connection status
type ConnectionStatusUI struct {
	backend          *BackendClient
	
	// UI components
	statusIcon       *canvas.Circle
	statusLabel      *widget.Label
	detailsLabel     *widget.Label
	reconnectButton  *widget.Button
	content          *fyne.Container
	
	// Animation
	pulseStop        chan bool
}

// NewConnectionStatusUI creates a new connection status indicator
func NewConnectionStatusUI(backend *BackendClient) *ConnectionStatusUI {
	ui := &ConnectionStatusUI{
		backend:     backend,
		statusIcon:  canvas.NewCircle(color.NRGBA{R: 200, G: 200, B: 200, A: 255}),
		statusLabel: widget.NewLabel("Disconnected"),
		detailsLabel: widget.NewLabel(""),
		pulseStop:   make(chan bool, 1),
	}
	
	ui.statusIcon.Resize(fyne.NewSize(12, 12))
	ui.statusLabel.TextStyle = fyne.TextStyle{Bold: true}
	
	ui.reconnectButton = widget.NewButton("Reconnect", func() {
		ui.backend.EnableWebSocketReconnect(true)
		ui.backend.ConnectWebSocket()
	})
	ui.reconnectButton.Hide()
	
	ui.createLayout()
	ui.startMonitoring()
	
	return ui
}

// createLayout creates the UI layout
func (ui *ConnectionStatusUI) createLayout() {
	// Icon and status in horizontal layout
	statusRow := container.NewHBox(
		ui.statusIcon,
		ui.statusLabel,
		ui.detailsLabel,
		ui.reconnectButton,
	)
	
	ui.content = statusRow
}

// GetContent returns the UI content
func (ui *ConnectionStatusUI) GetContent() *fyne.Container {
	return ui.content
}

// startMonitoring starts monitoring connection status
func (ui *ConnectionStatusUI) startMonitoring() {
	// Set up connection change callback
	ui.backend.SetConnectionChangeCallback(func(connected bool) {
		ui.updateStatus()
	})
	
	// Start periodic status updates
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			ui.updateStatus()
		}
	}()
	
	// Initial update
	ui.updateStatus()
}

// updateStatus updates the connection status display
func (ui *ConnectionStatusUI) updateStatus() {
	state := ui.backend.GetWebSocketState()
	
	switch state {
	case "Connected":
		ui.setConnected()
		
	case "Connecting":
		ui.setConnecting()
		
	case "Reconnecting":
		ui.setReconnecting()
		
	case "Disconnected":
		ui.setDisconnected()
	}
	
	// Update details
	ui.updateDetails()
}

// setConnected sets the UI to connected state
func (ui *ConnectionStatusUI) setConnected() {
	ui.statusIcon.FillColor = color.NRGBA{R: 52, G: 199, B: 89, A: 255} // Green
	ui.statusLabel.SetText("Connected")
	ui.reconnectButton.Hide()
	ui.stopPulse()
	ui.statusIcon.Refresh()
}

// setConnecting sets the UI to connecting state
func (ui *ConnectionStatusUI) setConnecting() {
	ui.statusIcon.FillColor = color.NRGBA{R: 255, G: 149, B: 0, A: 255} // Orange
	ui.statusLabel.SetText("Connecting...")
	ui.reconnectButton.Hide()
	ui.startPulse()
	ui.statusIcon.Refresh()
}

// setReconnecting sets the UI to reconnecting state
func (ui *ConnectionStatusUI) setReconnecting() {
	ui.statusIcon.FillColor = color.NRGBA{R: 255, G: 149, B: 0, A: 255} // Orange
	ui.statusLabel.SetText("Reconnecting...")
	ui.reconnectButton.Hide()
	ui.startPulse()
	ui.statusIcon.Refresh()
}

// setDisconnected sets the UI to disconnected state
func (ui *ConnectionStatusUI) setDisconnected() {
	ui.statusIcon.FillColor = color.NRGBA{R: 255, G: 69, B: 58, A: 255} // Red
	ui.statusLabel.SetText("Disconnected")
	ui.reconnectButton.Show()
	ui.stopPulse()
	ui.statusIcon.Refresh()
}

// updateDetails updates the connection details
func (ui *ConnectionStatusUI) updateDetails() {
	queueSize := ui.backend.GetWebSocketQueueSize()
	attempts := ui.backend.GetWebSocketReconnectAttempts()
	
	details := ""
	
	if queueSize > 0 {
		details += fmt.Sprintf("Queue: %d ", queueSize)
	}
	
	if attempts > 0 {
		details += fmt.Sprintf("Attempts: %d", attempts)
	}
	
	ui.detailsLabel.SetText(details)
}

// startPulse starts the pulsing animation
func (ui *ConnectionStatusUI) startPulse() {
	ui.stopPulse()
	
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		
		bright := true
		originalColor := ui.statusIcon.FillColor
		
		for {
			select {
			case <-ui.pulseStop:
				ui.statusIcon.FillColor = originalColor
				ui.statusIcon.Refresh()
				return
				
			case <-ticker.C:
				if bright {
					// Make brighter
					ui.statusIcon.FillColor = color.NRGBA{
						R: min(255, originalColor.(color.NRGBA).R+50),
						G: min(255, originalColor.(color.NRGBA).G+50),
						B: min(255, originalColor.(color.NRGBA).B+50),
						A: 255,
					}
				} else {
					// Restore original
					ui.statusIcon.FillColor = originalColor
				}
				bright = !bright
				ui.statusIcon.Refresh()
			}
		}
	}()
}

// stopPulse stops the pulsing animation
func (ui *ConnectionStatusUI) stopPulse() {
	select {
	case ui.pulseStop <- true:
	default:
	}
}

// min returns the minimum of two values
func min(a, b uint8) uint8 {
	if a < b {
		return a
	}
	return b
}

// ConnectionStatusCard creates a card widget with connection status
type ConnectionStatusCard struct {
	card      *widget.Card
	statusUI  *ConnectionStatusUI
	backend   *BackendClient
}

// NewConnectionStatusCard creates a new connection status card
func NewConnectionStatusCard(backend *BackendClient) *ConnectionStatusCard {
	statusUI := NewConnectionStatusUI(backend)
	
	card := &ConnectionStatusCard{
		statusUI: statusUI,
		backend:  backend,
	}
	
	// Create expandable card with details
	content := container.NewVBox(
		statusUI.GetContent(),
		widget.NewSeparator(),
		card.createDetailsSection(),
	)
	
	card.card = widget.NewCard("Connection Status", "", content)
	
	return card
}

// createDetailsSection creates the expandable details section
func (c *ConnectionStatusCard) createDetailsSection() *fyne.Container {
	showDetails := false
	
	// Details content
	wsStateLabel := widget.NewLabel("")
	queueLabel := widget.NewLabel("")
	attemptsLabel := widget.NewLabel("")
	
	detailsContent := container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("WebSocket State:"),
			wsStateLabel,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("Message Queue:"),
			queueLabel,
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("Reconnect Attempts:"),
			attemptsLabel,
		),
	)
	detailsContent.Hide()
	
	// Toggle button
	toggleButton := widget.NewButton("Show Details ▼", func() {})
	toggleButton.OnTapped = func() {
		showDetails = !showDetails
		if showDetails {
			toggleButton.SetText("Hide Details ▲")
			detailsContent.Show()
			// Update details
			wsStateLabel.SetText(c.backend.GetWebSocketState())
			queueLabel.SetText(fmt.Sprintf("%d messages", c.backend.GetWebSocketQueueSize()))
			attemptsLabel.SetText(fmt.Sprintf("%d", c.backend.GetWebSocketReconnectAttempts()))
		} else {
			toggleButton.SetText("Show Details ▼")
			detailsContent.Hide()
		}
	}
	
	return container.NewVBox(
		toggleButton,
		detailsContent,
	)
}

// GetCard returns the card widget
func (c *ConnectionStatusCard) GetCard() *widget.Card {
	return c.card
}

// CreateCompactStatusIndicator creates a compact status indicator for toolbar
func CreateCompactStatusIndicator(backend *BackendClient) *fyne.Container {
	icon := canvas.NewCircle(color.NRGBA{R: 200, G: 200, B: 200, A: 255})
	icon.Resize(fyne.NewSize(8, 8))
	
	label := widget.NewLabel("Offline")
	label.TextStyle = fyne.TextStyle{Monospace: true}
	
	// Update function
	update := func() {
		state := backend.GetWebSocketState()
		switch state {
		case "Connected":
			icon.FillColor = color.NRGBA{R: 52, G: 199, B: 89, A: 255}
			label.SetText("Online ")
		case "Connecting", "Reconnecting":
			icon.FillColor = color.NRGBA{R: 255, G: 149, B: 0, A: 255}
			label.SetText("Connecting")
		default:
			icon.FillColor = color.NRGBA{R: 255, G: 69, B: 58, A: 255}
			label.SetText("Offline")
		}
		icon.Refresh()
	}
	
	// Set up monitoring
	backend.SetConnectionChangeCallback(func(connected bool) {
		update()
	})
	
	// Initial update
	update()
	
	return container.NewHBox(icon, label)
} 