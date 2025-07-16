package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ConnectionState represents the WebSocket connection state
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
)

// ConnectionStateNames for display
var ConnectionStateNames = map[ConnectionState]string{
	StateDisconnected: "Disconnected",
	StateConnecting:   "Connecting",
	StateConnected:    "Connected",
	StateReconnecting: "Reconnecting",
}

// WebSocketManager handles WebSocket connections with automatic reconnection
type WebSocketManager struct {
	url               string
	conn              *websocket.Conn
	authToken         string
	
	// Connection state
	state             ConnectionState
	stateMu           sync.RWMutex
	lastError         error
	reconnectAttempts int
	maxReconnectDelay time.Duration
	
	// Message handling
	messageQueue      []interface{}
	queueMu           sync.Mutex
	maxQueueSize      int
	
	// Callbacks
	onStateChange     func(ConnectionState)
	onMessage         func([]byte)
	onError           func(error)
	
	// Control channels
	done              chan struct{}
	reconnectChan     chan struct{}
	sendChan          chan interface{}
	
	// Settings
	pingInterval      time.Duration
	pongTimeout       time.Duration
	reconnectEnabled  bool
}

// NewWebSocketManager creates a new WebSocket manager
func NewWebSocketManager(url string) *WebSocketManager {
	return &WebSocketManager{
		url:               url,
		state:             StateDisconnected,
		maxReconnectDelay: 2 * time.Minute,
		maxQueueSize:      1000,
		pingInterval:      30 * time.Second,
		pongTimeout:       10 * time.Second,
		reconnectEnabled:  true,
		done:              make(chan struct{}),
		reconnectChan:     make(chan struct{}, 1),
		sendChan:          make(chan interface{}, 100),
		messageQueue:      make([]interface{}, 0),
	}
}

// SetAuthToken sets the authentication token
func (wsm *WebSocketManager) SetAuthToken(token string) {
	wsm.authToken = token
}

// SetCallbacks sets the callback functions
func (wsm *WebSocketManager) SetCallbacks(
	onStateChange func(ConnectionState),
	onMessage func([]byte),
	onError func(error),
) {
	wsm.onStateChange = onStateChange
	wsm.onMessage = onMessage
	wsm.onError = onError
}

// Connect establishes the WebSocket connection
func (wsm *WebSocketManager) Connect() error {
	wsm.updateState(StateConnecting)
	
	headers := make(http.Header)
	if wsm.authToken != "" {
		headers.Set("Authorization", "Bearer "+wsm.authToken)
	}
	
	conn, resp, err := websocket.DefaultDialer.Dial(wsm.url, headers)
	if err != nil {
		wsm.lastError = err
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			wsm.reconnectEnabled = false // Don't reconnect on auth failure
			wsm.updateState(StateDisconnected)
			return fmt.Errorf("authentication failed: unauthorized")
		}
		wsm.updateState(StateDisconnected)
		return err
	}
	
	wsm.conn = conn
	wsm.reconnectAttempts = 0
	wsm.updateState(StateConnected)
	
	// Start goroutines
	go wsm.readLoop()
	go wsm.writeLoop()
	go wsm.pingLoop()
	
	// Send queued messages
	wsm.flushQueue()
	
	return nil
}

// Disconnect closes the WebSocket connection
func (wsm *WebSocketManager) Disconnect() {
	wsm.reconnectEnabled = false
	close(wsm.done)
	
	if wsm.conn != nil {
		wsm.conn.Close()
	}
	
	wsm.updateState(StateDisconnected)
}

// Send sends a message through WebSocket
func (wsm *WebSocketManager) Send(message interface{}) error {
	wsm.stateMu.RLock()
	state := wsm.state
	wsm.stateMu.RUnlock()
	
	if state == StateConnected {
		select {
		case wsm.sendChan <- message:
			return nil
		case <-time.After(5 * time.Second):
			return fmt.Errorf("send timeout")
		}
	} else {
		// Queue message if not connected
		wsm.queueMessage(message)
		if state == StateDisconnected && wsm.reconnectEnabled {
			wsm.triggerReconnect()
		}
		return fmt.Errorf("not connected, message queued")
	}
}

// GetState returns the current connection state
func (wsm *WebSocketManager) GetState() ConnectionState {
	wsm.stateMu.RLock()
	defer wsm.stateMu.RUnlock()
	return wsm.state
}

// GetStateString returns the current state as string
func (wsm *WebSocketManager) GetStateString() string {
	state := wsm.GetState()
	return ConnectionStateNames[state]
}

// IsConnected returns true if WebSocket is connected
func (wsm *WebSocketManager) IsConnected() bool {
	return wsm.GetState() == StateConnected
}

