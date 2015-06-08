package processors

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type batchedRequest struct {
	Sequence int
	Request  *http.Request
}

type batchedResponse struct {
	Sequence int
	Response *http.Response
}

// ProcessBatch sends a batch of HTTP requests using http.Transport.
// Each request is sent concurrently in a seperate goroutine.
// The HTTP responses are returned in the same sequence as their corresponding requests.
func ProcessBatch(requests []*http.Request, timeout time.Duration) ([]*http.Response, error) {
	z := len(requests)
	// Setup a buffered channel to queue up the requests for processing by individual HTTP Transport goroutines
	batchedRequests := make(chan batchedRequest, z)
	for i := 0; i < z; i++ {
		batchedRequests <- batchedRequest{i, requests[i]}
	}
	// Close the channel - nothing else is sent to it
	close(batchedRequests)
	// Setup a second buffered channel for collecting the BatchedResponses from the individual HTTP Transport goroutines
	batchedResponses := make(chan batchedResponse, z)
	// Setup a wait group so we know when all the BatchedRequests have been processed
	var wg sync.WaitGroup
	wg.Add(z)

	// Start our individual HTTP Transport goroutines to process the BatchedRequests
	for i := 0; i < z; i++ {
		go func() {
			defer wg.Done()
			r := <-batchedRequests
			transport := &http.Transport{ResponseHeaderTimeout: timeout}
			transport.DisableCompression = true
			response, err := transport.RoundTrip(r.Request)
			if err != nil {
				// Create an error response for any HTTP Transport errors - Status 400 (Bad Request)
				errorResponse := http.Response{}
				errorResponse.Proto = r.Request.Proto
				errorResponse.StatusCode = http.StatusBadRequest
				errorResponse.Status = strconv.Itoa(http.StatusBadRequest) + " " + err.Error()
				batchedResponses <- batchedResponse{r.Sequence, &errorResponse}
			} else {
				// TODO add support for all possible redirect status codes, see line 249 of https://golang.org/src/net/http/client.go
				if response.StatusCode == 302 {
					location := response.Header.Get("Location")
					if location != "" {
						redirectURL, err := url.Parse(location)
						if err == nil {
							if !redirectURL.IsAbs() { // handle relative URLs
								redirectURL, err = url.Parse(r.Request.URL.Scheme + "://" + r.Request.Host + "/" + location)
							}
							queryString := ""
							if len(redirectURL.Query()) > 0 {
								queryString = "?" + redirectURL.Query().Encode()
							}
							redirect, err := http.NewRequest("GET", redirectURL.Scheme+"://"+redirectURL.Host+redirectURL.Path+queryString, nil)
							if err == nil {
								response, err = transport.RoundTrip(redirect)
							}
						}
					}
				}
				batchedResponses <- batchedResponse{r.Sequence, response}
			}
		}()
	}

	// Wait for all the requests to be processed
	wg.Wait()
	// Close the second buffered channel that we used to collect the BatchedResponses
	close(batchedResponses)
	// Check we have the correct number of BatchedResponses
	if len(batchedResponses) == z {
		// Return the BatchedResponses in their correct sequence
		result := make([]*http.Response, z)
		for i := 0; i < z; i++ {
			r := <-batchedResponses
			result[r.Sequence] = r.Response
		}
		return result, nil
	}
	err := fmt.Errorf("expected %d responses for this batch but only recieved %d", z, len(batchedResponses))
	return nil, err
}
