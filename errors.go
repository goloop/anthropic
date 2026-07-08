package anthropic

import (
	"encoding/json"
	"errors"

	"github.com/goloop/ai"
)

// ErrNoResults is returned by BatchResults when a batch has not produced a
// results URL yet (it has not finished processing).
var ErrNoResults = errors.New("anthropic: batch results are not ready")

// parseError turns a non-success response body into an *ai.APIError, filling
// in the provider's error type and message when present.
func parseError(status int, body []byte) error {
	var w struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &w)

	return &ai.APIError{
		Status:  status,
		Type:    w.Error.Type,
		Message: w.Error.Message,
		Raw:     append(json.RawMessage(nil), body...),
	}
}
