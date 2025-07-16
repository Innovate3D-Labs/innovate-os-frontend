package main

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// LoginUI represents the login interface
type LoginUI struct {
	window        fyne.Window
	authManager   *AuthManager
	onLoginSuccess func()
	content       *fyne.Container
}

// NewLoginUI creates a new login interface
func NewLoginUI(window fyne.Window, authManager *AuthManager) *LoginUI {
	return &LoginUI{
		window:      window,
		authManager: authManager,
	}
}

// SetLoginSuccessCallback sets the callback for successful login
func (ui *LoginUI) SetLoginSuccessCallback(callback func()) {
	ui.onLoginSuccess = callback
}

// Show displays the login interface
func (ui *LoginUI) Show() {
	// Create logo/header
	headerText := canvas.NewText("Innovate OS", color.NRGBA{R: 0, G: 122, B: 255, A: 255})
	headerText.TextSize = 32
	headerText.TextStyle = fyne.TextStyle{Bold: true}
	headerText.Alignment = fyne.TextAlignCenter
	
	subHeaderText := canvas.NewText("3D Printer Control System", color.NRGBA{R: 142, G: 142, B: 147, A: 255})
	subHeaderText.TextSize = 18
	subHeaderText.Alignment = fyne.TextAlignCenter
	
	// Create form fields
	emailEntry := widget.NewEntry()
	emailEntry.SetPlaceHolder("Email")
	emailEntry.Validator = nil // Remove validation for touch keyboard
	
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")
	
	// Remember me checkbox
	rememberCheck := widget.NewCheck("Remember me", nil)
	rememberCheck.SetChecked(true)
	
	// Error label (hidden by default)
	errorLabel := widget.NewLabel("")
	errorLabel.TextStyle = fyne.TextStyle{Bold: true}
	errorLabel.Hide()
	
	// Login button
	loginButton := widget.NewButton("Login", func() {
		email := emailEntry.Text
		password := passwordEntry.Text
		
		if email == "" || password == "" {
			ui.showError(errorLabel, "Please enter email and password")
			return
		}
		
		// Show loading
		loginButton.Disable()
		loginButton.SetText("Logging in...")
		
		// Perform login
		go func() {
			err := ui.authManager.Login(email, password)
			
			// Update UI on main thread
			ui.window.Canvas().Refresh(loginButton)
			
			if err != nil {
				loginButton.Enable()
				loginButton.SetText("Login")
				ui.showError(errorLabel, err.Error())
			} else {
				// Clear sensitive data
				passwordEntry.SetText("")
				
				// Call success callback
				if ui.onLoginSuccess != nil {
					ui.onLoginSuccess()
				}
			}
		}()
	})
	loginButton.Importance = widget.HighImportance
	loginButton.Resize(fyne.NewSize(300, 60))
	
	// Demo login button
	demoButton := widget.NewButton("Demo Login", func() {
		emailEntry.SetText("demo@innovate3d.com")
		passwordEntry.SetText("demo")
		loginButton.OnTapped()
	})
	demoButton.Resize(fyne.NewSize(300, 50))
	
	// Form container
	form := container.NewVBox(
		container.NewPadded(headerText),
		subHeaderText,
		widget.NewSeparator(),
		container.NewPadded(emailEntry),
		container.NewPadded(passwordEntry),
		container.NewPadded(rememberCheck),
		container.NewPadded(errorLabel),
		container.NewPadded(loginButton),
		container.NewPadded(demoButton),
	)
	
	// Center the form
	ui.content = container.NewCenter(
		container.NewMaxSize(form, fyne.NewSize(400, 500)),
	)
}

// showError displays an error message
func (ui *LoginUI) showError(label *widget.Label, message string) {
	label.SetText(message)
	label.Show()
	label.Refresh()
	
	// Auto-hide after 5 seconds
	go func() {
		time.Sleep(5 * time.Second)
		label.Hide()
		label.Refresh()
	}()
}

// GetContent returns the login UI content
func (ui *LoginUI) GetContent() *fyne.Container {
	if ui.content == nil {
		ui.Show()
	}
	return ui.content
}

// UserProfileUI represents the user profile interface
type UserProfileUI struct {
	window      fyne.Window
	authManager *AuthManager
	content     *fyne.Container
	onLogout    func()
}

// NewUserProfileUI creates a new user profile interface
func NewUserProfileUI(window fyne.Window, authManager *AuthManager) *UserProfileUI {
	return &UserProfileUI{
		window:      window,
		authManager: authManager,
	}
}

// SetLogoutCallback sets the callback for logout
func (ui *UserProfileUI) SetLogoutCallback(callback func()) {
	ui.onLogout = callback
}

