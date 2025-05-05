package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// CleanData represents the structure for incoming data
type CleanData map[string]interface{}

func main() {
	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting clean ingestion service...")

	// Set up the Gin router
	r := gin.Default()

	// API endpoints
	r.GET("/health", healthCheck)
	r.POST("/api/data", ingestData)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	log.Printf("Server running on port %s", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// healthCheck provides a simple health check endpoint
func healthCheck(c *gin.Context) {
	log.Println("Health check requested")
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// ingestData handles the data ingestion endpoint
func ingestData(c *gin.Context) {
	log.Println("Data received")

	var data CleanData
	if err := c.ShouldBindJSON(&data); err != nil {
		log.Printf("Invalid JSON data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid JSON data",
		})
		return
	}

	if len(data) == 0 {
		log.Println("No data provided")
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "No data provided",
		})
		return
	}

	// Just log the data for now
	log.Printf("Data: %+v", data)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Data logged successfully",
		"data":    data,
	})
}