// updateState updates the connection state and notifies callback
func (wsm *WebSocketManager) updateState(state ConnectionState) {
	wsm.stateMu.Lock()
	oldState := wsm.state
	wsm.state = state
	wsm.stateMu.Unlock()
	
	if oldState != state && wsm.onStateChange != nil {
		wsm.onStateChange(state)
	}
}

// readLoop handles incoming messages
func (wsm *WebSocketManager) readLoop() {
	defer func() {
		wsm.conn.Close()
		wsm.handleDisconnect()
	}()
	
	for {
		select {
		case <-wsm.done:
			return
		default:
			_, message, err := wsm.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket read error: %v", err)
				}
				return
			}
			
			if wsm.onMessage != nil {
				wsm.onMessage(message)
			}
		}
	}
}

// writeLoop handles outgoing messages
func (wsm *WebSocketManager) writeLoop() {
	ticker := time.NewTicker(wsm.pingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-wsm.done:
			return
			
		case message := <-wsm.sendChan:
			wsm.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			
			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("Failed to marshal message: %v", err)
				continue
			}
			
			if err := wsm.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
			
		case <-ticker.C:
			wsm.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := wsm.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// pingLoop sends periodic pings to keep connection alive
func (wsm *WebSocketManager) pingLoop() {
	wsm.conn.SetReadDeadline(time.Now().Add(wsm.pongTimeout))
	wsm.conn.SetPongHandler(func(string) error {
		wsm.conn.SetReadDeadline(time.Now().Add(wsm.pongTimeout))
		return nil
	})
}

// handleDisconnect handles disconnection and triggers reconnect
func (wsm *WebSocketManager) handleDisconnect() {
	wsm.updateState(StateDisconnected)
	
	if wsm.reconnectEnabled {
		wsm.triggerReconnect()
	}
}

// triggerReconnect triggers a reconnection attempt
func (wsm *WebSocketManager) triggerReconnect() {
	select {
	case wsm.reconnectChan <- struct{}{}:
		go wsm.reconnectLoop()
	default:
		// Reconnect already in progress
	}
}

// reconnectLoop handles reconnection with exponential backoff
func (wsm *WebSocketManager) reconnectLoop() {
	wsm.updateState(StateReconnecting)
	
	baseDelay := 1 * time.Second
	maxDelay := wsm.maxReconnectDelay
	
	for wsm.reconnectEnabled {
		wsm.reconnectAttempts++
		
		// Calculate exponential backoff delay
		delay := baseDelay * time.Duration(1<<uint(wsm.reconnectAttempts-1))
		if delay > maxDelay {
			delay = maxDelay
		}
		
		log.Printf("Reconnecting in %v (attempt %d)", delay, wsm.reconnectAttempts)
		
		select {
		case <-time.After(delay):
			err := wsm.Connect()
			if err == nil {
				log.Println("Reconnected successfully")
				return
			}
			log.Printf("Reconnection failed: %v", err)
			
		case <-wsm.done:
			return
		}
	}
}

// queueMessage adds a message to the queue
func (wsm *WebSocketManager) queueMessage(message interface{}) {
	wsm.queueMu.Lock()
	defer wsm.queueMu.Unlock()
	
	if len(wsm.messageQueue) >= wsm.maxQueueSize {
		// Remove oldest message if queue is full
		wsm.messageQueue = wsm.messageQueue[1:]
	}
	
	wsm.messageQueue = append(wsm.messageQueue, message)
}

// flushQueue sends all queued messages
func (wsm *WebSocketManager) flushQueue() {
	wsm.queueMu.Lock()
	messages := wsm.messageQueue
	wsm.messageQueue = make([]interface{}, 0)
	wsm.queueMu.Unlock()
	
	for _, msg := range messages {
		select {
		case wsm.sendChan <- msg:
		case <-time.After(100 * time.Millisecond):
			// Re-queue if send fails
			wsm.queueMessage(msg)
			return
		}
	}
}

// GetQueueSize returns the number of queued messages
func (wsm *WebSocketManager) GetQueueSize() int {
	wsm.queueMu.Lock()
	defer wsm.queueMu.Unlock()
	return len(wsm.messageQueue)
}

// GetLastError returns the last connection error
func (wsm *WebSocketManager) GetLastError() error {
	return wsm.lastError
}

// GetReconnectAttempts returns the number of reconnect attempts
func (wsm *WebSocketManager) GetReconnectAttempts() int {
	return wsm.reconnectAttempts
}

// EnableReconnect enables or disables automatic reconnection
func (wsm *WebSocketManager) EnableReconnect(enable bool) {
	wsm.reconnectEnabled = enable
	if enable && wsm.GetState() == StateDisconnected {
		wsm.triggerReconnect()
	}
} 