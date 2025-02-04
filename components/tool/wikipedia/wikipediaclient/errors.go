package wikipediaclient

import "fmt"

var (
	ErrPageNotFound      = fmt.Errorf("page not found")
	ErrInvalidParameters = fmt.Errorf("invalid parameters")
	ErrTooManyRedirects  = fmt.Errorf("too many redirects")
)

type APIError struct {
	Code   string `json:"code"`
	Info   string `json:"info"`
	DocRef string `json:"docref"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: %s (code: %s)", e.Info, e.Code)
}
