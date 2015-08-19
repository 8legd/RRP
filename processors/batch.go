package processors

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	Sequence          int
	Response          *http.Response
	RoundTripDuration time.Duration
}

// we buffer the response body so we can handle timeouts on read
type bufferedBody struct {
	io.Reader
}

func (bufferedBody) Close() error { return nil }

func checkUserAgent(request *http.Request) {
	// Add default User-Agent of `RRP <version>` if none is specified in the request
	// TODO remove hard coded version and set it on build - need to setup our automated build first :)
	if request.Header != nil {
		if ua := request.Header["User-Agent"]; len(ua) == 0 {
			request.Header.Set("User-Agent", "RRP 1.0.0")
		}
	}
}

func errorInTransportResponse(request *http.Request, err error) *http.Response {
	// Create an error response for any HTTP Transport errors - Status 400 (Bad Request)
	response := &http.Response{}
	response.Proto = request.Proto
	response.StatusCode = http.StatusBadRequest
	response.Status = strconv.Itoa(http.StatusBadRequest) + " " + err.Error()
	return response
}

func errorInReadingResponse(request *http.Request, err error) *http.Response {
	// Create an error response for any HTTP Transport errors - Status 400 (Bad Request)
	response := &http.Response{}
	response.Proto = request.Proto
	response.StatusCode = http.StatusBadRequest
	response.Status = strconv.Itoa(http.StatusBadRequest) + " " + err.Error()
	return response
}

func readResponseBody(r *http.Response, timeout time.Duration) (*bufferedBody, error) {

	// Create a buffer to hold the data
	var buffy bytes.Buffer
	var err error

	// Create a timer to timeout if reading the response takes too long
	t := time.NewTimer(timeout)
	go func(r *http.Response) {
		<-t.C
		r.Body.Close()
		buffy.Truncate(0)
		err = errors.New("timeout whilst reading response body")
	}(r)

	// Defer closing of underlying connection so it can be re-used
	defer func(r *http.Response, t *time.Timer) {
		t.Stop()
		r.Body.Close()
	}(r, t)

	// Read the response
	chunkSize := 8192
	for {
		chunk := make([]byte, chunkSize)
		lastReadLength, err := r.Body.Read(chunk)
		if lastReadLength > 0 && lastReadLength < chunkSize {
			chunk = chunk[0:lastReadLength]
		}

		if lastReadLength < 1 || err == io.EOF {
			if lastReadLength > 0 {
				_, err = buffy.Write(chunk)
				if err != nil {
					return nil, err
				}
			}
			io.WriteString(&buffy, "\r\n")
			return &bufferedBody{&buffy}, nil
		}
		if err != nil {
			return nil, err
		}
		_, err = buffy.Write(chunk)
		if err != nil {
			return nil, err
		}
	}
}

// ProcessBatch sends a batch of HTTP requests using http.Transport.
// Each request is sent concurrently in a seperate goroutine.
// The HTTP responses are returned in the same sequence as their corresponding requests.
func ProcessBatch(requests []*http.Request, timeout time.Duration) ([]*http.Response, []time.Duration, error) {
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

	// http.Transport is safe for concurrent use by multiple goroutines and for efficiency should only be created once and re-used
	// TODO maybe handle timeout outside of transport so can just create a single instance for re-use on server start
	// (This would be more accurate anyway as `ResponseHeaderTimeout` does not include the time to read the response body)
	transport := &http.Transport{ResponseHeaderTimeout: timeout}
	transport.Proxy = http.ProxyFromEnvironment

	// Start our individual HTTP Transport goroutines to process the BatchedRequests
	for i := 0; i < z; i++ {
		go func(transport *http.Transport) {
			defer wg.Done()
			r := <-batchedRequests
			checkUserAgent(r.Request)
			startedRoundTrip := time.Now()
			response, err := transport.RoundTrip(r.Request)
			if err != nil {
				response = errorInTransportResponse(r.Request, err)
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
								checkUserAgent(redirect)
								response, err = transport.RoundTrip(redirect)
								if err != nil {
									response = errorInTransportResponse(r.Request, err)
								}
							} else {
								response = errorInTransportResponse(r.Request, err)
							}
						}
					}
				}
			}
			roundTripDuration := time.Since(startedRoundTrip)
			// read response body (into buffer) and then reassign the buffered body to our response
			// this is so we can manage timeouts during the read
			readTimeout := timeout - roundTripDuration
			if readTimeout < 1 {
				readTimeout = 1
			}
			response.Body, err = readResponseBody(response, readTimeout)
			if err != nil {
				response = errorInReadingResponse(r.Request, err)
			}
			batchedResponses <- batchedResponse{r.Sequence, response, roundTripDuration}
		}(transport)
	}

	// Wait for all the requests to be processed
	wg.Wait()
	// Close the second buffered channel that we used to collect the BatchedResponses
	close(batchedResponses)
	// Check we have the correct number of BatchedResponses
	if len(batchedResponses) == z {
		// Return the BatchedResponses in their correct sequence
		responses := make([]*http.Response, z)
		roundTripDurations := make([]time.Duration, z)
		for i := 0; i < z; i++ {
			r := <-batchedResponses
			responses[r.Sequence] = r.Response
			roundTripDurations[r.Sequence] = r.RoundTripDuration
		}
		return responses, roundTripDurations, nil
	}
	err := fmt.Errorf("expected %d responses for this batch but only recieved %d", z, len(batchedResponses))
	return nil, nil, err
}
