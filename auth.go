package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AuthManager handles authentication and token management
type AuthManager struct {
	baseURL      string
	httpClient   *http.Client
	currentToken string
	refreshToken string
	expiresAt    time.Time
	user         *User
	tokenFile    string
	mu           sync.RWMutex
	onAuthChange func(bool)
}

// User represents the authenticated user
type User struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	IsActive  bool   `json:"is_active"`
}

// LoginRequest represents login credentials
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents the login API response
type LoginResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	User         User   `json:"user"`
}

// TokenData represents stored token information
type TokenData struct {
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         User      `json:"user"`
}

// NewAuthManager creates a new authentication manager
func NewAuthManager(baseURL string) *AuthManager {
	configDir, _ := os.UserConfigDir()
	tokenFile := filepath.Join(configDir, "innovate-os", "auth.json")
	
	// Ensure directory exists
	os.MkdirAll(filepath.Dir(tokenFile), 0700)
	
	am := &AuthManager{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		tokenFile:  tokenFile,
	}
	
	// Load existing token if available
	am.loadToken()
	
	// Start token refresh goroutine
	go am.autoRefreshToken()
	
	return am
}

// SetAuthChangeCallback sets a callback for authentication state changes
func (am *AuthManager) SetAuthChangeCallback(callback func(bool)) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.onAuthChange = callback
}

// Login authenticates with email and password
func (am *AuthManager) Login(email, password string) error {
	loginReq := LoginRequest{
		Email:    email,
		Password: password,
	}
	
	jsonData, err := json.Marshal(loginReq)
	if err != nil {
		return fmt.Errorf("failed to marshal login request: %v", err)
	}
	
	url := fmt.Sprintf("http://%s/api/auth/login", am.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := am.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Message string `json:"message"`
		}
		json.Unmarshal(body, &errorResp)
		return fmt.Errorf("login failed: %s", errorResp.Message)
	}
	
	var apiResp struct {
		Data LoginResponse `json:"data"`
	}
	
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}
	
	// Store tokens and user info
	am.mu.Lock()
	am.currentToken = apiResp.Data.Token
	am.refreshToken = apiResp.Data.RefreshToken
	am.expiresAt = time.Unix(apiResp.Data.ExpiresAt, 0)
	am.user = &apiResp.Data.User
	am.mu.Unlock()
	
	// Save to file
	if err := am.saveToken(); err != nil {
		// Log error but don't fail login
		fmt.Printf("Failed to save token: %v\n", err)
	}
	
	// Notify auth change
	if am.onAuthChange != nil {
		am.onAuthChange(true)
	}
	
	return nil
}

// Logout logs out the current user
func (am *AuthManager) Logout() error {
	am.mu.RLock()
	token := am.currentToken
	am.mu.RUnlock()
	
	if token != "" {
		// Call logout endpoint
		url := fmt.Sprintf("http://%s/api/auth/logout", am.baseURL)
		req, err := http.NewRequest("POST", url, nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+token)
			am.httpClient.Do(req)
		}
	}
	
	// Clear tokens
	am.mu.Lock()
	am.currentToken = ""
	am.refreshToken = ""
	am.expiresAt = time.Time{}
	am.user = nil
	am.mu.Unlock()
	
	// Remove token file
	os.Remove(am.tokenFile)
	
	// Notify auth change
	if am.onAuthChange != nil {
		am.onAuthChange(false)
	}
	
	return nil
}

// GetToken returns the current authentication token
func (am *AuthManager) GetToken() string {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.currentToken
}

// GetUser returns the current user
func (am *AuthManager) GetUser() *User {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.user
}

// IsAuthenticated checks if user is authenticated
func (am *AuthManager) IsAuthenticated() bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.currentToken != "" && time.Now().Before(am.expiresAt)
}

// RefreshToken refreshes the authentication token
func (am *AuthManager) RefreshToken() error {
	am.mu.RLock()
	refreshToken := am.refreshToken
	am.mu.RUnlock()
	
	if refreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}
	
	var refreshReq struct {
		RefreshToken string `json:"refresh_token"`
	}
	refreshReq.RefreshToken = refreshToken
	
	jsonData, err := json.Marshal(refreshReq)
	if err != nil {
		return err
	}
	
	url := fmt.Sprintf("http://%s/api/auth/refresh", am.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := am.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		// Refresh failed, need to re-login
		am.Logout()
		return fmt.Errorf("token refresh failed")
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	
	var apiResp struct {
		Data LoginResponse `json:"data"`
	}
	
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return err
	}
	
	// Update tokens
	am.mu.Lock()
	am.currentToken = apiResp.Data.Token
	am.refreshToken = apiResp.Data.RefreshToken
	am.expiresAt = time.Unix(apiResp.Data.ExpiresAt, 0)
	am.mu.Unlock()
	
	// Save updated token
	am.saveToken()
	
	return nil
}

// autoRefreshToken automatically refreshes token before expiry
func (am *AuthManager) autoRefreshToken() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		am.mu.RLock()
		expiresAt := am.expiresAt
		am.mu.RUnlock()
		
		// Refresh token 5 minutes before expiry
		if time.Until(expiresAt) < 5*time.Minute && am.IsAuthenticated() {
			if err := am.RefreshToken(); err != nil {
				fmt.Printf("Auto token refresh failed: %v\n", err)
			}
		}
	}
}

// saveToken saves the current token to file
func (am *AuthManager) saveToken() error {
	am.mu.RLock()
	data := TokenData{
		Token:        am.currentToken,
		RefreshToken: am.refreshToken,
		ExpiresAt:    am.expiresAt,
		User:         *am.user,
	}
	am.mu.RUnlock()
	
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	
	return ioutil.WriteFile(am.tokenFile, jsonData, 0600)
}

// loadToken loads token from file
func (am *AuthManager) loadToken() error {
	data, err := ioutil.ReadFile(am.tokenFile)
	if err != nil {
		return err
	}
	
	var tokenData TokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return err
	}
	
	// Check if token is still valid
	if time.Now().After(tokenData.ExpiresAt) {
		// Token expired, remove file
		os.Remove(am.tokenFile)
		return fmt.Errorf("token expired")
	}
	
	am.mu.Lock()
	am.currentToken = tokenData.Token
	am.refreshToken = tokenData.RefreshToken
	am.expiresAt = tokenData.ExpiresAt
	am.user = &tokenData.User
	am.mu.Unlock()
	
	return nil
}

// ParseJWTClaims parses JWT claims without verification (for display purposes)
func (am *AuthManager) ParseJWTClaims() (jwt.MapClaims, error) {
	am.mu.RLock()
	token := am.currentToken
	am.mu.RUnlock()
	
	if token == "" {
		return nil, fmt.Errorf("no token available")
	}
	
	// Parse without verification for display
	parser := jwt.NewParser()
	parsedToken, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}
	
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}
	
	return claims, nil
} 