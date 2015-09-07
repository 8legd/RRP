package processors

import (
	"net/http"
	"net/url"
	"time"
)

var (
	// DefaultTimeout is the default timeout in seconds and is applied to all the requests contained in the batch
	// (unless the batch request specifies a different value through the `x-rrp-timeout` header)
	DefaultTimeout time.Duration
	// DefaultClient is an instance of the custom http.Client created with with the DefaultTimeout
	// (The returned client has a more leniant redirect policy always URL encoding/escaping the location prior to redirect)
	DefaultClient *http.Client
)

func init() {
	// http.Client is safe for concurrent use by multiple goroutines and for efficiency should only be created once and re-used
	DefaultTimeout = time.Duration(20) * time.Second // Default timeout is 20 seconds, TODO should be configurable e.j flag
	DefaultClient = CreateClient(DefaultTimeout)
}

// CreateClient is used to instantiate a custom http.Client with the specified timeout
// The returned client has a more leniant redirect policy always URL encoding/escaping the location prior to redirect
// TODO keep a cache of most commonly used clients so we can reuse them in the same way as we do for `DefaultClient`
func CreateClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			queryString := ""
			if len(req.URL.Query()) > 0 {
				queryString = "?" + req.URL.Query().Encode()
			}
			escapedURL, err := url.Parse(req.URL.Scheme + "://" + req.URL.Host + req.URL.EscapedPath() + queryString)
			if err != nil {
				req.URL = escapedURL
			}
			return err
		},
	}
}
