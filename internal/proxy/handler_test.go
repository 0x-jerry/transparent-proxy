package proxy

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newHandler(cors bool) *Handler {
	return &Handler{
		Client: NewClient(5*time.Second, false),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		CORS:   cors,
	}
}

func proxyRequest(t *testing.T, h *Handler, method, target string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, "/?url="+url.QueryEscape(target), nil)
	for name, values := range header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	h.ServeHTTP(rec, req)
	return rec
}

func TestProxyGet(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, "hello "+r.Header.Get("X-Test"))
	}))
	defer upstream.Close()

	rec := proxyRequest(t, newHandler(true), http.MethodGet, upstream.URL, http.Header{"X-Test": {"world"}})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if got := rec.Body.String(); got != "hello world" {
		t.Fatalf("body = %q", got)
	}
	if got := rec.Header().Get("X-Upstream"); got != "yes" {
		t.Fatalf("X-Upstream = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("ACAO = %q", got)
	}
}

func TestProxyHeadHasNoBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "should not appear")
	}))
	defer upstream.Close()

	rec := proxyRequest(t, newHandler(true), http.MethodHead, upstream.URL, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body = %q, want empty", rec.Body.String())
	}
}

func TestProxyRedirectPassedThrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/next", http.StatusFound)
	}))
	defer upstream.Close()

	rec := proxyRequest(t, newHandler(true), http.MethodGet, upstream.URL, nil)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/next" {
		t.Fatalf("Location = %q", got)
	}
}

func TestProxyUpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	target := upstream.URL
	upstream.Close()

	rec := proxyRequest(t, newHandler(true), http.MethodGet, target, nil)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}

func TestProxyRejectsBadTargets(t *testing.T) {
	h := newHandler(true)
	for _, target := range []string{"ftp://example.com", "example.com", "http://", "http://user:pass@example.com"} {
		if rec := proxyRequest(t, h, http.MethodGet, target, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("target %q: status = %d, want 400", target, rec.Code)
		}
	}
}

func TestMethodNotAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	newHandler(true).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != "GET, HEAD, OPTIONS" {
		t.Fatalf("Allow = %q", got)
	}
}

func TestPreflight(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/?url=https://example.com", nil)
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "X-Custom")
	newHandler(true).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("ACAO = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "X-Custom" {
		t.Fatalf("Allow-Headers = %q", got)
	}
}

func TestIndexGuide(t *testing.T) {
	h := newHandler(true)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), `id="u"`) {
		t.Fatal("guide missing the url input")
	}

	head := httptest.NewRecorder()
	h.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD / status = %d body = %q", head.Code, head.Body.String())
	}
}

func TestUnknownPath(t *testing.T) {
	rec := httptest.NewRecorder()
	newHandler(true).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestClientTLSVerification(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "secure")
	}))
	defer upstream.Close()

	verified := proxyRequest(t, newHandler(true), http.MethodGet, upstream.URL, nil)
	if verified.Code != http.StatusBadGateway {
		t.Fatalf("verification on: status = %d, want 502", verified.Code)
	}

	insecure := &Handler{Client: NewClient(5*time.Second, true), CORS: true}
	rec := proxyRequest(t, insecure, http.MethodGet, upstream.URL, nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "secure" {
		t.Fatalf("verification off: status = %d body = %q", rec.Code, rec.Body.String())
	}
}
