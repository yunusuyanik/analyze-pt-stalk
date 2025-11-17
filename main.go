package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DataPoint stores a variable's name and its delta values, as well as statistics
type DataPoint struct {
	Name   string  // Variable name (e.g., "Bytes_received")
	Values []int64 // Delta values for each snapshot
	Max    int64   // Maximum delta value
	Min    int64   // Minimum delta value
	Avg    float64 // Average delta value
}

// ChartForServer holds the DataPoints for all variables of a specific prefix on one server
type ChartForServer struct {
	ServerName string      // Server identifier
	DataPoints []DataPoint // All DataPoints for this server and prefix
}

// ChartGroupComparison holds charts for a specific variable prefix across all servers
type ChartGroupComparison struct {
	Prefix string           // Variable prefix (e.g., "Innodb", "Bytes", "Handler")
	Charts []ChartForServer // Charts for this prefix, one for each server
}

// PageData holds all data to be rendered in the template
type PageData struct {
	ChartComparisons []ChartGroupComparison // Top-level slice for the template
	Labels           []string               // Common X-axis labels (Snapshot 1, Snapshot 2, ...)
}

// Helper functions for template
func multiply(a, b int) int {
	return a * b
}

func sub(a, b int) int {
	return a - b
}

func formatAvg(avg float64) string {
	if avg >= 1000 {
		return fmt.Sprintf("%.0f", avg)
	} else if avg >= 100 {
		return fmt.Sprintf("%.1f", avg)
	} else if avg >= 10 {
		return fmt.Sprintf("%.2f", avg)
	}
	return fmt.Sprintf("%.3f", avg)
}

func jsQuote(s string) template.JS {
	return template.JS(strconv.Quote(s))
}

func toJSON(data interface{}) (template.JS, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return template.JS(b), nil
}

// Global regex to check if a string consists only of digits (optional leading minus sign)
var numericRegex = regexp.MustCompile(`^-?\d+$`)

// processFile reads a single pt-stalk output file and populates the nested deltaMap
func processFile(
	filePath string,
	serverName string,
	previousDataPoints map[string]map[string]int64, // server -> var -> last value
	deltaMap map[string]map[string][]int64,         // server -> var -> deltas
) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		// Parse lines that start with "|" and don't contain "+" (header separator)
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "+") {
			parts := strings.Split(line, "|")
			if len(parts) < 3 {
				continue
			}

			name := strings.TrimSpace(parts[1])
			valueStr := strings.TrimSpace(parts[2])

			// Normalize variable name to lowercase for consistency
			name = strings.ToLower(name)

			// Ensure the inner map for this server and variable exists
			if _, ok := previousDataPoints[serverName]; !ok {
				previousDataPoints[serverName] = make(map[string]int64)
			}
			if _, ok := deltaMap[serverName]; !ok {
				deltaMap[serverName] = make(map[string][]int64)
			}

			// Filter non-numeric strings
			if !numericRegex.MatchString(valueStr) {
				continue
			}

			value, err := strconv.ParseInt(valueStr, 10, 64)
			if err != nil {
				log.Printf("Warning: Value '%s' for variable '%s' (server %s) in file %s is out of int64 range: %v",
					valueStr, name, serverName, filePath, err)
				continue
			}

			// Decide whether to store absolute value or delta
			if strings.HasPrefix(name, "threads_") {
				// For threads, store absolute value
				deltaMap[serverName][name] = append(deltaMap[serverName][name], value)
			} else {
				// For other variables, calculate delta
				if prevValue, exists := previousDataPoints[serverName][name]; exists {
					delta := value - prevValue
					deltaMap[serverName][name] = append(deltaMap[serverName][name], delta)
				}
			}

			// Update the previous value for this server and variable
			previousDataPoints[serverName][name] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning file %s: %w", filePath, err)
	}
	return nil
}

// titleCase converts first letter to uppercase
func titleCase(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// determinePrefix determines the variable prefix based on the variable name
func determinePrefix(varName string) string {
	lowerVarName := strings.ToLower(varName)

	if strings.HasPrefix(lowerVarName, "innodb_buffer_pool") {
		return "Innodb_buffer_pool"
	} else if strings.HasPrefix(lowerVarName, "innodb_data") {
		return "Innodb_data"
	} else if strings.HasPrefix(lowerVarName, "innodb") {
		return "Innodb"
	}

	// Extract prefix from first underscore
	if idx := strings.Index(lowerVarName, "_"); idx != -1 {
		return titleCase(lowerVarName[:idx])
	}

	return titleCase(lowerVarName)
}

// calculateStats calculates max, min, and average from a slice of deltas
func calculateStats(deltas []int64) (max, min int64, avg float64) {
	if len(deltas) == 0 {
		return 0, 0, 0.0
	}

	max, min = deltas[0], deltas[0]
	var sum int64

	for _, v := range deltas {
		if v > max {
			max = v
		}
		if v < min {
			min = v
		}
		sum += v
	}

	avg = float64(sum) / float64(len(deltas))
	return max, min, avg
}

// hasNonZeroDelta checks if any delta in the slice is non-zero
func hasNonZeroDelta(deltas []int64) bool {
	for _, delta := range deltas {
		if delta != 0 {
			return true
		}
	}
	return false
}

// processDirectory walks through the directory and processes all pt-stalk files
func processDirectory(rootDir string) (map[string]map[string][]int64, []string, error) {
	previousDataPoints := make(map[string]map[string]int64) // server -> var -> last value
	deltaMap := make(map[string]map[string][]int64)         // server -> var -> []deltas
	uniqueServers := make(map[string]bool)
	var allUniqueServerNames []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error accessing path %s: %v", path, err)
			return nil
		}

		if !info.IsDir() && strings.HasSuffix(path, "-mysqladmin") {
			serverName := filepath.Base(filepath.Dir(path))
			if serverName == "" || serverName == rootDir {
				serverName = "default_server"
			}

			if !uniqueServers[serverName] {
				allUniqueServerNames = append(allUniqueServerNames, serverName)
				uniqueServers[serverName] = true
			}

			fmt.Printf("Processing file: %s (Server: %s)\n", path, serverName)

			if processErr := processFile(path, serverName, previousDataPoints, deltaMap); processErr != nil {
				log.Printf("Error processing file %s (Server: %s): %v", path, serverName, processErr)
			}
		}
		return nil
	})

	if err != nil {
		return nil, nil, fmt.Errorf("error walking directory: %w", err)
	}

	sort.Strings(allUniqueServerNames)
	return deltaMap, allUniqueServerNames, nil
}

