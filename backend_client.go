package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"log"
	"github.com/gorilla/websocket"
)

// BackendClient handles communication with the Go backend API
type BackendClient struct {
	baseURL      string
	httpClient   *http.Client
	authToken    string
	wsManager    *WebSocketManager
	
	// Connection callbacks
	onConnectionChange func(bool)
}

// PrinterStatus represents the real-time status from the printer
type PrinterStatus struct {
	Status        string  `json:"status"`
	Temperature   float64 `json:"temperature"`
	BedTemp       float64 `json:"bed_temperature"`
	Progress      float64 `json:"progress"`
	CurrentLayer  int     `json:"current_layer"`
	TotalLayers   int     `json:"total_layers"`
	PositionX     float64 `json:"position_x"`
	PositionY     float64 `json:"position_y"`
	PositionZ     float64 `json:"position_z"`
	EstimatedTime int     `json:"estimated_time"`
	IsConnected   bool    `json:"is_connected"`
}

// PrintJob represents a print job from the backend
type PrintJob struct {
	ID          int    `json:"id"`
	Filename    string `json:"filename"`
	Status      string `json:"status"`
	Progress    float64 `json:"progress"`
	CreatedAt   string `json:"created_at"`
	CompletedAt string `json:"completed_at"`
}

// NewBackendClient creates a new client for backend communication
func NewBackendClient(baseURL string) *BackendClient {
	wsURL := fmt.Sprintf("ws://%s/ws", baseURL)
	
	client := &BackendClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		wsManager: NewWebSocketManager(wsURL),
	}
	
	// Set up WebSocket callbacks
	client.wsManager.SetCallbacks(
		func(state ConnectionState) {
			// Connection state changed
			if client.onConnectionChange != nil {
				client.onConnectionChange(state == StateConnected)
			}
		},
		nil, // Message handler will be set by ListenForUpdates
		func(err error) {
			log.Printf("WebSocket error: %v", err)
		},
	)
	
	return client
}

// SetAuthToken sets the authentication token for API requests
func (c *BackendClient) SetAuthToken(token string) {
	c.authToken = token
	c.wsManager.SetAuthToken(token)
}

// SetConnectionChangeCallback sets callback for connection state changes
func (c *BackendClient) SetConnectionChangeCallback(callback func(bool)) {
	c.onConnectionChange = callback
}

// ConnectWebSocket establishes WebSocket connection for real-time updates
func (c *BackendClient) ConnectWebSocket() error {
	return c.wsManager.Connect()
}

// CloseWebSocket closes the WebSocket connection
func (c *BackendClient) CloseWebSocket() error {
	c.wsManager.Disconnect()
	return nil
}

// ListenForUpdates listens for real-time printer status updates
func (c *BackendClient) ListenForUpdates(statusChan chan<- PrinterStatus) {
	c.wsManager.SetCallbacks(
		func(state ConnectionState) {
			// Keep existing state callback
			if c.onConnectionChange != nil {
				c.onConnectionChange(state == StateConnected)
			}
		},
		func(message []byte) {
			// Handle incoming messages
			var status PrinterStatus
			if err := json.Unmarshal(message, &status); err != nil {
				log.Printf("Error parsing WebSocket message: %v", err)
				return
			}
			
			select {
			case statusChan <- status:
			default:
				// Channel full, skip update
			}
		},
		func(err error) {
			log.Printf("WebSocket error: %v", err)
		},
	)
}

// GetWebSocketState returns the WebSocket connection state
func (c *BackendClient) GetWebSocketState() string {
	return c.wsManager.GetStateString()
}

// IsWebSocketConnected returns true if WebSocket is connected
func (c *BackendClient) IsWebSocketConnected() bool {
	return c.wsManager.IsConnected()
}

// GetWebSocketQueueSize returns number of queued messages
func (c *BackendClient) GetWebSocketQueueSize() int {
	return c.wsManager.GetQueueSize()
}

// GetWebSocketReconnectAttempts returns reconnect attempt count
func (c *BackendClient) GetWebSocketReconnectAttempts() int {
	return c.wsManager.GetReconnectAttempts()
}

// EnableWebSocketReconnect enables/disables auto-reconnect
func (c *BackendClient) EnableWebSocketReconnect(enable bool) {
	c.wsManager.EnableReconnect(enable)
}

// SendWebSocketMessage sends a message through WebSocket
func (c *BackendClient) SendWebSocketMessage(message interface{}) error {
	return c.wsManager.Send(message)
}

// GetPrinterStatus retrieves current printer status via HTTP
func (c *BackendClient) GetPrinterStatus() (*PrinterStatus, error) {
	resp, err := c.makeRequest("GET", "/api/printer/status", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication required")
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get printer status: %s", resp.Status)
	}
	
	var status PrinterStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	
	return &status, nil
}

// StartPrint starts a print job
func (c *BackendClient) StartPrint(filename string) error {
	command := map[string]interface{}{
		"filename": filename,
	}
	
	jsonData, err := json.Marshal(command)
	if err != nil {
		return err
	}
	
	resp, err := c.makeRequest("POST", "/api/printer/print/start", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication required")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to start print: %s", resp.Status)
	}
	
	return nil
}

// PausePrint pauses the current print
func (c *BackendClient) PausePrint() error {
	resp, err := c.makeRequest("POST", "/api/printer/print/pause", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication required")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to pause print: %s", resp.Status)
	}
	
	return nil
}

// ResumePrint resumes the current print
func (c *BackendClient) ResumePrint() error {
	resp, err := c.makeRequest("POST", "/api/printer/print/resume", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication required")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to resume print: %s", resp.Status)
	}
	
	return nil
}

