# analyze-pt-stalk

A beautiful, modern dashboard tool that transforms collected MySQL variables from `pt-stalk` output files into interactive graphical visualizations using ECharts.

## Features

- 📊 **Interactive Charts**: Beautiful ECharts visualizations with smooth animations
- 🎨 **Modern UI**: Clean, responsive design with gradient backgrounds and hover effects
- 📈 **Multi-Server Support**: Compare MySQL variables across multiple servers
- 🔍 **Data Analysis**: Automatic calculation of max, min, and average values
- 📱 **Responsive Design**: Works seamlessly on desktop and mobile devices
- ⚡ **Fast Processing**: Efficient Go implementation for quick data processing

## Requirements

- Go 1.21 or higher
- pt-stalk output files (files ending with `-mysqladmin`)

## Installation

Clone the repository:

```bash
git clone https://github.com/yunusuyanik/analyze-pt-stalk.git
cd analyze-pt-stalk
```

## Usage

### Option 1: Using Make (Recommended)

```bash
make build
./analyze-pt-stalk /path-to-pt-stalk/
```

Or build and run in one command:

```bash
make run ARGS="/path-to-pt-stalk/"
```

### Option 2: Using Build Script

```bash
./build.sh
./analyze-pt-stalk /path-to-pt-stalk/
```

### Option 3: Manual Build (with CGO disabled)

**Important**: On macOS, you must disable CGO to avoid LC_UUID errors:

```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o analyze-pt-stalk main.go
./analyze-pt-stalk /path-to-pt-stalk/
```

### Option 4: Run directly (for development)

```bash
go run main.go /path-to-pt-stalk/
```

The server will start on `http://localhost:8080`. Open this URL in your browser to view the dashboard.

### Example Output

```
Processing file: /path-to-pt-stalk/2024_09_09_12_46_02-mysqladmin (Server: server1)
Processing file: /path-to-pt-stalk/2024_09_09_12_46_32-mysqladmin (Server: server1)
2024/09/09 16:58:47 Server started. Go to http://localhost:8080
2024/09/09 16:58:51 Rendering template...
```

## How It Works

1. **File Processing**: The tool scans the specified directory for files ending with `-mysqladmin`
2. **Data Extraction**: Parses MySQL variable values from pt-stalk output files
3. **Delta Calculation**: Calculates deltas between snapshots (except for `threads_*` variables which use absolute values)
4. **Grouping**: Groups variables by prefix (e.g., `Innodb`, `Handler`, `Bytes`)
5. **Visualization**: Renders interactive charts using ECharts with statistics tables

## Dashboard Features

- **Grouped Views**: Variables are organized by prefix for easy comparison
- **Server Comparison**: Side-by-side comparison of the same variables across different servers
- **Interactive Charts**: 
  - Zoom and pan functionality
  - Hover tooltips with detailed information
  - Click to toggle series visibility
  - Smooth animations
- **Statistics Tables**: View max, min, and average values for each variable

## Technical Details

- Built with Go for performance and easy deployment
- Uses ECharts 5.4.3 for beautiful, interactive visualizations
- Bootstrap 5.3.2 for responsive layout
- Font Awesome icons for enhanced UI

## Notes

The code was originally written in Python and later converted to Go for:
- Easier installation (single binary)
- Better performance
- More beautiful charts with ECharts
- Improved user experience

## License

This project is open source and available for use.
