# Innovate OS Frontend

Native Linux GUI application for 3D printer control, designed for 10-inch touchscreen displays.

## Features

- **Apple-inspired Design**: Clean, minimalistic interface optimized for touch interaction
- **Real-time Updates**: Live printer status via WebSocket connection
- **Touch-Optimized**: Large buttons and controls designed for 10-inch touchscreens
- **Printer Control**: Start, pause, resume, and stop print jobs
- **Manual Control**: Move printer axes and control temperatures
- **File Management**: Upload, download, and manage G-code files
- **Emergency Stop**: Quick access emergency stop functionality
- **System Monitoring**: Real-time logs and system status

## Technology Stack

- **Framework**: Fyne (Go-native GUI framework)
- **Language**: Go 1.21+
- **Communication**: REST API + WebSocket for real-time updates
- **Target Platform**: Linux ARM64 (embedded systems)
- **Display**: Optimized for 1024x600 resolution (10-inch touchscreen)

## Prerequisites

- Go 1.21 or higher
- Linux system with display server (X11)
- Backend API running on port 8080
- Touch-capable display (recommended: 10-inch)

## Installation

### Development Setup

```bash
cd frontend
make dev-setup
```

### Build for Development

```bash
make build
```

### Build for Production (ARM Linux)

```bash
make build-arm
```

### Build All Platforms

```bash
make build-all
```

## Deployment

### Option 1: Manual Deployment

1. Build for ARM:
   ```bash
   make build-arm
   ```

2. Copy to target system:
   ```bash
   scp build/innovate-os-frontend-arm user@printer-host:/tmp/
   ```

3. Install on target:
   ```bash
   ssh user@printer-host
   sudo mkdir -p /opt/innovate-os
   sudo mv /tmp/innovate-os-frontend-arm /opt/innovate-os/innovate-os-frontend
   sudo chmod +x /opt/innovate-os/innovate-os-frontend
   ```

### Option 2: Automated Deployment

```bash
make install-arm TARGET_HOST=user@printer-host
```

### Option 3: Complete Deployment Package

```bash
make package
```

This creates a deployment package in `build/deploy/` containing:
- `innovate-os-frontend` - The compiled binary
- `innovate-os-frontend.service` - Systemd service file
- `install.sh` - Installation script

Transfer the package to your target system and run:
```bash
sudo ./install.sh
```

## Configuration

The frontend connects to the backend at `localhost:8080`. To change this, modify the `NewBackendClient` call in `main_integrated.go`.

## Running

### Development Mode

```bash
make run
```

### Production Mode

The application runs automatically as a systemd service after installation.

Manual start:
```bash
sudo systemctl start innovate-os-frontend
```

Check status:
```bash
sudo systemctl status innovate-os-frontend
```

## Screen Setup

### For 10-inch Touchscreen

1. **Auto-start on boot**: The systemd service automatically starts the application
2. **Fullscreen Mode**: Application runs in fullscreen mode by default
3. **Touch Calibration**: Use system tools to calibrate touch if needed

### Display Environment

Ensure the `DISPLAY` environment variable is set:
```bash
export DISPLAY=:0
```

## User Interface

### Main Sections

1. **Dashboard**: Real-time printer status, temperature, progress
2. **Print Control**: Start/pause/resume/stop print jobs
3. **File Manager**: Upload, download, and manage G-code files
4. **Settings**: Configure temperatures and printer settings
5. **Emergency Stop**: Always accessible emergency stop button

### Touch Interaction

- **Large Buttons**: All buttons are sized for finger touch (minimum 60x60px)
- **Clear Feedback**: Visual feedback for all interactions
- **Swipe Scrolling**: Lists and logs support touch scrolling
- **Gesture Support**: Standard touch gestures supported

## API Integration

The frontend communicates with the backend via:

- **REST API**: For commands and configuration
- **WebSocket**: For real-time status updates
- **File Upload**: For G-code file management

### Backend Dependencies

Ensure the backend is running with these endpoints:
- `GET /api/printer/status` - Printer status
- `POST /api/printer/start` - Start print
- `POST /api/printer/pause` - Pause print
- `POST /api/printer/resume` - Resume print
- `POST /api/printer/stop` - Stop print
- `POST /api/printer/emergency-stop` - Emergency stop
- `GET /api/jobs` - List print jobs
- `POST /api/files/upload` - Upload file
- `DELETE /api/files/{filename}` - Delete file
- `WS /ws` - WebSocket for real-time updates

## Troubleshooting

### Common Issues

1. **Display not showing**: Check `DISPLAY` environment variable
2. **Touch not working**: Verify touch device permissions
3. **Backend connection failed**: Ensure backend is running on port 8080
4. **Service not starting**: Check systemd logs: `journalctl -u innovate-os-frontend`

### Debug Mode

Run with debug output:
```bash
./innovate-os-frontend -debug
```

### Log Files

System logs are available via:
```bash
journalctl -u innovate-os-frontend -f
```

## Development

### Adding New Features

1. UI components go in `main.go` or `main_integrated.go`
2. Backend communication in `backend_client.go`
3. Follow the existing pattern for new screens

### Testing

```bash
make test
```

### Code Quality

```bash
make fmt
make vet
```

## Performance

- **Memory Usage**: ~50MB RAM typical
- **CPU Usage**: Minimal when idle, moderate during updates
- **Storage**: ~25MB binary size
- **Network**: Minimal bandwidth for status updates

## Security

- **Local Communication**: Only communicates with local backend
- **No External Network**: No internet connectivity required
- **File Permissions**: Runs with minimal required permissions
- **Input Validation**: All user inputs are validated

## Customization

### Theme Customization

Edit `InnovateTheme` in `main.go` to customize colors and sizes.

### Layout Customization

Modify the screen functions (`showDashboard`, `showPrintControl`, etc.) to change layouts.

### Touch Sensitivity

Adjust button sizes and touch areas in the UI creation functions.

## License

Part of the Innovate OS project for 3D printer control systems. 