package mcpclient

import "net/http"

type headerRoundTripper struct {
	next    http.RoundTripper
	headers map[string]string
}

func (t headerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	for name, value := range t.headers {
		clone.Header.Set(name, value)
	}
	return t.next.RoundTrip(clone)
}
