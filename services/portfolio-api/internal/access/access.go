package access

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

var bearerToken = regexp.MustCompile(`^[A-Za-z0-9._~+/-]+=*$`)

// Load reads a bearer token from a file and fails closed for non-demo data.
func Load(portfolio, path string) (string, error) {
	token, err := loadToken(path)
	if err != nil {
		return "", err
	}
	if portfolio != "demo" && token == "" {
		return "", errors.New("an access token is required for a non-demo portfolio")
	}
	return token, nil
}

func loadToken(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("access token file path must be absolute")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New("access token file could not be read")
	}
	if !utf8.Valid(data) {
		return "", errors.New("access token file contains invalid UTF-8")
	}
	token := strings.TrimSuffix(string(data), "\n")
	if length := len(token); length < 32 || length > 512 {
		return "", errors.New("access token must contain 32 to 512 characters")
	}
	if !bearerToken.MatchString(token) {
		return "", errors.New("access token must use bearer-safe ASCII characters")
	}
	return token, nil
}
