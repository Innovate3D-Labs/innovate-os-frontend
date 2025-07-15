package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

// uploadGCodeFile uploads a G-code file to the backend
func (ui *PrintJobsUI) uploadGCodeFile(reader fyne.URIReadCloser) error {
	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	// Add file field
	part, err := writer.CreateFormFile("file", reader.URI().Name())
	if err != nil {
		return err
	}
	
	// Copy file content
	if _, err := io.Copy(part, reader); err != nil {
		return err
	}
	
	// Close multipart writer
	if err := writer.Close(); err != nil {
		return err
	}
	
	// Create request
	url := fmt.Sprintf("%s/api/v1/gcode/upload", ui.backendURL)
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	
	req.Header.Set("Authorization", "Bearer "+ui.authToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed: %s", resp.Status)
	}
	
	return nil
}

// loadGCodeFiles loads G-code files from the backend
func (ui *PrintJobsUI) loadGCodeFiles() {
	go func() {
		url := fmt.Sprintf("%s/api/v1/gcode", ui.backendURL)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return
		}
		
		req.Header.Set("Authorization", "Bearer "+ui.authToken)
		
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			var files []GCodeFile
			if err := json.NewDecoder(resp.Body).Decode(&files); err == nil {
				ui.gcodeFiles = files
				ui.fileList.Refresh()
			}
		}
	}()
}

// loadPrintJobs loads print job history from the backend
func (ui *PrintJobsUI) loadPrintJobs() {
	go func() {
		url := fmt.Sprintf("%s/api/v1/jobs?printer_id=%d", ui.backendURL, ui.currentPrinter.ID)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return
		}
		
		req.Header.Set("Authorization", "Bearer "+ui.authToken)
		
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			var jobs []PrintJob
			if err := json.NewDecoder(resp.Body).Decode(&jobs); err == nil {
				ui.printJobs = jobs
				ui.jobList.Refresh()
				ui.updateStatistics()
			}
		}
	}()
}

// startPrint starts a print job
func (ui *PrintJobsUI) startPrint(file *GCodeFile) {
	// Create print job request
	reqBody := struct {
		PrinterID uint `json:"printer_id"`
		FileID    uint `json:"file_id"`
	}{
		PrinterID: ui.currentPrinter.ID,
		FileID:    file.ID,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		dialog.ShowError(err, ui.window)
		return
	}
	
	// Send request
	go func() {
		url := fmt.Sprintf("%s/api/v1/print-jobs", ui.backendURL)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			dialog.ShowError(err, ui.window)
			return
		}
		
		req.Header.Set("Authorization", "Bearer "+ui.authToken)
		req.Header.Set("Content-Type", "application/json")
		
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			dialog.ShowError(err, ui.window)
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			var job PrintJob
			if err := json.NewDecoder(resp.Body).Decode(&job); err == nil {
				ui.currentJob = &job
				ui.statusLabel.SetText(fmt.Sprintf("Print started: %s", file.Name))
				ui.updateActiveJobUI()
				
				// Start monitoring job status
				go ui.monitorPrintJob(&job)
			}
		} else {
			ui.statusLabel.SetText("Failed to start print")
		}
	}()
}

// pauseJob pauses an active print job
func (ui *PrintJobsUI) pauseJob(job *PrintJob) {
	go func() {
		url := fmt.Sprintf("%s/api/v1/print-jobs/%d/pause", ui.backendURL, job.ID)
		req, err := http.NewRequest("POST", url, nil)
		if err != nil {
			return
		}
		
		req.Header.Set("Authorization", "Bearer "+ui.authToken)
		
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			ui.statusLabel.SetText("Print paused")
			job.Status = "paused"
			ui.updateActiveJobUI()
		}
	}()
}

// resumeJob resumes a paused print job
func (ui *PrintJobsUI) resumeJob(job *PrintJob) {
	go func() {
		url := fmt.Sprintf("%s/api/v1/print-jobs/%d/resume", ui.backendURL, job.ID)
		req, err := http.NewRequest("POST", url, nil)
		if err != nil {
			return
		}
		
		req.Header.Set("Authorization", "Bearer "+ui.authToken)
		
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			ui.statusLabel.SetText("Print resumed")
			job.Status = "printing"
			ui.updateActiveJobUI()
		}
	}()
}

// cancelJob cancels an active print job
func (ui *PrintJobsUI) cancelJob(job *PrintJob) {
	go func() {
		url := fmt.Sprintf("%s/api/v1/print-jobs/%d/cancel", ui.backendURL, job.ID)
		req, err := http.NewRequest("POST", url, nil)
		if err != nil {
			return
		}
		
		req.Header.Set("Authorization", "Bearer "+ui.authToken)
		
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			ui.statusLabel.SetText("Print cancelled")
			ui.currentJob = nil
			ui.updateActiveJobUI()
			ui.loadPrintJobs()
		}
	}()
}

// deleteFile deletes a G-code file
func (ui *PrintJobsUI) deleteFile(file *GCodeFile) {
	go func() {
		url := fmt.Sprintf("%s/api/v1/gcode/%d", ui.backendURL, file.ID)
		req, err := http.NewRequest("DELETE", url, nil)
		if err != nil {
			return
		}
		
		req.Header.Set("Authorization", "Bearer "+ui.authToken)
		
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			ui.statusLabel.SetText("File deleted")
			ui.loadGCodeFiles()
		}
	}()
}

// monitorPrintJob monitors the status of an active print job
func (ui *PrintJobsUI) monitorPrintJob(job *PrintJob) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		if ui.currentJob == nil || ui.currentJob.ID != job.ID {
			return
		}
		
		// Get job status
		url := fmt.Sprintf("%s/api/v1/print-jobs/%d", ui.backendURL, job.ID)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		
		req.Header.Set("Authorization", "Bearer "+ui.authToken)
		
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == http.StatusOK {
			var updatedJob PrintJob
			if err := json.NewDecoder(resp.Body).Decode(&updatedJob); err == nil {
				ui.currentJob = &updatedJob
				ui.updateActiveJobUI()
				
				// Stop monitoring if job is completed or cancelled
				if updatedJob.Status == "completed" || updatedJob.Status == "cancelled" || updatedJob.Status == "failed" {
					ui.currentJob = nil
					ui.loadPrintJobs()
					return
				}
			}
		}
	}
}

// updateActiveJobUI updates the active job UI elements
func (ui *PrintJobsUI) updateActiveJobUI() {
	if ui.currentJob == nil {
		ui.progressBar.SetValue(0)
		return
	}
	
	// Update progress
	ui.progressBar.SetValue(float64(ui.currentJob.Progress) / 100.0)
	
	// Update other UI elements...
	// This would update labels, buttons, etc.
}

// updateStatistics updates the print statistics
func (ui *PrintJobsUI) updateStatistics() {
	totalPrints := len(ui.printJobs)
	successCount := 0
	totalTime := 0
	
	for _, job := range ui.printJobs {
		if job.Status == "completed" {
			successCount++
		}
		totalTime += job.TimeElapsed
	}
	
	successRate := 0.0
	if totalPrints > 0 {
		successRate = float64(successCount) / float64(totalPrints) * 100
	}
	
	// Update stats display
	// This would update the statistics card
} 