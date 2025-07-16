// +build ignore

package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Test temperature chart with simulated data
func main() {
	// Create test app
	testApp := app.New()
	testApp.Settings().SetTheme(&InnovateTheme{})
	
	window := testApp.NewWindow("Temperature Chart Test")
	window.Resize(fyne.NewSize(1200, 800))
	
	// Create mock backend
	backend := &MockBackend{}
	
	// Create temperature UI
	tempUI := NewTemperatureUI(window, backend)
	
	// Test controls
	simulateBtn := widget.NewButton("Start Heating Simulation", func() {
		go simulateHeatingCycle(tempUI)
	})
	simulateBtn.Resize(fyne.NewSize(200, 50))
	
	addRandomBtn := widget.NewButton("Add Random Data", func() {
		hotend := 20 + rand.Float64()*250
		bed := 20 + rand.Float64()*100
		tempUI.AddTemperatureReading(hotend, hotend-5+rand.Float64()*10, bed, bed-2+rand.Float64()*4)
	})
	addRandomBtn.Resize(fyne.NewSize(200, 50))
	
	stressTestBtn := widget.NewButton("Stress Test", func() {
		go stressTestChart(tempUI)
	})
	stressTestBtn.Resize(fyne.NewSize(200, 50))
	
	coolingBtn := widget.NewButton("Simulate Cooling", func() {
		go simulateCooling(tempUI)
	})
	coolingBtn.Resize(fyne.NewSize(200, 50))
	
	// Test stats
	statsLabel := widget.NewLabel("Chart Statistics:")
	
	// Update stats periodically
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			chart := tempUI.GetChart()
			current := chart.GetCurrentTemperatures()
			
			stats := "No data"
			if current != nil {
				stats = fmt.Sprintf(
					"Latest: Hotend %.1f°C (target %.1f°C), Bed %.1f°C (target %.1f°C)\nData points: %d",
					current.HotendActual, current.HotendTarget,
					current.BedActual, current.BedTarget,
					len(chart.dataPoints),
				)
			}
			statsLabel.SetText(stats)
		}
	}()
	
	// Controls panel
	controls := container.NewGridWithColumns(2,
		simulateBtn,
		addRandomBtn,
		stressTestBtn,
		coolingBtn,
	)
	
	controlCard := widget.NewCard("Test Controls", "", container.NewVBox(
		controls,
		widget.NewSeparator(),
		statsLabel,
	))
	
	// Layout
	content := container.NewVBox(
		controlCard,
		tempUI.GetContent(),
	)
	
	window.SetContent(content)
	
	// Instructions
	log.Println("Temperature Chart Test Demo")
	log.Println("===========================")
	log.Println("This demo tests:")
	log.Println("1. Real-time temperature chart rendering")
	log.Println("2. Interactive controls and zoom")
	log.Println("3. Export functionality")
	log.Println("4. Auto-scaling and time ranges")
	log.Println("")
	log.Println("Try these features:")
	log.Println("- Start heating simulation to see realistic data")
	log.Println("- Change time ranges and zoom levels")
	log.Println("- Export data to CSV")
	log.Println("- Test with different temperature presets")
	log.Println("- Run stress test to check performance")
	
	window.ShowAndRun()
}

// MockBackend provides mock data for testing
type MockBackend struct {
	hotendTarget float64
	bedTarget    float64
	status       string
}

func (m *MockBackend) GetPrinterStatus() (*PrinterStatus, error) {
	return &PrinterStatus{
		Status:       m.status,
		Temperature:  m.hotendTarget + rand.Float64()*10 - 5,
		BedTemp:      m.bedTarget + rand.Float64()*5 - 2.5,
		IsConnected:  true,
	}, nil
}

func (m *MockBackend) SetTemperature(heater string, temperature float64) error {
	if heater == "hotend" {
		m.hotendTarget = temperature
		if temperature > 0 {
			m.status = fmt.Sprintf("Heating hotend to %.0f°C", temperature)
		} else {
			m.status = "Cooling hotend"
		}
	} else if heater == "bed" {
		m.bedTarget = temperature
		if temperature > 0 {
			m.status = fmt.Sprintf("Heating bed to %.0f°C", temperature)
		} else {
			m.status = "Cooling bed"
		}
	}
	return nil
}

