// +build ignore

package main

import (
	"fmt"
	"log"
	"time"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Test WebSocket stability and reconnection
func main() {
	// Create test app
	testApp := app.New()
	testApp.Settings().SetTheme(&InnovateTheme{})
	
	window := testApp.NewWindow("WebSocket Stability Test")
	window.Resize(fyne.NewSize(800, 600))
	
	// Create backend client
	backend := NewBackendClient("localhost:8080")
	backend.SetAuthToken("test-token") // Set test token
	
	// Create connection status card
	connectionCard := NewConnectionStatusCard(backend)
	
	// Test log
	logEntry := widget.NewMultiLineEntry()
	logEntry.SetText("WebSocket Test Log:\n")
	logEntry.Resize(fyne.NewSize(750, 300))
	
	// Add log function
	addLog := func(msg string) {
		timestamp := time.Now().Format("15:04:05")
		logEntry.SetText(logEntry.Text + fmt.Sprintf("[%s] %s\n", timestamp, msg))
		logEntry.CursorRow = len(logEntry.Text)
	}
	
	// Connection stats
	statsLabel := widget.NewLabel("Stats will appear here...")
	
	// Update stats periodically
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			state := backend.GetWebSocketState()
			queue := backend.GetWebSocketQueueSize()
			attempts := backend.GetWebSocketReconnectAttempts()
			
			stats := fmt.Sprintf(
				"State: %s | Queue: %d messages | Reconnect Attempts: %d",
				state, queue, attempts,
			)
			statsLabel.SetText(stats)
		}
	}()
	
	// Set up message handler
	backend.wsManager.SetCallbacks(
		func(state ConnectionState) {
			addLog(fmt.Sprintf("Connection state changed: %s", ConnectionStateNames[state]))
		},
		func(message []byte) {
			addLog(fmt.Sprintf("Received message: %s", string(message)))
		},
		func(err error) {
			addLog(fmt.Sprintf("WebSocket error: %v", err))
		},
	)
	
	// Test controls
	connectBtn := widget.NewButton("Connect", func() {
		addLog("Attempting to connect...")
		err := backend.ConnectWebSocket()
		if err != nil {
			addLog(fmt.Sprintf("Connect failed: %v", err))
		} else {
			addLog("Connect initiated")
		}
	})
	
	disconnectBtn := widget.NewButton("Disconnect", func() {
		addLog("Disconnecting...")
		backend.CloseWebSocket()
		addLog("Disconnected")
	})
	
	// Simulate network issues
	simulateDropBtn := widget.NewButton("Simulate Network Drop", func() {
		if backend.IsWebSocketConnected() {
			addLog("Simulating network drop...")
			// Force close the connection
			backend.wsManager.conn.Close()
			addLog("Connection forcibly closed - reconnect should start automatically")
		} else {
			addLog("Not connected - connect first")
		}
	})
	
	// Send test message
	sendBtn := widget.NewButton("Send Test Message", func() {
		msg := map[string]interface{}{
			"type":    "test",
			"content": "Hello from WebSocket test",
			"time":    time.Now().Format("15:04:05"),
		}
		
		err := backend.SendWebSocketMessage(msg)
		if err != nil {
			addLog(fmt.Sprintf("Send failed: %v", err))
		} else {
			addLog("Test message sent (or queued)")
		}
	})
	
	// Toggle auto-reconnect
	autoReconnectCheck := widget.NewCheck("Auto-Reconnect Enabled", func(checked bool) {
		backend.EnableWebSocketReconnect(checked)
		addLog(fmt.Sprintf("Auto-reconnect: %v", checked))
	})
	autoReconnectCheck.SetChecked(true)
	
	// Fill message queue
	fillQueueBtn := widget.NewButton("Fill Message Queue", func() {
		addLog("Filling message queue with 10 messages...")
		for i := 0; i < 10; i++ {
			msg := map[string]interface{}{
				"type":    "queued",
				"index":   i + 1,
				"content": fmt.Sprintf("Queued message %d", i+1),
			}
			backend.SendWebSocketMessage(msg)
		}
		addLog(fmt.Sprintf("Queue size: %d", backend.GetWebSocketQueueSize()))
	})
	
	// Control buttons
	controls := container.NewGridWithColumns(3,
		connectBtn,
		disconnectBtn,
		simulateDropBtn,
	)
	
	controls2 := container.NewGridWithColumns(3,
		sendBtn,
		fillQueueBtn,
		autoReconnectCheck,
	)
	
	// Layout
	content := container.NewVBox(
		connectionCard.GetCard(),
		widget.NewCard("Connection Stats", "", statsLabel),
		widget.NewCard("Test Controls", "", container.NewVBox(
			controls,
			controls2,
		)),
		widget.NewCard("Test Log", "", container.NewScroll(logEntry)),
	)
	
	window.SetContent(content)
	
	// Instructions
	log.Println("WebSocket Stability Test")
	log.Println("========================")
	log.Println("This demo tests:")
	log.Println("1. Automatic reconnection with exponential backoff")
	log.Println("2. Message queuing during disconnection")
	log.Println("3. Connection state tracking")
	log.Println("4. Network failure simulation")
	log.Println("")
	log.Println("Try these scenarios:")
	log.Println("- Connect and disconnect manually")
	log.Println("- Simulate network drop and watch auto-reconnect")
	log.Println("- Send messages while disconnected (they'll be queued)")
	log.Println("- Fill the queue and reconnect to see messages flush")
	log.Println("- Stop the backend and watch reconnection attempts")
	
	window.ShowAndRun()
} 