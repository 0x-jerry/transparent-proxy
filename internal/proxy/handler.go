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
	Client *http.Client
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

	req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), nil)
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	copyHeader(req.Header, r.Header)
	removeHopByHop(req.Header)
	// Never expose the client's address to the upstream.
	for _, name := range []string{"Forwarded", "X-Forwarded-For", "X-Real-Ip"} {
		req.Header.Del(name)
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		h.logError(r, target, status, err)
		http.Error(w, http.StatusText(status), status)
		return
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	removeHopByHop(w.Header())
	if h.CORS {
		// Upstream may send its own CORS headers; ours must win.
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.WriteHeader(resp.StatusCode)

	if r.Method != http.MethodHead {
		if _, err := io.Copy(w, resp.Body); err != nil {
			h.logError(r, target, resp.StatusCode, err)
		}
	}

	if h.Logger != nil {
		h.Logger.Info("proxied",
			"method", r.Method,
			"target", target.String(),
			"status", resp.StatusCode,
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
