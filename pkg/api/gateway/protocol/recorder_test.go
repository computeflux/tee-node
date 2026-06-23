package protocol

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponseRecorder(t *testing.T) {
	upstream := httptest.NewRecorder()
	rec := NewResponseRecorder(upstream)

	rec.Header().Set("X-Custom", "value")
	rec.WriteHeader(http.StatusCreated)
	if rec.StatusCode() != http.StatusCreated {
		t.Errorf("status code mismatch: got %d, want %d", rec.StatusCode(), http.StatusCreated)
	}
	rec.FlushStatus()
	if upstream.Code != http.StatusCreated {
		t.Errorf("upstream status not delegated after FlushStatus: got %d, want %d", upstream.Code, http.StatusCreated)
	}

	body := []byte(`{"ok":true}`)
	if n, err := rec.Write(body); err != nil || n != len(body) {
		t.Fatalf("write failed: n=%d err=%v", n, err)
	}
	if string(rec.Body()) != string(body) {
		t.Errorf("buffered body mismatch: got %q, want %q", rec.Body(), body)
	}
	if upstream.Body.Len() != 0 {
		t.Errorf("body should not be written to upstream yet, got %q", upstream.Body.String())
	}

	if rec.ResponseWriter() != upstream {
		t.Error("ResponseWriter did not return underlying writer")
	}

	// Flush should be a safe no-op for the recorder itself.
	rec.Flush()

	// Header buffering: changes before FlushStatus should be reflected upstream.
	rec2 := NewResponseRecorder(httptest.NewRecorder())
	rec2.Header().Set("Content-Type", "application/json")
	rec2.FlushStatus()
	if rec2.ResponseWriter().Header().Get("Content-Type") != "application/json" {
		t.Errorf("header not flushed upstream: %v", rec2.ResponseWriter().Header())
	}
}
