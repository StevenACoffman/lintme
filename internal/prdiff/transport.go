package prdiff

import "net/http"

// bearerTransport is an http.RoundTripper that attaches a Bearer token to
// each outgoing request. If token is empty the request is forwarded
// unauthenticated (the GitHub API allows 60 requests/hour without a token).
type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.token != "" {
		req = req.Clone(req.Context())
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip( //nolint:wrapcheck // RoundTripper implementations must propagate transport errors unwrapped
		req,
	)
}

// NewHTTPClient returns an *http.Client that authenticates requests with
// token. Pass an empty string to make unauthenticated requests.
func NewHTTPClient(token string) *http.Client {
	return &http.Client{Transport: &bearerTransport{token: token}}
}
