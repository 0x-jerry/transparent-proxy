package proxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/enetx/g"
	"github.com/enetx/surf"
)

var hopByHop = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

type Handler struct {
	Client *surf.Client
	Logger *slog.Logger
	CORS   bool
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.CORS {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}

	if h.CORS && r.Method == http.MethodOptions {
		h.servePreflight(w, r)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	raw := r.URL.Query().Get("url")
	if raw == "" {
		h.serveIndex(w, r)
		return
	}

	target, err := parseTarget(raw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.forward(w, r, target)
}

func (h *Handler) servePreflight(w http.ResponseWriter, r *http.Request) {
	header := w.Header()
	header.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	if requested := r.Header.Get("Access-Control-Request-Headers"); requested != "" {
		header.Set("Access-Control-Allow-Headers", requested)
	} else {
		header.Set("Access-Control-Allow-Headers", "*")
	}
	header.Set("Access-Control-Max-Age", "600")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(indexHTML))
}

func (h *Handler) forward(w http.ResponseWriter, r *http.Request, target *url.URL) {
	start := time.Now()

	var req *surf.Request
	if r.Method == http.MethodHead {
		req = h.Client.Head(g.String(target.String()))
	} else {
		req = h.Client.Get(g.String(target.String()))
	}
	req.WithContext(r.Context())

	out := make(http.Header, len(r.Header))
	copyHeader(out, r.Header)
	removeHopByHop(out)
	removeRemoteHeaders(out)
	for name, values := range out {
		for _, value := range values {
			req.AddHeaders(name, value)
		}
	}

	result := req.Do()
	if result.IsErr() {
		status := http.StatusBadGateway
		if result.ErrIs(context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		h.logError(r, target, status, result.Err())
		http.Error(w, http.StatusText(status), status)
		return
	}

	resp := result.Ok()
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	copyHeader(w.Header(), http.Header(resp.Headers))
	removeHopByHop(w.Header())
	if h.CORS {
		// Upstream may send its own CORS headers; ours must win.
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.WriteHeader(int(resp.StatusCode))

	if r.Method != http.MethodHead && resp.Body != nil {
		if _, err := io.Copy(w, resp.Body.Reader); err != nil {
			h.logError(r, target, int(resp.StatusCode), err)
		}
	}

	if h.Logger != nil {
		h.Logger.Info("proxied",
			"method", r.Method,
			"target", target.String(),
			"status", int(resp.StatusCode),
			"duration", time.Since(start),
		)
	}
}

func (h *Handler) logError(r *http.Request, target *url.URL, status int, err error) {
	if h.Logger == nil {
		return
	}
	h.Logger.Warn("proxy failed",
		"method", r.Method,
		"target", target.String(),
		"status", status,
		"error", err,
	)
}

// removeRemoteHeaders drops everything that fingerprints the caller's network origin.
func removeRemoteHeaders(header http.Header) {
	for name := range header {
		if strings.HasPrefix(name, "X-Forwarded-") {
			header.Del(name)
		}
	}
	for _, name := range []string{
		"Forwarded",
		"X-Real-Ip",
		"X-Client-Ip",
		"X-Originating-Ip",
		"X-Remote-Ip",
		"X-Remote-Addr",
		"Client-Ip",
		"True-Client-Ip",
		"Cf-Connecting-Ip",
		"Fastly-Client-Ip",
		"X-Cluster-Client-Ip",
	} {
		header.Del(name)
	}
}

func parseTarget(raw string) (*url.URL, error) {
	target, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("invalid url parameter")
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, errors.New("url scheme must be http or https")
	}
	if target.Host == "" {
		return nil, errors.New("url must be absolute")
	}
	if target.User != nil {
		return nil, errors.New("url must not contain credentials")
	}
	return target, nil
}

func copyHeader(dst, src http.Header) {
	for name, values := range src {
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

func removeHopByHop(header http.Header) {
	for _, name := range strings.Split(header.Get("Connection"), ",") {
		if name = strings.TrimSpace(name); name != "" {
			header.Del(name)
		}
	}
	for name := range hopByHop {
		header.Del(name)
	}
}
