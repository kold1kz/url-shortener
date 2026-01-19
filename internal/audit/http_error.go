package audit

import "fmt"

// httpError возвращается, если HTTP sink получил не-2xx статус.
type httpError struct {
	StatusCode int
}

func (e *httpError) Error() string {
	return fmt.Sprintf(
		"audit http sink status not ok: status=%d",
		e.StatusCode,
	)
}
