package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Handler serves the telemetry REST API (api/telemetry.openapi.yaml).
type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/telemetry/devices/{deviceId}/latest", h.latest)
	mux.HandleFunc("GET /api/v1/telemetry/devices/{deviceId}/history", h.history)
	return mux
}

type latestResponse struct {
	DeviceID string    `json:"deviceId"`
	Readings []Reading `json:"readings"`
}

type historyResponse struct {
	DeviceID string  `json:"deviceId"`
	Metric   string  `json:"metric"`
	Unit     string  `json:"unit"`
	Interval string  `json:"interval"`
	Points   []Point `json:"points"`
}

func (h *Handler) latest(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := deviceIDFrom(w, r)
	if !ok {
		return
	}
	readings, err := h.store.Latest(r.Context(), deviceID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Внутренняя ошибка", err.Error())
		return
	}
	if len(readings) == 0 {
		writeProblem(w, http.StatusNotFound, "Устройство не найдено", "Нет телеметрии для устройства "+deviceID)
		return
	}
	writeJSON(w, http.StatusOK, latestResponse{DeviceID: deviceID, Readings: readings})
}

// history: from and to default to the last 24 hours, interval defaults to 1h.
func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := deviceIDFrom(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()

	metric := q.Get("metric")
	if metric == "" {
		writeProblem(w, http.StatusBadRequest, "Некорректный запрос", "Параметр metric обязателен")
		return
	}
	interval := q.Get("interval")
	if interval == "" {
		interval = "1h"
	}
	if _, known := truncUnits[interval]; !known && interval != "raw" {
		writeProblem(w, http.StatusBadRequest, "Некорректный интервал", "interval: raw, 1m, 1h или 1d")
		return
	}

	to, err := parseTime(q.Get("to"), time.Now().UTC())
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "Некорректный период", "Параметр to должен быть в формате RFC 3339")
		return
	}
	from, err := parseTime(q.Get("from"), to.Add(-24*time.Hour))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "Некорректный период", "Параметр from должен быть в формате RFC 3339")
		return
	}
	if !from.Before(to) {
		writeProblem(w, http.StatusBadRequest, "Некорректный период", "Параметр from должен быть раньше to")
		return
	}
	if to.Sub(from) > 366*24*time.Hour {
		writeProblem(w, http.StatusBadRequest, "Некорректный период", "Период не длиннее 366 дней")
		return
	}

	points, unit, err := h.store.History(r.Context(), deviceID, metric, from, to, interval)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Внутренняя ошибка", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, historyResponse{
		DeviceID: deviceID,
		Metric:   metric,
		Unit:     unit,
		Interval: interval,
		Points:   points,
	})
}

func parseTime(value string, fallback time.Time) (time.Time, error) {
	if value == "" {
		return fallback, nil
	}
	return time.Parse(time.RFC3339, value)
}

func deviceIDFrom(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("deviceId")
	if _, err := uuid.Parse(id); err != nil {
		writeProblem(w, http.StatusBadRequest, "Некорректный запрос", "deviceId должен быть UUID")
		return "", false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeProblem writes an RFC 7807 error.
func writeProblem(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   "about:blank",
		"title":  title,
		"status": status,
		"detail": detail,
	})
}
