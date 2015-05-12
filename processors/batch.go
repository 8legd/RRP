package processors

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type BatchedRequest struct {
	Sequence int
	Request  *http.Request
}

type BatchedResponse struct {
	Sequence int
	Response *http.Response
}

func ProcessBatch(requests []*http.Request, timeout time.Duration) ([]*http.Response, error) {
	z := len(requests)
	// Setup a buffered channel to queue up the requests for processing by individual HTTP Client goroutines
	batchedRequests := make(chan BatchedRequest, z)
	for i := 0; i < z; i++ {
		batchedRequests <- BatchedRequest{i, requests[i]}
	}
	// Close the channel - nothing else is sent to it
	close(batchedRequests)
	// Setup a second buffered channel for collecting the BatchedResponses from the individual HTTP Client goroutines
	batchedResponses := make(chan BatchedResponse, z)
	// Setup a wait group so we know when all the BatchedRequests have been processed
	var wg sync.WaitGroup
	wg.Add(z)
	// Start our individual HTTP Client goroutines to process the BatchedRequests
	for i := 0; i < z; i++ {
		go func() {
			defer wg.Done()
			r := <-batchedRequests
			client := &http.Client{Timeout: timeout}
			response, err := client.Do(r.Request)
			if err != nil {
				// Create an error response for any HTTP Client errors - Status 400 (Bad Request)
				errorResponse := http.Response{}
				errorResponse.Proto = r.Request.Proto
				errorResponse.StatusCode = http.StatusBadRequest
				errorResponse.Status = strconv.Itoa(http.StatusBadRequest) + " " + err.Error()
				batchedResponses <- BatchedResponse{r.Sequence, &errorResponse}
			} else {
				batchedResponses <- BatchedResponse{r.Sequence, response}
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
