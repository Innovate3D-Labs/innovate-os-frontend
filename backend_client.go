package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"log"
	"github.com/gorilla/websocket"
)

// BackendClient handles communication with the Go backend API
type BackendClient struct {
	baseURL    string
	httpClient *http.Client
	wsConn     *websocket.Conn
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
	return &BackendClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ConnectWebSocket establishes WebSocket connection for real-time updates
func (c *BackendClient) ConnectWebSocket() error {
	wsURL := fmt.Sprintf("ws://%s/ws", c.baseURL)
	
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %v", err)
	}
	
	c.wsConn = conn
	return nil
}

// ListenForUpdates listens for real-time printer status updates
func (c *BackendClient) ListenForUpdates(statusChan chan<- PrinterStatus) {
	if c.wsConn == nil {
		log.Println("WebSocket connection not established")
		return
	}
	
	for {
		var status PrinterStatus
		err := c.wsConn.ReadJSON(&status)
		if err != nil {
			log.Printf("Error reading WebSocket message: %v", err)
			break
		}
		
		statusChan <- status
	}
}

// CloseWebSocket closes the WebSocket connection
func (c *BackendClient) CloseWebSocket() error {
	if c.wsConn != nil {
		return c.wsConn.Close()
	}
	return nil
}

// GetPrinterStatus retrieves current printer status via HTTP
func (c *BackendClient) GetPrinterStatus() (*PrinterStatus, error) {
	url := fmt.Sprintf("http://%s/api/printer/status", c.baseURL)
	
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get printer status: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var status PrinterStatus
	err = json.NewDecoder(resp.Body).Decode(&status)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	
	return &status, nil
}

// StartPrint starts a new print job
func (c *BackendClient) StartPrint(filename string) error {
	url := fmt.Sprintf("http://%s/api/printer/start", c.baseURL)
	
	payload := map[string]interface{}{
		"filename": filename,
	}
	
	return c.sendCommand(url, payload)
}

// PausePrint pauses the current print job
func (c *BackendClient) PausePrint() error {
	url := fmt.Sprintf("http://%s/api/printer/pause", c.baseURL)
	return c.sendCommand(url, nil)
}

// ResumePrint resumes the current print job
func (c *BackendClient) ResumePrint() error {
	url := fmt.Sprintf("http://%s/api/printer/resume", c.baseURL)
	return c.sendCommand(url, nil)
}

// StopPrint stops the current print job
func (c *BackendClient) StopPrint() error {
	url := fmt.Sprintf("http://%s/api/printer/stop", c.baseURL)
	return c.sendCommand(url, nil)
}

// EmergencyStop sends emergency stop command
func (c *BackendClient) EmergencyStop() error {
	url := fmt.Sprintf("http://%s/api/printer/emergency-stop", c.baseURL)
	return c.sendCommand(url, nil)
}

// HomeAxes homes all printer axes
func (c *BackendClient) HomeAxes() error {
	url := fmt.Sprintf("http://%s/api/printer/home", c.baseURL)
	return c.sendCommand(url, nil)
}

// MoveAxis moves a specific axis by the given amount
func (c *BackendClient) MoveAxis(axis string, distance float64) error {
	url := fmt.Sprintf("http://%s/api/printer/move", c.baseURL)
	
	payload := map[string]interface{}{
		"axis":     axis,
		"distance": distance,
	}
	
	return c.sendCommand(url, payload)
}

// SetTemperature sets the temperature for hotend or bed
func (c *BackendClient) SetTemperature(target string, temperature float64) error {
	url := fmt.Sprintf("http://%s/api/printer/temperature", c.baseURL)
	
	payload := map[string]interface{}{
		"target":      target,
		"temperature": temperature,
	}
	
	return c.sendCommand(url, payload)
}

// GetPrintJobs retrieves list of print jobs
func (c *BackendClient) GetPrintJobs() ([]PrintJob, error) {
	url := fmt.Sprintf("http://%s/api/jobs", c.baseURL)
	
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get print jobs: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var jobs []PrintJob
	err = json.NewDecoder(resp.Body).Decode(&jobs)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	
	return jobs, nil
}

// UploadFile uploads a G-code file to the printer
func (c *BackendClient) UploadFile(filename string, content []byte) error {
	url := fmt.Sprintf("http://%s/api/files/upload", c.baseURL)
	
	payload := map[string]interface{}{
		"filename": filename,
		"content":  content,
	}
	
	return c.sendCommand(url, payload)
}

// DeleteFile deletes a file from the printer
func (c *BackendClient) DeleteFile(filename string) error {
	url := fmt.Sprintf("http://%s/api/files/%s", c.baseURL, filename)
	
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	return nil
}

// sendCommand sends a command to the backend API
func (c *BackendClient) sendCommand(url string, payload interface{}) error {
	var body io.Reader
	
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %v", err)
		}
		body = bytes.NewBuffer(jsonData)
	}
	
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
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