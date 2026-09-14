package proxy

import (
	"crypto/tls"
	"net/http"
	"time"
)

func NewClient(timeout time.Duration, insecureTLS bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: insecureTLS}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		// Relay redirects to the caller instead of following them.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
