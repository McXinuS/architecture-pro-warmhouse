package main

import (
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"time"
)

// TemperatureResponse matches services.TemperatureResponse in the smart_home monolith.
type TemperatureResponse struct {
	Value       float64   `json:"value"`
	Unit        string    `json:"unit"`
	Timestamp   time.Time `json:"timestamp"`
	Location    string    `json:"location"`
	Status      string    `json:"status"`
	SensorID    string    `json:"sensor_id"`
	SensorType  string    `json:"sensor_type"`
	Description string    `json:"description"`
}

var locationBySensorID = map[string]string{
	"1": "Living Room",
	"2": "Bedroom",
	"3": "Kitchen",
}

var sensorIDByLocation = map[string]string{
	"Living Room": "1",
	"Bedroom":     "2",
	"Kitchen":     "3",
}

func resolve(location, sensorID string) (string, string) {
	if location == "" {
		location = locationBySensorID[sensorID]
		if location == "" {
			location = "Unknown"
		}
	}
	if sensorID == "" {
		sensorID = sensorIDByLocation[location]
		if sensorID == "" {
			sensorID = "0"
		}
	}
	return location, sensorID
}

func randomTemperature() float64 {
	return math.Round((16+rand.Float64()*10)*10) / 10
}

func writeTemperature(w http.ResponseWriter, location, sensorID string) {
	location, sensorID = resolve(location, sensorID)
	resp := TemperatureResponse{
		Value:       randomTemperature(),
		Unit:        "°C",
		Timestamp:   time.Now().UTC(),
		Location:    location,
		Status:      "active",
		SensorID:    sensorID,
		SensorType:  "temperature",
		Description: "Temperature reading for " + location,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /temperature", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		writeTemperature(w, q.Get("location"), q.Get("sensor_id"))
	})

	mux.HandleFunc("GET /temperature/{sensorId}", func(w http.ResponseWriter, r *http.Request) {
		writeTemperature(w, r.URL.Query().Get("location"), r.PathValue("sensorId"))
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	log.Printf("temperature-api listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
