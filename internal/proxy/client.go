package proxy

import (
	"time"

	"github.com/enetx/surf"
)

func NewClient(timeout time.Duration, insecureTLS bool) *surf.Client {
	builder := surf.NewClient().Builder().
		Timeout(timeout).
		Impersonate().Chrome()
	if !insecureTLS {
		builder = builder.SecureTLS()
	}
	return builder.Build().Unwrap()
}
