package main

import (
    "bufio"
    "fmt"
    "html/template"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
)

// DataPoint stores a variable's name and its delta values, as well as the max, min, and avg value
type DataPoint struct {
    Name    string
    Values  []int
    Max     int
    Min     int
    Avg     float64
}

// GroupedData stores a prefix and its associated DataPoints
type GroupedData struct {
    Prefix     string
    DataPoints []DataPoint
}

func multiply(a, b int) int {
    return a * b
}

func formatAvg(avg float64) string {
    if avg >= 1000 {
        return fmt.Sprintf("%.0f", avg)
    } else if avg >= 100 {
        return fmt.Sprintf("%.1f", avg)
    } else if avg >= 10 {
        return fmt.Sprintf("%.2f", avg)
    } else {
        return fmt.Sprintf("%.3f", avg)
    }
}

func processFile(filePath string, previousDataPoints map[string]int, deltaMap map[string][]int) {
    file, err := os.Open(filePath)
    if err != nil {
        log.Fatalf("Failed to open file: %v", err)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)

    for scanner.Scan() {
        line := scanner.Text()

        if strings.HasPrefix(line, "|") && !strings.Contains(line, "+") {
            parts := strings.Split(line, "|")
            if len(parts) < 3 {
                continue
            }
            name := strings.TrimSpace(parts[1])
            valueStr := strings.TrimSpace(parts[2])
            value, err := strconv.Atoi(valueStr)
            if err != nil {
                continue
            }

            if strings.HasPrefix(name, "Threads_") {
                // Threads ile başlayan değişkenler için delta yerine doğrudan değeri ekle
                deltaMap[name] = append(deltaMap[name], value)
            } else {
                if prevValue, exists := previousDataPoints[name]; exists {
                    delta := value - prevValue
                    deltaMap[name] = append(deltaMap[name], delta)
                }
            }

            previousDataPoints[name] = value
        }
    }

    if err := scanner.Err(); err != nil {
        log.Fatalf("Error scanning file: %v", err)
    }
}

