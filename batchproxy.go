package batchproxy

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"strconv"
	"strings"
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

func MultipartMixed(w http.ResponseWriter, r *http.Request) {
	var batch []*http.Request
	ct, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ct != "multipart/mixed" {
		err = errors.New("unsupported content type, expected multipart/mixed")
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}
	// check for optional timeout header

	tm := r.Header.Get("x-batchproxy-timeout")
	var timeout time.Duration
	if tm != "" {
		timeout, err = time.ParseDuration(tm + "s")
		if err != nil {
			http.Error(w, "invalid value for x-batchproxy-timeout header, expected number of seconds", http.StatusBadRequest)
			return
		}
	} else {
		timeout = time.Duration(20) * time.Second // Default timeout is 20 seconds
	}
	log.Println(timeout)
	boundary, ok := params["boundary"]
	if !ok {
		err = errors.New("missing multipart boundary")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mr := multipart.NewReader(r.Body, boundary)
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		pct, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		if pct != "application/http" {
			err = errors.New("unsupported content type for multipart/mixed content, expected each part to be application/http")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r, err := http.ReadRequest(bufio.NewReader(p))
		// We need to get the protocol from a header in the part's request
		protocol := r.Header.Get("Forwarded")
		if protocol == "" || !strings.Contains(protocol, "proto=http") { // proto must be `http` or `https`
			err = errors.New("missing header in multipart/mixed content, expected each part to contain a Forwarded header with a valid proto value (proto=http or proto=https)")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		parts := strings.Split(protocol, "proto=")
		if len(parts) < 2 || (parts[1] != "http" && parts[1] != "https") {
			err = errors.New("invalid proto value in Forwarded header, expected proto=http or proto=https")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		protocol = parts[1]
		url := protocol + "://" + r.Host + r.RequestURI
		request, err := http.NewRequest(r.Method, url, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		batch = append(batch, request)
	}
	responses, err := ProcessBatch(batch, timeout)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	mw := multipart.NewWriter(w)
	defer mw.Close()
	w.Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())

	var pw io.Writer
	var pb []byte

	for _, next := range responses {
		ph := make(textproto.MIMEHeader)
		ph.Set("Content-Type", "application/http")
		pw, err = mw.CreatePart(ph)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		io.WriteString(pw, next.Proto+" "+next.Status+"\n")
		if next.Header != nil {
			log.Println(next.Header)
			next.Header.Write(pw)
			io.WriteString(pw, "\n")
		}
		if next.Body != nil {
			pb, err = ioutil.ReadAll(next.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			pw.Write(pb)
			io.WriteString(pw, "\n")
		}
	}
}

func DumpRequest(w http.ResponseWriter, r *http.Request) {
	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		log.Fatal(err)
	} else {
		log.Print("<-- DumpRequest -->")
		log.Print(string(dump))
		log.Print("<-- /DumpRequest -->")
	}
}
