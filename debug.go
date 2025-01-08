package gopilot

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
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
