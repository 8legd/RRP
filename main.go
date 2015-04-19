package main

import (
	"bufio"
	"fmt"
	"io"
	//"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"sync"

	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web"
)

type BatchedRequest struct {
	Sequence int
	Request  *http.Request
}

type BatchedResponse struct {
	Sequence int
	Response *http.Response
}

func ProcessBatch(requests []*http.Request) ([]*http.Response, error) {
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
			client := &http.Client{}
			response, err := client.Do(r.Request)
			if err != nil {
				// Create an error response for any HTTP Client errors - Status 400 (Bad Request)
				errorResponse := http.Response{}
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
	} else {
		err := fmt.Errorf("expected %d responses for this batch but only recieved %d", z, len(batchedResponses))
		return nil, err
	}
}

func MultipartMixed(c web.C, w http.ResponseWriter, r *http.Request) {
	var batch []*http.Request
	ct, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || ct != "multipart/mixed" {
		log.Fatal("TODO return error status code invalid content type - multipart/mixed instead")
	}
	boundary, ok := params["boundary"]
	if !ok {
		log.Fatal("TODO return error status code missing multipart boundary instead")
	}
	mr := multipart.NewReader(r.Body, boundary)
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal("TODO return error status code and error detail", err)
		}
		pct, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if err != nil || pct != "application/http" {
			log.Fatal("TODO return error status code invalid multipart content")
		}
		r, err := http.ReadRequest(bufio.NewReader(p))
		// We need to get the protocol from a header in the part's request
		protocol := r.Header.Get("X-Forwarded-Proto")
		if protocol == "" {
			log.Fatal("TODO return error status code missing X-Forwarded-Proto header")
		}
		url := protocol + "://" + r.Host + r.RequestURI
		request, err := http.NewRequest(r.Method, url, r.Body)
		if err != nil {
			log.Fatal("TODO return error status code and error detail", err)
		}
		batch = append(batch, request)
	}
	responses, err := ProcessBatch(batch)
	if err != nil {
		log.Fatal("TODO return error status code and error detail", err)
	}
	for _, next := range responses {
		log.Println(next.Status)
		//defer next.Body.Close()

		//log.Println("response Status:", next.Status)
		//log.Println("response Headers:", next.Header)
		//body, _ := ioutil.ReadAll(next.Body)
		//log.Println("response Body:", string(body))
	}
}

func main() {
	goji.Post("/multipart/mixed", MultipartMixed)
	// TODO support other batch requests e.g. AJAX support?
	goji.Serve()
}
