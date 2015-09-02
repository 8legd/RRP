package processors

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var (
	// DefaultTimeout is the default timeout in seconds and is applied to all the requests contained in the batch
	// (unless the batch request specifies a different value through the `x-rrp-timeout` header)
	DefaultTimeout time.Duration
	defaultClient  *http.Client
)

func init() {
	// http.Client is safe for concurrent use by multiple goroutines and for efficiency should only be created once and re-used
	DefaultTimeout = time.Duration(20) * time.Second // Default timeout is 20 seconds, TODO should be configurable e.j flag
	defaultClient = &http.Client{Timeout: DefaultTimeout}
}

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

func sendErrorResponse(sequence int, proto string, err error, timeout time.Duration, startedProcessing time.Time, batchedResponses chan BatchedResponse) {
	// Return an error response - Status 400 (Bad Request)
	e := err
	if time.Since(startedProcessing) > timeout {
		e = fmt.Errorf("request probably cancelled by timeout causing error: %s", err.Error())
	}
	errResponse := &http.Response{}
	errResponse.Proto = proto
	errResponse.StatusCode = http.StatusBadRequest
	errResponse.Status = strconv.Itoa(http.StatusBadRequest) + " " + e.Error()
	batchedResponses <- BatchedResponse{sequence, errResponse.Status, errResponse.Proto, &errResponse.Header, nil, time.Since(startedProcessing)}
}

// ProcessBatch sends a batch of HTTP requests using http.Client.
// Each request is sent concurrently in a seperate goroutine.
// The HTTP responses are returned in the same sequence as their corresponding requests.
func ProcessBatch(requests []*http.Request, timeout time.Duration) ([]*BatchedResponse, error) {
	z := len(requests)
	// Setup a buffered channel to queue up the requests for processing by individual HTTP Client goroutines
	batchedRequests := make(chan batchedRequest, z)
	for i := 0; i < z; i++ {
		batchedRequests <- batchedRequest{i, requests[i]}
	}
	// Close the channel - nothing else is sent to it
	close(batchedRequests)
	// Setup a second buffered channel for collecting the BatchedResponses from the individual HTTP Client goroutines
	batchedResponses := make(chan BatchedResponse, z)
	// Setup a wait group so we know when all the BatchedRequests have been processed
	var wg sync.WaitGroup
	wg.Add(z)

	var client *http.Client
	if DefaultTimeout == timeout {
		client = defaultClient
	} else {
		// create a non standard client for the request
		// TODO keep a cache of most commonly used clients/timeout values so we can reuse them in the same way as we do for `defaultClient`
		client = &http.Client{Timeout: timeout}
	}

	// Start our individual HTTP Client goroutines to process the BatchedRequests
	for i := 0; i < z; i++ {
		go func() {
			defer wg.Done()
			r := <-batchedRequests
			startedProcessing := time.Now()

			response, err := client.Do(r.Request)

			// Defer closing of underlying connection so it can be re-used
			defer func() {
				if response != nil && response.Body != nil {
					response.Body.Close()
				}
			}()
			if err != nil {
				sendErrorResponse(r.Sequence, r.Request.Proto, err, timeout, startedProcessing, batchedResponses)
				return
			}
			// If there is no body to read we are done
			if response.Body == nil {
				batchedResponses <- BatchedResponse{r.Sequence, response.Status, response.Proto, &response.Header, nil, time.Since(startedProcessing)}
				return
			}
			// Create a buffer to hold the data
			var buffy bytes.Buffer

			// Read the response
			// TODO chunkSize should be configurable (as per timeout above e.g. flag)
			chunkSize := 16384
			if response.ContentLength > 0 && response.ContentLength < int64(chunkSize) {
				chunkSize = int(response.ContentLength)
			}
			for {
				chunk := make([]byte, chunkSize)
				lastReadLength, err := response.Body.Read(chunk)

				if err != nil && err != io.EOF { // return on error in read
					sendErrorResponse(r.Sequence, response.Proto, err, timeout, startedProcessing, batchedResponses)
					return
				}

				if lastReadLength > 0 && lastReadLength < chunkSize {
					chunk = chunk[0:lastReadLength]
				}
				if lastReadLength < 1 || err == io.EOF { // return on success (finished reading without error)
					if lastReadLength > 0 {
						_, err = buffy.Write(chunk)

						if err != nil { // return on error in write to buffer
							sendErrorResponse(r.Sequence, response.Proto, err, timeout, startedProcessing, batchedResponses)
							return
						}
					}
					io.WriteString(&buffy, "\r\n") // success
					batchedResponses <- BatchedResponse{r.Sequence, response.Status, response.Proto, &response.Header, bytes.NewReader(buffy.Bytes()), time.Since(startedProcessing)}
					return
				}

				_, err = buffy.Write(chunk) // write next chunk, and keep reading in loop

				if err != nil { // return on error in write
					sendErrorResponse(r.Sequence, response.Proto, err, timeout, startedProcessing, batchedResponses)
					return
				}
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
