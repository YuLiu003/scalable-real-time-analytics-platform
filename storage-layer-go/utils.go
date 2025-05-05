package main

import (
	"log"
	"os"
	"path/filepath"
)

// ensureDataDir ensures the data directory exists
func ensureDataDir(dbPath string) {
	// Extract directory from the full path
	dataDir := filepath.Dir(dbPath)
	if dataDir == "." {
		// If no directory is specified, use the current directory
		return
	}

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("Failed to create data directory '%s': %v", dataDir, err)
	}
}
