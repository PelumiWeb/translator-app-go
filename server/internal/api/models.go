package api

import (
	"errors"
	"net/http"

	"github.com/PelumiWeb/translator-app-go/server/internal/models"
)

type manifestResponse struct {
	Models []models.Model `json:"models"`
}

// handleModelManifest lists the models the app may download.
func (a *API) handleModelManifest(w http.ResponseWriter, r *http.Request) {
	list := a.Models.List()
	if list == nil {
		list = []models.Model{} // encode as [] rather than null
	}
	writeJSON(w, http.StatusOK, manifestResponse{Models: list})
}

// handleModelDownload sends one model file, whole or in part.
func (a *API) handleModelDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	model, err := a.Models.Get(id)
	if errors.Is(err, models.ErrNotFound) {
		writeError(w, http.StatusNotFound, "model_not_found", "no such model")
		return
	}
	file, modified, err := a.Models.Open(id)
	if err != nil {
		a.Logger.Error("opening model", "model", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not read the model")
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	// The ETag names this exact content. A client resuming a download sends
	// it back in If-Range; if the file has been replaced since, ServeContent
	// sends the whole new file instead of a tail that would not fit the
	// half the client already has.
	w.Header().Set("ETag", `"`+model.SHA256+`"`)

	// ServeContent does the rest: Range, If-Range, 206 Partial Content, 416
	// for an impossible range, and HEAD. Resume support costs no code.
	http.ServeContent(w, r, id, modified, file)
}
