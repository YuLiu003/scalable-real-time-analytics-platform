package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	// URL of the Python tenant management service
	tenantServiceURL = getTenantServiceURL()
)

// getTenantServiceURL returns the URL of the tenant management service
func getTenantServiceURL() string {
	// In a real app, this would come from config
	return "http://localhost:5010"
}

// ListTenants proxies the GET request to list all tenants
func ListTenants(c *gin.Context) {
	url := fmt.Sprintf("%s/api/tenants", tenantServiceURL)

	response, err := http.Get(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Failed to connect to tenant service: %v", err),
		})
		return
	}
	defer response.Body.Close()

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Failed to read response: %v", err),
		})
		return
	}

	// Set the same status code
	c.Status(response.StatusCode)

	// Write the response as-is
	c.Writer.Write(body)
}

// GetTenant proxies the GET request to get a single tenant
func GetTenant(c *gin.Context) {
	tenantID := c.Param("id")
	url := fmt.Sprintf("%s/api/tenants/%s", tenantServiceURL, tenantID)

	response, err := http.Get(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Failed to connect to tenant service: %v", err),
		})
		return
	}
	defer response.Body.Close()

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Failed to read response: %v", err),
		})
		return
	}

	// Set the same status code
	c.Status(response.StatusCode)

	// Write the response as-is
	c.Writer.Write(body)
}

// CreateTenant proxies the POST request to create a new tenant
func CreateTenant(c *gin.Context) {
	url := fmt.Sprintf("%s/api/tenants", tenantServiceURL)

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request body",
		})
		return
	}

	// Create a new request with the same body
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to create request",
		})
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Failed to connect to tenant service: %v", err),
		})
		return
	}
	defer response.Body.Close()

	// Read the response
	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Failed to read response: %v", err),
		})
		return
	}

	// Forward the response
	c.Status(response.StatusCode)
	c.Writer.Write(respBody)
}

// DeleteTenant proxies the DELETE request to delete a tenant
func DeleteTenant(c *gin.Context) {
	tenantID := c.Param("id")
	url := fmt.Sprintf("%s/api/tenants/%s", tenantServiceURL, tenantID)

	// Create a new DELETE request
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to create request",
		})
		return
	}

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Failed to connect to tenant service: %v", err),
		})
		return
	}
	defer response.Body.Close()

	// Read the response
	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Failed to read response: %v", err),
		})
		return
	}

	// Forward the response
	c.Status(response.StatusCode)
	c.Writer.Write(respBody)
}
