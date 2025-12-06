package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Global metrics counters
var (
	totalRequests int64
	totalLogs     int64
	totalErrors   int64
)

// Request/Response types
type RawLog struct {
	Level     string  `json:"level"`
	Message   string  `json:"message"`
	Timestamp float64 `json:"timestamp"`
	Service   string  `json:"service"`
	ErrorType string  `json:"error_type,omitempty"`
	RequestID string  `json:"request_id,omitempty"`
}

// Transformed log structure
type TransformedLog struct {
	Level        string `json:"level"`
	Message      string `json:"message"`
	Service      string `json:"service"`
	ErrorType    string `json:"error_type,omitempty"`
	TimestampISO string `json:"timestamp_iso"`
	ReceivedAt   string `json:"received_at"`
	RequestID    string `json:"request_id"`
	Pipeline     string `json:"pipeline_stage"`
}

// Metrics structure for response
type Metrics struct {
	TotalLogs       int            `json:"total_logs"`
	ErrorsByService map[string]int `json:"errors_by_service"`
	ErrorsByType    map[string]int `json:"errors_by_type"`
}

// Response structure
type Response struct {
	Transformed []TransformedLog `json:"transformed_logs"`
	Metrics     Metrics          `json:"metrics"`
	RequestID   string           `json:"request_id"`
	ProcessedAt string           `json:"processed_at"`
}

// generateRequestID creates a unique request ID
func generateRequestID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("req-%s", hex.EncodeToString(bytes))
}

// getRequestID extracts or generates request ID for tracing
func getRequestID(r *http.Request) string {
	if rid := r.Header.Get("X-Request-ID"); rid != "" {
		return rid
	}
	if rid := r.Header.Get("X-Correlation-ID"); rid != "" {
		return rid
	}
	if rid := r.Header.Get("X-Trace-ID"); rid != "" {
		return rid
	}
	return generateRequestID()
}

// validateLog performs basic validation on raw log data
func validateLog(rawLog RawLog) error {
	if strings.TrimSpace(rawLog.Message) == "" {
		return errors.New("message is required and cannot be empty")
	}
	if strings.TrimSpace(rawLog.Service) == "" {
		return errors.New("service is required and cannot be empty")
	}
	return nil
}

// normalizeLevel standardizes log level strings
func normalizeLevel(level string) string {
	l := strings.ToUpper(strings.TrimSpace(level))
	switch l {
	case "ERROR", "ERR", "E", "FATAL", "CRITICAL":
		return "ERROR"
	case "WARN", "WARNING", "W":
		return "WARN"
	case "INFO", "I", "INFORMATION":
		return "INFO"
	case "DEBUG", "D", "TRACE":
		return "DEBUG"
	default:
		return "INFO"
	}
}

// readRequestBody reads and returns the full request body bytes.
func readRequestBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// parseRawLogs unmarshals the body into either []RawLog or a single RawLog.
// Returns a slice of RawLog on success or an error describing the JSON problem.
func parseRawLogs(body []byte) ([]RawLog, error) {
	var rawLogs []RawLog
	if err := json.Unmarshal(body, &rawLogs); err == nil {
		return rawLogs, nil
	}
	// Try single object
	var single RawLog
	if err2 := json.Unmarshal(body, &single); err2 == nil {
		return []RawLog{single}, nil
	}
	// Return the first error (array parse error) to keep behavior similar to original
	return nil, errors.New("invalid JSON format")
}

// validateLogs iterates and validates each RawLog. If validation fails,
// it returns the index and the validation error.
func validateLogs(rawLogs []RawLog) (int, error) {
	for i, rl := range rawLogs {
		if err := validateLog(rl); err != nil {
			return i, err
		}
	}
	return -1, nil
}