// Show displays the user profile
func (ui *UserProfileUI) Show() {
	user := ui.authManager.GetUser()
	if user == nil {
		return
	}
	
	// Profile header
	profileIcon := canvas.NewCircle(color.NRGBA{R: 0, G: 122, B: 255, A: 255})
	profileIcon.Resize(fyne.NewSize(80, 80))
	
	nameLabel := widget.NewLabel(fmt.Sprintf("%s %s", user.FirstName, user.LastName))
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}
	nameLabel.Alignment = fyne.TextAlignCenter
	
	emailLabel := widget.NewLabel(user.Email)
	emailLabel.Alignment = fyne.TextAlignCenter
	
	// User info card
	infoCard := widget.NewCard("User Information", "", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Username:"),
			widget.NewLabel(user.Username),
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("User ID:"),
			widget.NewLabel(fmt.Sprintf("%d", user.ID)),
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("Status:"),
			widget.NewLabel(func() string {
				if user.IsActive {
					return "Active"
				}
				return "Inactive"
			}()),
		),
	))
	
	// Token info
	tokenCard := widget.NewCard("Session Information", "", container.NewVBox(
		container.NewGridWithColumns(2,
			widget.NewLabel("Expires:"),
			widget.NewLabel(ui.authManager.expiresAt.Format("Jan 2, 2006 3:04 PM")),
		),
		container.NewGridWithColumns(2,
			widget.NewLabel("Authenticated:"),
			widget.NewLabel(func() string {
				if ui.authManager.IsAuthenticated() {
					return "Yes"
				}
				return "No"
			}()),
		),
	))
	
	// Action buttons
	refreshButton := widget.NewButton("Refresh Token", func() {
		if err := ui.authManager.RefreshToken(); err != nil {
			dialog.ShowError(err, ui.window)
		} else {
			dialog.ShowInformation("Success", "Token refreshed successfully", ui.window)
			ui.Show() // Refresh display
		}
	})
	
	logoutButton := widget.NewButton("Logout", func() {
		dialog.ShowConfirm("Logout", "Are you sure you want to logout?", func(confirmed bool) {
			if confirmed {
				ui.authManager.Logout()
				if ui.onLogout != nil {
					ui.onLogout()
				}
			}
		}, ui.window)
	})
	logoutButton.Importance = widget.DangerImportance
	
	// Layout
	ui.content = container.NewVBox(
		container.NewCenter(container.NewVBox(
			profileIcon,
			nameLabel,
			emailLabel,
		)),
		widget.NewSeparator(),
		infoCard,
		tokenCard,
		container.NewHBox(
			layout.NewSpacer(),
			refreshButton,
			logoutButton,
			layout.NewSpacer(),
		),
	)
}

// GetContent returns the user profile content
func (ui *UserProfileUI) GetContent() *fyne.Container {
	if ui.content == nil {
		ui.Show()
	}
	return ui.content
}

// Refresh updates the user profile display
func (ui *UserProfileUI) Refresh() {
	ui.Show()
	if ui.content != nil {
		ui.content.Refresh()
	}
}

// AuthRequiredDialog shows a dialog when authentication is required
func ShowAuthRequiredDialog(window fyne.Window, message string, onLogin func()) {
	content := container.NewVBox(
		widget.NewLabel(message),
		widget.NewLabel("Please login to continue."),
	)
	
	dialog := dialog.NewCustomConfirm("Authentication Required", "Login", "Cancel", content, func(login bool) {
		if login && onLogin != nil {
			onLogin()
		}
	}, window)
	
	dialog.Show()
}

// TokenExpiredHandler handles expired token errors
type TokenExpiredHandler struct {
	window      fyne.Window
	authManager *AuthManager
	onReauth    func()
}

// NewTokenExpiredHandler creates a new token expired handler
func NewTokenExpiredHandler(window fyne.Window, authManager *AuthManager) *TokenExpiredHandler {
	return &TokenExpiredHandler{
		window:      window,
		authManager: authManager,
	}
}

// HandleTokenExpired handles token expiration
func (h *TokenExpiredHandler) HandleTokenExpired() {
	// Try to refresh first
	if err := h.authManager.RefreshToken(); err == nil {
		// Refresh successful, continue
		return
	}
	
	// Refresh failed, need to re-login
	content := container.NewVBox(
		widget.NewLabel("Your session has expired."),
		widget.NewLabel("Please login again to continue."),
	)
	
	dialog := dialog.NewCustomConfirm("Session Expired", "Login", "Cancel", content, func(login bool) {
		if login {
			h.authManager.Logout()
			if h.onReauth != nil {
				h.onReauth()
			}
		}
	}, h.window)
	
	dialog.Show()
} 