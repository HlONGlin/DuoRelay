package subscription

import (
	"fmt"
	"net/url"
	"strings"
)

// Keep the original cause for errors.Is/As without printing subscription URLs,
// whose path and query often contain credentials.
type fetchError struct{ kind, cause error }

func (e *fetchError) Error() string {
	cause := e.cause
	for {
		wrapped, ok := cause.(*url.Error)
		if !ok {
			break
		}
		cause = wrapped.Err
	}
	detail := cause.Error()
	// Errors from custom transports can contain a URL outside url.Error.
	if strings.Contains(detail, "://") {
		detail = "request failed (URL redacted)"
	}
	return fmt.Sprintf("%s: %s: %s", ErrUpdate, e.kind, detail)
}

func (e *fetchError) Unwrap() []error { return []error{ErrUpdate, e.kind, e.cause} }