// CancelPrint cancels the current print
func (c *BackendClient) CancelPrint() error {
	resp, err := c.makeRequest("POST", "/api/printer/print/cancel", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication required")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to cancel print: %s", resp.Status)
	}
	
	return nil
}

// CancelPrintJob cancels a specific print job by filename
func (c *BackendClient) CancelPrintJob(filename string) error {
	command := map[string]interface{}{
		"filename": filename,
	}
	
	jsonData, err := json.Marshal(command)
	if err != nil {
		return err
	}
	
	resp, err := c.makeRequest("POST", "/api/printer/print/cancel", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication required")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to cancel print job: %s", resp.Status)
	}
	
	return nil
}

// EmergencyStop performs an emergency stop
func (c *BackendClient) EmergencyStop() error {
	resp, err := c.makeRequest("POST", "/api/printer/emergency-stop", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication required")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to perform emergency stop: %s", resp.Status)
	}
	
	return nil
}

// HomeAll homes all axes
func (c *BackendClient) HomeAll() error {
	command := map[string]interface{}{
		"command": "home_all",
	}
	
	jsonData, err := json.Marshal(command)
	if err != nil {
		return err
	}
	
	resp, err := c.makeRequest("POST", "/api/printer/home", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication required")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to home printer: %s", resp.Status)
	}
	
	return nil
}

// MoveAxis moves the printer axis
func (c *BackendClient) MoveAxis(axis string, distance float64) error {
	command := map[string]interface{}{
		"command": "move",
		"axis":    axis,
		"distance": distance,
	}
	
	jsonData, err := json.Marshal(command)
	if err != nil {
		return err
	}
	
	resp, err := c.makeRequest("POST", "/api/printer/move", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication required")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to move axis: %s", resp.Status)
	}
	
	return nil
}

// SetTemperature sets the target temperature
func (c *BackendClient) SetTemperature(heater string, temperature float64) error {
	command := map[string]interface{}{
		"heater":      heater,
		"temperature": temperature,
	}
	
	jsonData, err := json.Marshal(command)
	if err != nil {
		return err
	}
	
	resp, err := c.makeRequest("POST", "/api/printer/temperature", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication required")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to set temperature: %s", resp.Status)
	}
	
	return nil
}

// GetPrintJobs retrieves list of print jobs
func (c *BackendClient) GetPrintJobs() ([]PrintJob, error) {
	resp, err := c.makeRequest("GET", "/api/print-jobs", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication required")
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get print jobs: %s", resp.Status)
	}
	
	var jobs []PrintJob
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, err
	}
	
	return jobs, nil
}

// UploadFile uploads a G-code file
func (c *BackendClient) UploadFile(filename string, data []byte) error {
	// Create multipart form data would go here
	// For now, simplified version
	return fmt.Errorf("upload not implemented yet")
}

// DeletePrintJob deletes a print job
func (c *BackendClient) DeletePrintJob(filename string) error {
	endpoint := fmt.Sprintf("/api/print-jobs/%s", filename)
	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication required")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete print job: %s", resp.Status)
	}
	
	return nil
}

// GetSystemLogs retrieves system logs from the backend
func (c *BackendClient) GetSystemLogs() ([]string, error) {
	url := fmt.Sprintf("http://%s/api/logs", c.baseURL)
	
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get system logs: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var logs []string
	err = json.NewDecoder(resp.Body).Decode(&logs)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	
	return logs, nil
} 

// makeRequest is a helper function to make authenticated HTTP requests
func (c *BackendClient) makeRequest(method, endpoint string, body io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("http://%s%s", c.baseURL, endpoint)
	
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	
	// Add auth header if token is available
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	return c.httpClient.Do(req)
}

// StartPrinterDiscovery starts the printer discovery process
func (c *BackendClient) StartPrinterDiscovery() error {
	resp, err := c.makeRequest("POST", "/api/serial/discover", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to start discovery: %s", resp.Status)
	}
	
	return nil
}

// DiscoveryStatus represents the discovery status response
type DiscoveryStatus struct {
	IsScanning bool                `json:"is_scanning"`
	Discovered []DiscoveredPrinter `json:"discovered"`
	Count      int                 `json:"count"`
}

// DiscoveredPrinter represents a discovered printer from the backend
type DiscoveredPrinter struct {
	Port         string                 `json:"port"`
	Name         string                 `json:"name"`
	Firmware     string                 `json:"firmware"`
	MachineType  string                 `json:"machine_type"`
	BaudRate     int                    `json:"baud_rate"`
	IsCompatible bool                   `json:"is_compatible"`
	DiscoveredAt time.Time              `json:"discovered_at"`
	Identity     *PrinterIdentity       `json:"identity"`
	Manufacturer map[string]string      `json:"manufacturer,omitempty"`
}

// PrinterIdentity represents the unique identity of a printer
type PrinterIdentity struct {
	ID           string `json:"id"`
	SerialNumber string `json:"serial_number"`
	UUID         string `json:"uuid"`
	Fingerprint  string `json:"fingerprint"`
}

// GetDiscoveryStatus gets the current discovery status
func (c *BackendClient) GetDiscoveryStatus() (*DiscoveryStatus, error) {
	resp, err := c.makeRequest("GET", "/api/serial/discovery/status", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var result struct {
		Data DiscoveryStatus `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return &result.Data, nil
}

// ConnectPrinter connects to a discovered printer
func (c *BackendClient) ConnectPrinter(printer DiscoveredPrinter) error {
	data := map[string]interface{}{
		"port":         printer.Port,
		"baud_rate":    printer.BaudRate,
		"identity":     printer.Identity,
		"manufacturer": printer.Manufacturer,
	}
	
	resp, err := c.makeRequest("POST", "/api/serial/connect", bytes.NewBuffer(json.RawMessage(data)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to connect: %s", resp.Status)
	}
	
	return nil
} 