package api //nolint:revive // package name is intentional

import (
	"net/http"

	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
)

// AudioTranscriptions handles POST /v1/audio/transcriptions requests.
func (h *ClientHandler) AudioTranscriptions(w http.ResponseWriter, r *http.Request) {
	h.writeError(w, llmerrors.NewInvalidRequestError("", "", "audio endpoints are not enabled"))
}

// AudioTranslations handles POST /v1/audio/translations requests.
func (h *ClientHandler) AudioTranslations(w http.ResponseWriter, r *http.Request) {
	h.writeError(w, llmerrors.NewInvalidRequestError("", "", "audio endpoints are not enabled"))
}

// AudioSpeech handles POST /v1/audio/speech requests.
func (h *ClientHandler) AudioSpeech(w http.ResponseWriter, r *http.Request) {
	h.writeError(w, llmerrors.NewInvalidRequestError("", "", "audio endpoints are not enabled"))
}

// Batches handles POST /v1/batches requests.
func (h *ClientHandler) Batches(w http.ResponseWriter, r *http.Request) {
	h.writeError(w, llmerrors.NewInvalidRequestError("", "", "batch endpoint is not enabled"))
}
