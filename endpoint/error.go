package endpoint

import "fmt"

func (e *EndpointError) Error() string {
	return fmt.Sprintf("%s (got: %d, want: %d)", e.Message, e.StatusCode, e.Expected)
}

func NewStatusError(got, want int) *EndpointError {
	return &EndpointError{
		StatusCode: got,
		Expected:   want,
		Message:    "unexpected status code",
	}
}
