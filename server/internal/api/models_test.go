package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/models"
)

const modelContent = "0123456789abcdefghij" // 20 bytes

// newModelsAPI serves one real model file, "base", from a temp directory.
func newModelsAPI(t *testing.T) (*API, models.Model) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "base.bin"), []byte(modelContent), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := models.Load(dir)
	if err != nil {
		t.Fatalf("models.Load: %v", err)
	}
	a := newTestAPI()
	a.Models = catalog
	return a, catalog.List()[0]
}

func get(a *API, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)
	return rec
}

func TestModelManifest(t *testing.T) {
	a, model := newModelsAPI(t)

	rec := get(a, "/v1/models/manifest", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got manifestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding manifest: %v", err)
	}
	if len(got.Models) != 1 || got.Models[0] != model {
		t.Errorf("manifest = %+v, want [%+v]", got.Models, model)
	}
	if model.ID != "base" || model.Size != 20 || model.URL != "/v1/models/base" || len(model.SHA256) != 64 {
		t.Errorf("unexpected model entry: %+v", model)
	}
}

func TestModelManifestWithNoModelsIsAnEmptyList(t *testing.T) {
	rec := get(newTestAPI(), "/v1/models/manifest", nil)

	if got := rec.Body.String(); got != `{"models":[]}`+"\n" {
		t.Errorf("body = %q, want an empty list, not null", got)
	}
}

func TestModelDownloadWholeFile(t *testing.T) {
	a, model := newModelsAPI(t)

	rec := get(a, "/v1/models/base", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != modelContent {
		t.Errorf("body = %q, want the whole file", got)
	}
	if got := rec.Header().Get("ETag"); got != `"`+model.SHA256+`"` {
		t.Errorf("ETag = %s, want the quoted sha256", got)
	}
	if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Errorf("Accept-Ranges = %q, want bytes", got)
	}
	if got := rec.Header().Get("Content-Length"); got != "20" {
		t.Errorf("Content-Length = %q, want 20", got)
	}
}

// What a resumed download looks like: "I have the first 12 bytes, send the rest."
func TestModelDownloadResumesFromAnOffset(t *testing.T) {
	a, model := newModelsAPI(t)

	rec := get(a, "/v1/models/base", map[string]string{
		"Range":    "bytes=12-",
		"If-Range": `"` + model.SHA256 + `"`,
	})

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", rec.Code)
	}
	if got := rec.Body.String(); got != modelContent[12:] {
		t.Errorf("body = %q, want %q", got, modelContent[12:])
	}
	if got := rec.Header().Get("Content-Range"); got != "bytes 12-19/20" {
		t.Errorf("Content-Range = %q, want bytes 12-19/20", got)
	}
}

// If the file changed since the client started, its half-file is useless.
// The server must send everything again, not a tail of the new file.
func TestModelDownloadSendsWholeFileWhenItChangedSinceThePartialDownload(t *testing.T) {
	a, _ := newModelsAPI(t)

	rec := get(a, "/v1/models/base", map[string]string{
		"Range":    "bytes=12-",
		"If-Range": `"the-hash-of-an-older-file"`,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != modelContent {
		t.Errorf("body = %q, want the whole file", got)
	}
}

func TestModelDownloadRejectsARangePastTheEnd(t *testing.T) {
	a, _ := newModelsAPI(t)

	rec := get(a, "/v1/models/base", map[string]string{"Range": "bytes=500-"})

	if rec.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("status = %d, want 416", rec.Code)
	}
}

func TestModelDownloadUnknownModel(t *testing.T) {
	a, _ := newModelsAPI(t)

	for _, path := range []string{"/v1/models/missing", "/v1/models/base.bin", "/v1/models/..%2Fsecret"} {
		rec := get(a, path, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", path, rec.Code)
		}
	}
}
