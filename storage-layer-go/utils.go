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

	// Create the directory if it doesn't exist with restricted permissions
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		log.Printf("Failed to create data directory '%s': %v", dataDir, err)
	}
}