// Implement other required methods with no-op
func (m *MockBackend) ConnectWebSocket() error { return nil }
func (m *MockBackend) CloseWebSocket() error { return nil }
func (m *MockBackend) SetAuthToken(token string) {}
func (m *MockBackend) GetWebSocketState() string { return "Connected" }
func (m *MockBackend) IsWebSocketConnected() bool { return true }
func (m *MockBackend) GetWebSocketQueueSize() int { return 0 }
func (m *MockBackend) GetWebSocketReconnectAttempts() int { return 0 }
func (m *MockBackend) EnableWebSocketReconnect(enable bool) {}
func (m *MockBackend) SendWebSocketMessage(message interface{}) error { return nil }
func (m *MockBackend) SetConnectionChangeCallback(callback func(bool)) {}
func (m *MockBackend) ListenForUpdates(statusChan chan<- PrinterStatus) {}

// simulateHeatingCycle simulates a realistic heating cycle
func simulateHeatingCycle(tempUI *TemperatureUI) {
	log.Println("Starting heating simulation...")
	
	hotendTarget := 200.0
	bedTarget := 60.0
	hotendTemp := 20.0
	bedTemp := 20.0
	
	// Set targets
	tempUI.hotendTarget.SetText("200")
	tempUI.bedTarget.SetText("60")
	
	for i := 0; i < 300; i++ { // 5 minutes of simulation
		// Simulate heating curves
		hotendHeatRate := 0.8 + rand.Float64()*0.4  // 0.8-1.2°C per second
		bedHeatRate := 0.3 + rand.Float64()*0.2     // 0.3-0.5°C per second
		
		// Heat towards target with some overshoot/undershoot
		if hotendTemp < hotendTarget {
			hotendTemp += hotendHeatRate
			if hotendTemp > hotendTarget {
				hotendTemp = hotendTarget + rand.Float64()*3 - 1.5 // Small overshoot
			}
		}
		
		if bedTemp < bedTarget {
			bedTemp += bedHeatRate
			if bedTemp > bedTarget {
				bedTemp = bedTarget + rand.Float64()*2 - 1
			}
		}
		
		// Add noise
		hotendNoise := rand.Float64()*2 - 1
		bedNoise := rand.Float64()*1 - 0.5
		
		tempUI.AddTemperatureReading(
			hotendTemp+hotendNoise, hotendTarget,
			bedTemp+bedNoise, bedTarget,
		)
		
		time.Sleep(100 * time.Millisecond) // 10 updates per second
	}
	
	log.Println("Heating simulation complete")
}

// simulateCooling simulates cooling down from high temperatures
func simulateCooling(tempUI *TemperatureUI) {
	log.Println("Starting cooling simulation...")
	
	hotendTemp := 200.0
	bedTemp := 60.0
	ambient := 20.0
	
	// Set targets to 0 (cooling down)
	tempUI.hotendTarget.SetText("0")
	tempUI.bedTarget.SetText("0")
	
	for i := 0; i < 600; i++ { // 10 minutes of cooling
		// Exponential cooling curve
		hotendCoolRate := (hotendTemp - ambient) * 0.01
		bedCoolRate := (bedTemp - ambient) * 0.008
		
		hotendTemp -= hotendCoolRate
		bedTemp -= bedCoolRate
		
		// Don't go below ambient
		if hotendTemp < ambient {
			hotendTemp = ambient
		}
		if bedTemp < ambient {
			bedTemp = ambient
		}
		
		// Add noise
		hotendNoise := rand.Float64()*1 - 0.5
		bedNoise := rand.Float64()*0.5 - 0.25
		
		tempUI.AddTemperatureReading(
			hotendTemp+hotendNoise, 0,
			bedTemp+bedNoise, 0,
		)
		
		time.Sleep(100 * time.Millisecond)
		
		// Stop if both reached ambient
		if hotendTemp <= ambient+1 && bedTemp <= ambient+1 {
			break
		}
	}
	
	log.Println("Cooling simulation complete")
}

// stressTestChart tests chart performance with rapid updates
func stressTestChart(tempUI *TemperatureUI) {
	log.Println("Starting stress test...")
	
	baseHotend := 200.0
	baseBed := 60.0
	
	for i := 0; i < 1000; i++ {
		// Generate sine wave patterns
		t := float64(i) * 0.1
		hotendWave := baseHotend + 10*math.Sin(t*0.5) + 5*math.Sin(t*2)
		bedWave := baseBed + 5*math.Sin(t*0.3+1) + 2*math.Sin(t*1.5)
		
		// Add random noise
		hotendNoise := rand.Float64()*4 - 2
		bedNoise := rand.Float64()*2 - 1
		
		tempUI.AddTemperatureReading(
			hotendWave+hotendNoise, baseHotend,
			bedWave+bedNoise, baseBed,
		)
		
		time.Sleep(10 * time.Millisecond) // Very fast updates
	}
	
	log.Println("Stress test complete - 1000 data points added")
} 