func main() {
    if len(os.Args) < 2 {
        log.Fatal("Please specify a directory. Usage: go run main.go <directory>")
    }

    rootDir := os.Args[1]

    previousDataPoints := make(map[string]int)
    deltaMap := make(map[string][]int)

    // Dosyaları bul ve sırayla işle
    err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() && strings.HasSuffix(path, "-mysqladmin") {
            fmt.Printf("Processing file: %s\n", path)
            processFile(path, previousDataPoints, deltaMap)
        }
        return nil
    })

    if err != nil {
        log.Fatalf("Error walking the directory: %v", err)
    }

    groupedData := make(map[string][]DataPoint)
    for name, deltas := range deltaMap {
        allZero := true
        for _, delta := range deltas {
            if delta != 0 {
                allZero = false
                break
            }
        }
        if !allZero {
            var prefix string
            if strings.HasPrefix(name, "Innodb_buffer_pool") {
                prefix = "Innodb_buffer_pool"
            } else if strings.HasPrefix(name, "Innodb") {
                prefix = "Innodb"
            } else {
                if idx := strings.Index(name, "_"); idx != -1 {
                    prefix = name[:idx]
                } else {
                    prefix = name
                }
            }
            max, min, sum := deltas[0], deltas[0], 0
            for _, v := range deltas {
                if v > max {
                    max = v
                }
                if v < min {
                    min = v
                }
                sum += v
            }
            avg := float64(sum) / float64(len(deltas))
            groupedData[prefix] = append(groupedData[prefix], DataPoint{Name: name, Values: deltas, Max: max, Min: min, Avg: avg})
        }
    }

    var sortedGroups []GroupedData
    for prefix, dataPoints := range groupedData {
        if len(dataPoints) > 0 {
            sort.Slice(dataPoints, func(i, j int) bool {
                return dataPoints[i].Name < dataPoints[j].Name
            })
            sortedGroups = append(sortedGroups, GroupedData{Prefix: prefix, DataPoints: dataPoints})
        }
    }

    sort.Slice(sortedGroups, func(i, j int) bool {
        return sortedGroups[i].Prefix < sortedGroups[j].Prefix
    })

    tmpl := template.Must(template.New("chart").Funcs(template.FuncMap{
        "multiply":  multiply,
        "formatAvg": formatAvg,
    }).Parse(chartTemplate))

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        log.Println("Rendering template...")
        err := tmpl.Execute(w, sortedGroups)
        if err != nil {
            log.Printf("Error rendering template: %v", err)
        }
    })

    log.Println("Server started. Go to http://localhost:8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}


const chartTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go Delta Charts</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        .chart-container {
            display: flex;
            flex-wrap: wrap;
            justify-content: space-between;
            gap: 20px; /* Add space between chart items */
        }
        .chart-item {
            width: 48%; /* Make each chart occupy half the width */
            margin-bottom: 40px; /* Increase bottom margin for more space between rows */
        }
        table {
            width: 100%;
            margin-top: 3px;
            border-collapse: collapse;
        }
        th, td {
            border: 1px solid #ddd;
            padding: 3px;
        }
        th {
            background-color: #f2f2f2;
            text-align: left;
        }
        canvas {
            width: 100% !important;
            height: auto !important;
        }
    </style>
</head>
<body>
    <div class="container mt-5">
        <h1 class="text-center">Delta Charts</h1>
        <div class="chart-container">
            {{range .}}
            <div class="chart-item">
                <h2>{{.Prefix}} Variables</h2>
                <canvas id="{{.Prefix}}Chart" width="400" height="200"></canvas>
                <table>
                    <tr>
                        <th>Name</th>
                        <th>Max</th>
                        <th>Min</th>
                        <th>Avg</th>
                    </tr>
                    {{range .DataPoints}}
                    <tr>
                        <td>{{.Name}}</td>
                        <td>{{.Max}}</td>
                        <td>{{.Min}}</td>
                        <td>{{formatAvg .Avg}}</td>
                    </tr>
                    {{end}}
                </table>
            </div>
            <script>
                console.log("Creating chart for: {{.Prefix}}");
                var canvasElement = document.getElementById('{{.Prefix}}Chart');
                var ctx = canvasElement.getContext('2d');
                var labels = Array.from({length: {{len (index .DataPoints 0).Values}}}, (_, i) => i + 1);

                var datasets = [
                    {{range $index, $element := .DataPoints}}
                    {
                        label: "{{js $element.Name}}",
                        data: [{{range $element.Values}}{{.}},{{end}}],
                        borderColor: "hsl({{multiply $index 30}}, 70%, 50%)",
                        backgroundColor: "hsla({{multiply $index 30}}, 70%, 50%, 0.2)",
                        fill: true,
                        borderWidth: 1,
                        tension: 0.4,
                        pointRadius: 2,
                        pointHoverRadius: 4,
                        pointBackgroundColor: "hsl({{multiply $index 30}}, 70%, 50%)"
                    },
                    {{end}}
                ];

                var myChart = new Chart(ctx, {
                    type: 'line',
                    data: {
                        labels: labels,
                        datasets: datasets
                    },
                    options: {
                        scales: {
                            y: {
                                beginAtZero: false
                            }
                        },
                        plugins: {
                            tooltip: {
                                mode: 'index',
                                intersect: false,
                                callbacks: {
                                    label: function(tooltipItem) {
                                        return tooltipItem.dataset.label + ': ' + tooltipItem.raw;
                                    }
                                }
                            },
                            legend: {
                                display: true,
                                position: 'bottom'
                            }
                        },
                        onClick: function(event, elements) {
                            if (elements.length > 0) {
                                const index = elements[0].index;
                                const datasetIndex = elements[0].datasetIndex;
                                const dataset = this.data.datasets[datasetIndex];
                                dataset.hidden = !dataset.hidden;
                                this.update();
                            }
                        }
                    }
                });
            </script>
            {{end}}
        </div>
    </div>
</body>
</html>
` 
