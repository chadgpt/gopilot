package gopilot

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/tidwall/gjson"
)

func newTempfile(baseDir string) (*os.File, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate filename with current timestamp
	timestamp := time.Now().Format("20060102-150405.00")
	filename := filepath.Join(baseDir, timestamp+".log")

	// Create and open the file
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return file, nil
}

type debugResponseWriter struct {
	http.ResponseWriter
	logFile  *os.File
	isStream bool
}

func (dw *debugResponseWriter) Write(b []byte) (int, error) {
	n, err := dw.ResponseWriter.Write(b)
	if dw.logFile != nil {
		if _, werr := dw.logFile.Write(b); werr != nil {
			log.Println("Error writing to debug log:", werr)
		}
	}
	return n, err
}

func DebugLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("DEBUG") == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Capture request body
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// Determine if stream
		isStream := gjson.GetBytes(body, "stream").Bool()
		model := gjson.GetBytes(body, "model").String()

		logPath := filepath.Join("debug_logs", r.URL.Path, model)
		// Create log file
		logFile, err := newTempfile(logPath)
		if err != nil {
			log.Println("Debug logging error:", err)
			next.ServeHTTP(w, r)
			return
		}
		defer logFile.Close()

		absoluteLogPath, err := filepath.Abs(logFile.Name())
		if err != nil {
			absoluteLogPath = logFile.Name()
		}

		// Write initial log information
		log.Printf("Debug log: %s (stream: %v)", absoluteLogPath, isStream)
		logFile.Write(body)
		logFile.WriteString("\n\n")

		// Wrap response writer
		dw := &debugResponseWriter{
			ResponseWriter: w,
			logFile:        logFile,
			isStream:       isStream,
		}

		next.ServeHTTP(dw, r)
	})
}
