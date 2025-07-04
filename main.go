package main

import (
    "bufio"
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
    "encoding/json"
)

// DataPoint stores a variable's name and its delta values, as well as the max, min, and avg value
type DataPoint struct {
    Name   string // Just the variable name (e.g., "Bytes_received")
    Values []int64
    Max    int64
    Min    int64
    Avg    float64
}

// GroupedData (no longer directly used for display, but logic is similar)
// Kept for conceptual clarity within the ChartForServer context
type GroupedData struct {
    Prefix     string
    DataPoints []DataPoint
}

// ChartForServer holds the DataPoints for ALL variables of a specific prefix on one server
type ChartForServer struct {
    ServerName string
    // DataPoints now holds all DataPoint objects for this server and this prefix
    DataPoints []DataPoint // A slice of DataPoint, one for each variable (e.g., Handler_commit, Handler_read_key)
}

// ChartGroupComparison holds charts for a specific variable prefix across all servers
type ChartGroupComparison struct {
    Prefix string // e.g., "Innodb", "Bytes", "Handler"
    Charts []ChartForServer // Charts for this prefix, one for each server (e.g., Handler for server-a, Handler for server-b)
}

// PageData (modified to reflect the new top-level grouping)
type PageData struct {
    ChartComparisons []ChartGroupComparison // Top-level slice for the template
    Labels           []string               // Common X-axis labels (Snapshot 1, Snapshot 2, ...)
}

// Helper functions (unchanged)
func multiply(a, b int) int { return a * b }
func formatAvg(avg float64) string {
    if avg >= 1000 { return fmt.Sprintf("%.0f", avg) } else if avg >= 100 { return fmt.Sprintf("%.1f", avg) } else if avg >= 10 { return fmt.Sprintf("%.2f", avg) } else { return fmt.Sprintf("%.3f", avg) }
}
func jsQuote(s string) template.JS { return template.JS(strconv.Quote(s)) }
func toJSON(data interface{}) (template.JS, error) {
    b, err := json.Marshal(data); if err != nil { return "", err }
    return template.JS(b), nil
}

// Global regex to check if a string consists only of digits (optional leading minus sign)
var numericRegex = regexp.MustCompile(`^-?\d+$`)