// aggregateData converts the deltaMap into ChartGroupComparison structure
func aggregateData(deltaMap map[string]map[string][]int64, allUniqueServerNames []string) []ChartGroupComparison {
	// tempMap: prefix -> serverName -> (map of varName -> DataPoint)
	tempChartComparisonData := make(map[string]map[string]map[string]DataPoint)

	// Iterate through processed data (server -> variable -> deltas)
	for serverName, varsData := range deltaMap {
		for varName, deltas := range varsData {
			// Filter out variables with all zero deltas
			if !hasNonZeroDelta(deltas) {
				continue
			}

			prefix := determinePrefix(varName)
			max, min, avg := calculateStats(deltas)

			// Store in temp map: prefix -> serverName -> varName -> DataPoint
			if _, ok := tempChartComparisonData[prefix]; !ok {
				tempChartComparisonData[prefix] = make(map[string]map[string]DataPoint)
			}
			if _, ok := tempChartComparisonData[prefix][serverName]; !ok {
				tempChartComparisonData[prefix][serverName] = make(map[string]DataPoint)
			}

			tempChartComparisonData[prefix][serverName][varName] = DataPoint{
				Name:   varName,
				Values: deltas,
				Max:    max,
				Min:    min,
				Avg:    avg,
			}
		}
	}

	// Convert temp map into final ChartGroupComparison structure
	var finalChartComparisons []ChartGroupComparison

	for prefix, serverDataMap := range tempChartComparisonData {
		var chartsForThisPrefix []ChartForServer

		// Iterate through sorted server names to ensure consistent order
		for _, serverName := range allUniqueServerNames {
			varsForServerPrefix, exists := serverDataMap[serverName]

			if !exists || len(varsForServerPrefix) == 0 {
				// Create empty ChartForServer for consistent layout
				chartsForThisPrefix = append(chartsForThisPrefix, ChartForServer{
					ServerName: serverName,
					DataPoints: []DataPoint{},
				})
				continue
			}

			// Convert map of DataPoints to sorted slice
			var sortedDataPointsForChart []DataPoint
			for _, dp := range varsForServerPrefix {
				sortedDataPointsForChart = append(sortedDataPointsForChart, dp)
			}

			sort.Slice(sortedDataPointsForChart, func(i, j int) bool {
				return sortedDataPointsForChart[i].Name < sortedDataPointsForChart[j].Name
			})

			chartsForThisPrefix = append(chartsForThisPrefix, ChartForServer{
				ServerName: serverName,
				DataPoints: sortedDataPointsForChart,
			})
		}

		finalChartComparisons = append(finalChartComparisons, ChartGroupComparison{
			Prefix: prefix,
			Charts: chartsForThisPrefix,
		})
	}

	// Sort the top-level prefix groups by prefix name
	sort.Slice(finalChartComparisons, func(i, j int) bool {
		return finalChartComparisons[i].Prefix < finalChartComparisons[j].Prefix
	})

	return finalChartComparisons
}

// generateLabels generates snapshot labels based on the maximum number of data points
func generateLabels(deltaMap map[string]map[string][]int64) []string {
	maxDataPoints := 0
	for _, serverVars := range deltaMap {
		for _, deltas := range serverVars {
			if len(deltas) > maxDataPoints {
				maxDataPoints = len(deltas)
			}
		}
	}

	if maxDataPoints == 0 {
		maxDataPoints = 1
	}

	var labels []string
	for i := 0; i < maxDataPoints; i++ {
		labels = append(labels, fmt.Sprintf("Snapshot %d", i+1))
	}

	return labels
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Please specify a directory. Usage: go run main.go <directory>")
	}

	rootDir := os.Args[1]

	// Process all files in the directory
	deltaMap, allUniqueServerNames, err := processDirectory(rootDir)
	if err != nil {
		log.Fatalf("Error processing directory: %v", err)
	}

	// Generate labels
	labels := generateLabels(deltaMap)

	// Aggregate data into ChartGroupComparison structure
	chartComparisons := aggregateData(deltaMap, allUniqueServerNames)

	// Load the HTML template
	tmpl := template.Must(template.New("chart.html").Funcs(template.FuncMap{
		"multiply":  multiply,
		"sub":       sub,
		"formatAvg": formatAvg,
		"js":        jsQuote,
		"toJSON":    toJSON,
	}).ParseFiles("templates/chart.html"))

	// Prepare the data to be passed to the template
	dataToRender := PageData{
		ChartComparisons: chartComparisons,
		Labels:           labels,
	}

	// Set up HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Rendering template...")
		if err := tmpl.Execute(w, dataToRender); err != nil {
			log.Printf("Error rendering template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})

	log.Println("Server started. Go to http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
