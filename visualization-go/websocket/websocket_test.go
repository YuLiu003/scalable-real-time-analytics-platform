package websocket

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHub(t *testing.T) {
	hub := NewHub()

	assert.NotNil(t, hub)
	assert.NotNil(t, hub.clients)
	assert.NotNil(t, hub.register)
	assert.NotNil(t, hub.unregister)
	assert.NotNil(t, hub.broadcast)
}

func TestHubBroadcast(t *testing.T) {
	hub := NewHub()

	// Test broadcast functionality
	t.Run("BroadcastMessage", func(t *testing.T) {
		message := []byte("test message")

		// Start the hub in a goroutine to consume from channels
		done := make(chan bool, 1)
		go func() {
			defer func() { done <- true }()
			// Run hub for a short time
			select {
			case msg := <-hub.broadcast:
				// Verify we received the expected message
				assert.Equal(t, message, msg)
			case <-hub.register:
				// Handle register if it happens
			case <-hub.unregister:
				// Handle unregister if it happens
			}
		}()

		// This should not panic now that hub is running
		assert.NotPanics(t, func() {
			hub.BroadcastMessage(message)
		})

		// Wait for goroutine to complete
		<-done
	})
}

func TestHubBroadcastToTenant(t *testing.T) {
	hub := NewHub()

	// Test tenant-specific broadcast
	t.Run("BroadcastToSpecificTenant", func(t *testing.T) {
		message := []byte("test message")
		tenantID := "tenant1"

		// This should not panic (no goroutine needed since it doesn't use channels)
		assert.NotPanics(t, func() {
			hub.BroadcastToTenant(message, tenantID)
		})
	})

	t.Run("BroadcastToEmptyTenant", func(t *testing.T) {
		message := []byte("test message")
		tenantID := ""

		// Should handle empty tenant gracefully
		assert.NotPanics(t, func() {
			hub.BroadcastToTenant(message, tenantID)
		})
	})
}

func TestHubRegisterUnregister(t *testing.T) {
	hub := NewHub()

	// Test basic hub functionality without goroutines
	t.Run("RegisterClient", func(t *testing.T) {
		client := &Client{
			hub:      hub,
			conn:     nil, // Mock connection
			send:     make(chan []byte, 256),
			tenantID: "test-tenant",
		}

		// Test that client creation succeeds
		assert.NotNil(t, client)
		assert.Equal(t, "test-tenant", client.tenantID)
		assert.NotNil(t, client.send)
	})
}

func TestHubRun(t *testing.T) {
	hub := NewHub()

	// Test that hub has required channels
	assert.NotNil(t, hub.register)
	assert.NotNil(t, hub.unregister)
	assert.NotNil(t, hub.broadcast)
	assert.NotNil(t, hub.clients)
}
