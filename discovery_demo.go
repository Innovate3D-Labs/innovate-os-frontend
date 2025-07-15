package main

import (
	"log"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// DiscoveryDemo demonstrates the printer discovery UI
func runDiscoveryDemo() {
	// Create app
	myApp := app.New()
	myApp.Settings().SetTheme(&InnovateTheme{})
	
	// Create main window
	window := myApp.NewWindow("Printer Discovery Demo")
	window.Resize(fyne.NewSize(400, 300))
	window.CenterOnScreen()
	
	// Get backend URL from environment or use default
	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "localhost:8080"
	}
	
	// Create backend client
	client := NewBackendClient(backendURL)
	
	// Login info
	loginInfo := widget.NewCard("Demo Login", "", widget.NewLabel(
		"Use these credentials in the backend:\n"+
		"Username: demo@example.com\n"+
		"Password: demo123",
	))
	
	// Create discovery button
	btnDiscover := widget.NewButtonWithIcon("Open Printer Discovery", nil, func() {
		// Set a demo auth token (you'd get this from actual login)
		client.SetAuthToken("demo-jwt-token")
		
		discoveryUI := NewPrinterDiscoveryUI(myApp, client)
		discoveryUI.SetOnConnect(func(printer DiscoveredPrinter) {
			dialog := widget.NewCard("Printer Connected", "", widget.NewLabel(
				"Successfully connected to:\n"+
				"Name: "+printer.Name+"\n"+
				"ID: "+printer.Identity.ID+"\n"+
				"Serial: "+printer.Identity.SerialNumber+"\n"+
				"Port: "+printer.Port,
			))
			
			popup := widget.NewModalPopUp(dialog, window.Canvas())
			popup.Show()
		})
		discoveryUI.Show()
	})
	btnDiscover.Importance = widget.HighImportance
	
	// Instructions
	instructions := widget.NewCard("Instructions", "", widget.NewLabel(
		"1. Make sure the backend is running\n"+
		"2. Ensure you have a printer connected via USB\n"+
		"3. Click 'Open Printer Discovery'\n"+
		"4. The system will scan all USB ports\n"+
		"5. Select and connect to your printer",
	))
	
	// Layout
	content := container.NewVBox(
		loginInfo,
		btnDiscover,
		instructions,
	)
	
	window.SetContent(container.NewPadded(content))
	
	// Show and run
	window.ShowAndRun()
}

// Add this to your main.go or run separately
func main() {
	if len(os.Args) > 1 && os.Args[1] == "discovery-demo" {
		runDiscoveryDemo()
		return
	}
	
	// Your normal main function
	log.Println("Run with 'discovery-demo' argument to see the discovery UI demo")
} 