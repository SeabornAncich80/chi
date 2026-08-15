package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeout(t *testing.T) {
	t.Parallel()

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)

	handler := Timeout(50 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Write([]byte("finished"))
	}))

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusGatewayTimeout {
		t.Errorf("expected status 504, got %d", res.Code)
	}
	if res.Body.String() != "Gateway Timeout" {
		t.Errorf("expected body 'Gateway Timeout', got %q", res.Body.String())
	}
}

func TestTimeoutResponseStarted(t *testing.T) {
	t.Parallel()

	handler := Timeout(50 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("started"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(100 * time.Millisecond)
	}))

	ts := httptest.NewServer(handler)
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", res.StatusCode)
	}
	if string(body) != "started" {
		t.Errorf("expected body 'started', got %q", string(body))
	}
}
