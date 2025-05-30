package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	// Set up test environment
	gin.SetMode(gin.TestMode)

	// Initialize API keys for testing
	InitAPIKeys()

	// Run tests
	code := m.Run()

	os.Exit(code)
}

func TestInitAPIKeys(t *testing.T) {
	// Save original environment
	originalKey1 := os.Getenv("API_KEY_1")
	originalKey2 := os.Getenv("API_KEY_2")
	originalKeys := os.Getenv("API_KEYS")

	// Clean up after test
	defer func() {
		os.Setenv("API_KEY_1", originalKey1)
		os.Setenv("API_KEY_2", originalKey2)
		os.Setenv("API_KEYS", originalKeys)
	}()

	t.Run("DefaultKeys", func(t *testing.T) {
		os.Unsetenv("API_KEY_1")
		os.Unsetenv("API_KEY_2")
		os.Unsetenv("API_KEYS")

		InitAPIKeys()

		assert.Contains(t, validAPIKeys, "test-api-key-123456")
		assert.Contains(t, validAPIKeys, "test-api-key-789012")
	})

	t.Run("CustomKeys", func(t *testing.T) {
		os.Setenv("API_KEY_1", "custom-key-1")
		os.Setenv("API_KEY_2", "custom-key-2")

		InitAPIKeys()

		assert.Contains(t, validAPIKeys, "custom-key-1")
		assert.Contains(t, validAPIKeys, "custom-key-2")
	})

	t.Run("CommaSeparatedKeys", func(t *testing.T) {
		os.Setenv("API_KEYS", "key1,key2,key3")

		InitAPIKeys()

		assert.Contains(t, validAPIKeys, "key1")
		assert.Contains(t, validAPIKeys, "key2")
		assert.Contains(t, validAPIKeys, "key3")
	})
}

func TestRequireAPIKey(t *testing.T) {
	// Initialize API keys for testing
	InitAPIKeys()

	middleware := RequireAPIKey()

	t.Run("MissingAPIKey", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)

		middleware(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Missing API key")
	})

	t.Run("InvalidAPIKey", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		c.Request.Header.Set("X-API-Key", "invalid-key")

		middleware(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid API key")
	})

	t.Run("ValidAPIKey", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		c.Request.Header.Set("X-API-Key", "test-api-key-123456")

		middleware(c)

		// If no abort was called, the middleware passed
		assert.False(t, c.IsAborted())
	})

	t.Run("BypassAuth", func(t *testing.T) {
		// Save original value
		originalBypass := os.Getenv("BYPASS_AUTH")
		defer os.Setenv("BYPASS_AUTH", originalBypass)

		os.Setenv("BYPASS_AUTH", "true")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/test", nil)
		// No API key header set

		middleware(c)

		// If no abort was called, the middleware passed
		assert.False(t, c.IsAborted())
	})
}

func TestContains(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	t.Run("ItemExists", func(t *testing.T) {
		assert.True(t, contains(slice, "banana"))
	})

	t.Run("ItemDoesNotExist", func(t *testing.T) {
		assert.False(t, contains(slice, "grape"))
	})

	t.Run("EmptySlice", func(t *testing.T) {
		assert.False(t, contains([]string{}, "item"))
	})
}