// processFile reads a single pt-stalk output file and populates the nested deltaMap
// `previousDataPoints` and `deltaMap` are now nested maps, allowing server-specific data.
func processFile(
    filePath string,
    serverName string,
    previousDataPoints map[string]map[string]int64, // server -> var -> last value
    deltaMap map[string]map[string][]int64, // server -> var -> deltas
) error {
    file, err := os.Open(filePath)
    if err != nil {
        return fmt.Errorf("failed to open file %s: %w", filePath, err)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)

    for scanner.Scan() {
        line := scanner.Text()

        if strings.HasPrefix(line, "|") && !strings.Contains(line, "+") {
            parts := strings.Split(line, "|")
            if len(parts) < 3 { continue }
            name := strings.TrimSpace(parts[1]) // Original variable name
            valueStr := strings.TrimSpace(parts[2])

            // NEW: Normalize variable name to lowercase for consistency
            name = strings.ToLower(name)

            // Ensure the inner map for this server and variable exists
            if _, ok := previousDataPoints[serverName]; !ok {
                previousDataPoints[serverName] = make(map[string]int64)
            }
            if _, ok := deltaMap[serverName]; !ok {
                deltaMap[serverName] = make(map[string][]int64)
            }

            // Proactively filter non-numeric strings
            if !numericRegex.MatchString(valueStr) { continue }

            value, err := strconv.ParseInt(valueStr, 10, 64)
            if err != nil {
                log.Printf("Warning: Value '%s' for variable '%s' (server %s) in file %s is numeric but out of int64 range and will be skipped: %v", valueStr, name, serverName, filePath, err)
                continue
            }

            // Decide whether to store absolute value or delta
            if strings.HasPrefix(name, "threads_") { // Use lowercase for prefix check
                deltaMap[serverName][name] = append(deltaMap[serverName][name], value)
            } else {
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

func main() {
    if len(os.Args) < 2 {
        log.Fatal("Please specify a directory. Usage: go run main.go <directory>")
    }

    rootDir := os.Args[1]

    // previousDataPoints: server -> variable -> last value
    previousDataPoints := make(map[string]map[string]int64)
    // deltaMap: server -> variable -> []deltas
    deltaMap := make(map[string]map[string][]int64)

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

            fmt.Printf("Processing file: %s (Server: %s)\n", path, serverName)

            if processErr := processFile(path, serverName, previousDataPoints, deltaMap); processErr != nil {
                log.Printf("Error processing file %s (Server: %s): %v", path, serverName, processErr)
            }
        }
        return nil
    })

    if err != nil {
        log.Fatalf("Error walking the directory: %v", err)
    }

    // Generate labels (Snapshot N) based on the maximum number of data points found across all data.
    var allTimestamps []string
    maxDataPoints := 0
    for _, serverVars := range deltaMap {
        for _, deltas := range serverVars {
            if len(deltas) > maxDataPoints {
                maxDataPoints = len(deltas)
            }
        }
    }
    if maxDataPoints == 0 { maxDataPoints = 1 } 
    
    for i := 0; i < maxDataPoints; i++ {
        allTimestamps = append(allTimestamps, fmt.Sprintf("Snapshot %d", i+1))
    }

    // Aggregate data into the ChartGroupComparison structure (Prefix -> Charts for each Server)
    // tempMap: prefix -> serverName -> (map of varName -> DataPoint)
    tempChartComparisonData := make(map[string]map[string]map[string]DataPoint) // prefix -> serverName -> varName -> DataPoint

    var allUniqueServerNames []string // To maintain consistent server order for display
    uniqueServers := make(map[string]bool)

    // Iterate through processed data (server -> variable -> deltas)
    for serverName, varsData := range deltaMap {
        if _, exists := uniqueServers[serverName]; !exists {
            allUniqueServerNames = append(allUniqueServerNames, serverName)
            uniqueServers[serverName] = true
        }

        for varName, deltas := range varsData {
            // Filter out variables with all zero deltas (as per your request)
            allZero := true
            for _, delta := range deltas {
                if delta != 0 {
                    allZero = false
                    break
                }
            }
            if allZero { continue } // Keep this filter active

            // Determine prefix for this variable (using lowercase varName for consistency)
            var prefix string
            lowerVarName := strings.ToLower(varName)
            if strings.HasPrefix(lowerVarName, "innodb_buffer_pool") {
                prefix = "Innodb_buffer_pool"
            } else if strings.HasPrefix(lowerVarName, "innodb_data") {
                prefix = "Innodb_data"
            } else if strings.HasPrefix(lowerVarName, "innodb") {
                prefix = "Innodb"
            } else {
                if idx := strings.Index(lowerVarName, "_"); idx != -1 {
                    prefix = strings.Title(lowerVarName[:idx]) // Title case for common prefixes like "Handler"
                } else {
                    prefix = strings.Title(lowerVarName)
                }
            }

            // Calculate Max, Min, Avg
            var max, min, sum int64
            var avg float64

            if len(deltas) > 0 { // Calculate only if there are deltas
                max, min, sum = deltas[0], deltas[0], int64(0)
                for _, v := range deltas {
                    if v > max { max = v }; if v < min { min = v }; sum += v
                }
                avg = float64(sum) / float64(len(deltas))
            } else { // This case should ideally not be reached if allZero filter is active
                max, min, sum = 0, 0, 0
                avg = 0.0
            }

            // Store in temp map: prefix -> serverName -> varName -> DataPoint
            if _, ok := tempChartComparisonData[prefix]; !ok {
                tempChartComparisonData[prefix] = make(map[string]map[string]DataPoint)
            }
            if _, ok := tempChartComparisonData[prefix][serverName]; !ok {
                tempChartComparisonData[prefix][serverName] = make(map[string]DataPoint)
            }
            tempChartComparisonData[prefix][serverName][varName] = DataPoint{Name: varName, Values: deltas, Max: max, Min: min, Avg: avg}
        }
    }
    sort.Strings(allUniqueServerNames) // Sort server names alphabetically

    // Convert temp map into final PageData structure
    var finalChartComparisons []ChartGroupComparison
    for prefix, serverDataMap := range tempChartComparisonData { // Iterate prefix -> map[serverName]map[varName]DataPoint
        var chartsForThisPrefix []ChartForServer // Collect ChartForServer objects for this prefix

        // Iterate through sorted server names to ensure consistent column order
        for _, serverName := range allUniqueServerNames {
            // Get all DataPoints for this server and this prefix
            varsForServerPrefix, exists := serverDataMap[serverName]
            
            if !exists || len(varsForServerPrefix) == 0 {
                // If this server has no data for this prefix, create an empty/dummy ChartForServer.
                // This ensures its column appears in the grid for consistent layout, but with no data.
                chartsForThisPrefix = append(chartsForThisPrefix, ChartForServer{
                    ServerName: serverName,
                    DataPoints: []DataPoint{}, // Empty slice means no lines will be drawn
                })
                continue
            }

            // Convert map of DataPoints to sorted slice for this ChartForServer
            var sortedDataPointsForChart []DataPoint
            for _, dp := range varsForServerPrefix {
                sortedDataPointsForChart = append(sortedDataPointsForChart, dp)
            }
            sort.Slice(sortedDataPointsForChart, func(i, j int) bool {
                return sortedDataPointsForChart[i].Name < sortedDataPointsForChart[j].Name
            })

            // Add the ChartForServer for this server and prefix
            chartsForThisPrefix = append(chartsForThisPrefix, ChartForServer{
                ServerName: serverName,
                DataPoints: sortedDataPointsForChart, // Contains all relevant DataPoints for this server/prefix
            })
        }
        // Sort ChartForServer objects within this prefix group by server name (already sorted by allUniqueServerNames)
        // No need for sort.Slice(chartsForThisPrefix, ...) if allUniqueServerNames iteration is used correctly.

        finalChartComparisons = append(finalChartComparisons, ChartGroupComparison{Prefix: prefix, Charts: chartsForThisPrefix})
    }
    // Sort the top-level prefix groups by prefix name
    sort.Slice(finalChartComparisons, func(i, j int) bool { return finalChartComparisons[i].Prefix < finalChartComparisons[j].Prefix })


    // Load the HTML template from the file system
    tmpl := template.Must(template.New("chart.html").Funcs(template.FuncMap{
        "multiply":  multiply,
        "formatAvg": formatAvg,
        "js":        jsQuote,
        "toJSON":    toJSON,
    }).ParseFiles("templates/chart.html"))

    // Prepare the data to be passed to the template
    dataToRender := PageData{
        ChartComparisons: finalChartComparisons,
        Labels:           allTimestamps,
    }

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        log.Println("Rendering template...")
        err := tmpl.Execute(w, dataToRender)
        if err != nil { log.Printf("Error rendering template: %v", err) }
    })

    log.Println("Server started. Go to http://localhost:8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
