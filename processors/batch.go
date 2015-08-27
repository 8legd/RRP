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

// BatchedResponse is a simple type used as a container for the HTTP responses returned by ProcessBatch
type BatchedResponse struct {
	Sequence           int
	Status             string
	Proto              string
	Header             *http.Header
	Body               *bytes.Reader
	ProcessingDuration time.Duration
}

func checkUserAgent(request *http.Request) {
	// Add default User-Agent of `RRP <version>` if none is specified in the request
	// TODO remove hard coded version and set on build - need to setup our automated build first :)
	if request.Header != nil {
		if ua := request.Header["User-Agent"]; len(ua) == 0 {
			request.Header.Set("User-Agent", "RRP 1.0.1")
		}
	}
}

func errorInTransportResponse(request *http.Request, err error) *http.Response {
	// Return an error response for any HTTP Transport errors - Status 400 (Bad Request)
	response := &http.Response{}
	response.Proto = request.Proto
	response.StatusCode = http.StatusBadRequest
	response.Status = strconv.Itoa(http.StatusBadRequest) + " " + err.Error()
	return response
}

func errorInReadingBody(sequence int, response *http.Response, err error, startedProcessing time.Time, batchedResponses chan BatchedResponse) {
	// Create an error response for any HTTP Transport errors - Status 400 (Bad Request)
	errResponse := &http.Response{}
	errResponse.Proto = response.Request.Proto
	errResponse.StatusCode = http.StatusBadRequest
	errResponse.Status = strconv.Itoa(http.StatusBadRequest) + " " + err.Error()
	batchedResponses <- BatchedResponse{sequence, errResponse.Status, errResponse.Proto, &errResponse.Header, nil, time.Since(startedProcessing)}
}

func readResponseBody(sequence int, response *http.Response, timeout time.Duration, startedProcessing time.Time, batchedResponses chan BatchedResponse) {

	readTimeout := timeout - time.Since(startedProcessing)

	if response.Body == nil {
		batchedResponses <- BatchedResponse{sequence, response.Status, response.Proto, &response.Header, nil, time.Since(startedProcessing)}
		return
	}

	// Create a buffer to hold the data
	var buffy bytes.Buffer

	// Create a timer to timeout if reading the response takes too long
	timedOut := false
	t := time.NewTimer(readTimeout)
	go func() {
		<-t.C
		errorInReadingBody(sequence, response, errors.New("timeout reading response body"), startedProcessing, batchedResponses)
		response.Body.Close()
		timedOut = true
	}()

	// Defer closing of underlying connection so it can be re-used
	defer func() {
		t.Stop()
		response.Body.Close()
	}()

	// Read the response

	// TODO chunkSize should be configurable (as per timeout)
	chunkSize := 16384
	if response.ContentLength > 0 && response.ContentLength < int64(chunkSize) {
		chunkSize = int(response.ContentLength)
	}
	for {
		chunk := make([]byte, chunkSize)
		lastReadLength, err := response.Body.Read(chunk)

		if timedOut { // return on timeout (error response is handled in timeout goroutine)
			return
		}

		if err != nil && err != io.EOF { // return on error in read
			errorInReadingBody(sequence, response, err, startedProcessing, batchedResponses)
			return
		}

		if lastReadLength > 0 && lastReadLength < chunkSize {
			chunk = chunk[0:lastReadLength]
		}
		if lastReadLength < 1 || err == io.EOF { // return on success (finished reading without error)
			if lastReadLength > 0 {
				_, err = buffy.Write(chunk)

				if err != nil { // return on error in write to buffer
					errorInReadingBody(sequence, response, err, startedProcessing, batchedResponses)
					return
				}
			}
			io.WriteString(&buffy, "\r\n") // success
			batchedResponses <- BatchedResponse{sequence, response.Status, response.Proto, &response.Header, bytes.NewReader(buffy.Bytes()), time.Since(startedProcessing)}
			return
		}

		_, err = buffy.Write(chunk) // write next chunk, and keep reading in loop

		if err != nil { // return on error in write
			errorInReadingBody(sequence, response, err, startedProcessing, batchedResponses)
			return
		}
	}
}

// ProcessBatch sends a batch of HTTP requests using http.Transport.
// Each request is sent concurrently in a seperate goroutine.
// The HTTP responses are returned in the same sequence as their corresponding requests.
func ProcessBatch(requests []*http.Request, timeout time.Duration) ([]*BatchedResponse, error) {
	z := len(requests)
	// Setup a buffered channel to queue up the requests for processing by individual HTTP Transport goroutines
	batchedRequests := make(chan batchedRequest, z)
	for i := 0; i < z; i++ {
		batchedRequests <- batchedRequest{i, requests[i]}
	}
	// Close the channel - nothing else is sent to it
	close(batchedRequests)
	// Setup a second buffered channel for collecting the BatchedResponses from the individual HTTP Transport goroutines
	batchedResponses := make(chan BatchedResponse, z)
	// Setup a wait group so we know when all the BatchedRequests have been processed
	var wg sync.WaitGroup
	wg.Add(z)

	// http.Transport is safe for concurrent use by multiple goroutines and for efficiency should only be created once and re-used
	transport := &http.Transport{ResponseHeaderTimeout: timeout}
	transport.Proxy = http.ProxyFromEnvironment

	// Start our individual HTTP Transport goroutines to process the BatchedRequests
	for i := 0; i < z; i++ {
		go func(transport *http.Transport) {
			defer wg.Done()
			r := <-batchedRequests
			startedProcessing := time.Now()
			checkUserAgent(r.Request)
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

			readResponseBody(r.Sequence, response, timeout, startedProcessing, batchedResponses)
			return
		}(transport)
	}

	// Wait for all the requests to be processed
	wg.Wait()
	// Close the second buffered channel that we used to collect the BatchedResponses
	close(batchedResponses)
	// Check we have the correct number of BatchedResponses
	if len(batchedResponses) == z {
		// Return the BatchedResponses in their correct sequence
		responses := make([]*BatchedResponse, z)
		for i := 0; i < z; i++ {
			r := <-batchedResponses
			responses[r.Sequence] = &r
		}
		return responses, nil
	}
	err := fmt.Errorf("expected %d responses for this batch but only recieved %d", z, len(batchedResponses))
	return nil, err
}