// transformLogs converts raw logs into transformed logs and computes local metrics.
func transformLogs(rawLogs []RawLog, requestID string) ([]TransformedLog, Metrics) {
	now := time.Now().UTC().Format(time.RFC3339)

	transformed := make([]TransformedLog, 0, len(rawLogs))
	metrics := Metrics{
		TotalLogs:       len(rawLogs),
		ErrorsByService: make(map[string]int),
		ErrorsByType:    make(map[string]int),
	}

	for _, raw := range rawLogs {
		level := normalizeLevel(raw.Level)

		// Convert timestamp: if not provided or zero, fall back to 'now'
		timestamp := now
		if raw.Timestamp > 0 {
			timestamp = time.Unix(int64(raw.Timestamp), 0).UTC().Format(time.RFC3339)
		}

		// Propagate or generate request ID for each log
		logRequestID := raw.RequestID
		if logRequestID == "" {
			logRequestID = requestID
		}

		// Create transformed log entry
		tLog := TransformedLog{
			Level:        level,
			Message:      strings.TrimSpace(raw.Message),
			Service:      raw.Service,
			ErrorType:    raw.ErrorType,
			TimestampISO: timestamp,
			ReceivedAt:   now,
			RequestID:    logRequestID,
			Pipeline:     "fission-log-processor",
		}
		transformed = append(transformed, tLog)

		// Update error metrics if log is an error
		if level == "ERROR" {
			service := raw.Service
			if service == "" {
				service = "unknown"
			}
			metrics.ErrorsByService[service]++

			errorType := raw.ErrorType
			if errorType == "" {
				errorType = "generic"
			}
			metrics.ErrorsByType[errorType]++
		}
	}

	return transformed, metrics
}

// writeJSON writes a JSON-encoded payload to the ResponseWriter with the given status.
func writeJSON(w http.ResponseWriter, status int, payload interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(payload)
}

// writeError writes a small JSON error response including the request ID and updates error metric.
func writeError(w http.ResponseWriter, status int, msg string, requestID string) {
	atomic.AddInt64(&totalErrors, 1)
	_ = writeJSON(w, status, map[string]string{
		"error":      msg,
		"request_id": requestID,
	})
}

// handler processes incoming log transformation requests
func handler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := getRequestID(r)

	// Increment request counter
	atomic.AddInt64(&totalRequests, 1)

	log.Printf("[REQUEST_START] request_id=%s method=%s path=%s remote_addr=%s",
		requestID, r.Method, r.URL.Path, r.RemoteAddr)

	// Read request body
	body, err := readRequestBody(r)
	if err != nil {
		log.Printf("[REQUEST_ERROR] request_id=%s error=failed_to_read_body err=%v",
			requestID, err)
		writeError(w, http.StatusBadRequest, "failed to read request body", requestID)
		return
	}
	defer r.Body.Close()

	// Parse JSON - accept either array or single object
	rawLogs, err := parseRawLogs(body)
	if err != nil {
		log.Printf("[REQUEST_ERROR] request_id=%s error=invalid_json err=%v",
			requestID, err)
		writeError(w, http.StatusBadRequest, "invalid JSON format", requestID)
		return
	}

	log.Printf("[PROCESSING] request_id=%s log_count=%d", requestID, len(rawLogs))

	// Validate logs
	if idx, vErr := validateLogs(rawLogs); vErr != nil {
		log.Printf("[VALIDATION_ERROR] request_id=%s log_index=%d error=%v",
			requestID, idx, vErr)
		writeError(w, http.StatusBadRequest, fmt.Sprintf("validation failed for log at index %d: %s", idx, vErr.Error()), requestID)
		return
	}

	// Transform logs and compute per-request metrics
	transformed, metrics := transformLogs(rawLogs, requestID)

	// Update global metrics
	atomic.AddInt64(&totalLogs, int64(len(rawLogs)))

	// Build response payload
	now := time.Now().UTC().Format(time.RFC3339)
	response := Response{
		Transformed: transformed,
		Metrics:     metrics,
		RequestID:   requestID,
		ProcessedAt: now,
	}

	duration := time.Since(start)
	log.Printf("[REQUEST_END] request_id=%s duration_ms=%d logs_processed=%d errors=%d",
		requestID, duration.Milliseconds(), len(transformed), len(metrics.ErrorsByType))

	// Set response headers
	w.Header().Set("X-Request-ID", requestID)
	w.Header().Set("X-Processing-Time-Ms", fmt.Sprintf("%d", duration.Milliseconds()))

	// Encode response and handle encode errors
	if err := writeJSON(w, http.StatusOK, response); err != nil {
		log.Printf("[RESPONSE_ERROR] request_id=%s error=%v", requestID, err)
		atomic.AddInt64(&totalErrors, 1)
	}
}

func main() {
	// Configure routes
	http.HandleFunc("/", handler)
	http.HandleFunc("/transform", handler)

	port := ":8888"
	log.Printf("Log Processor starting on port %s", port)
	log.Printf("Endpoints:")
	log.Printf("   - POST /           : Transform logs")
	log.Printf("   - POST /transform  : Transform logs")

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
