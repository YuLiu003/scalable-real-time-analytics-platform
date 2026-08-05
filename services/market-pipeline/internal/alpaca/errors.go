package alpaca

import (
	"errors"
	"fmt"
)

// ProviderError deliberately omits provider bodies and private request data.
type ProviderError struct {
	Code      int
	Permanent bool
}

func (e ProviderError) Error() string {
	return fmt.Sprintf("market-data provider request failed with status %d", e.Code)
}

func providerError(code int) ProviderError {
	permanent := (code >= 300 && code < 400) || code == 400 || code == 401 || code == 402 || code == 403 ||
		code == 404 || code == 405 || code == 409 || code == 410
	return ProviderError{Code: code, Permanent: permanent}
}

func isPermanent(err error) bool {
	var provider ProviderError
	return errors.As(err, &provider) && provider.Permanent
}
