package batch

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/8legd/RRP/logging/elf"
	"github.com/8legd/RRP/processors"
)

// MultipartMixed handles a batch of HTTP requests in `multipart/mixed` format.
// Each part contains `application/http` content representing an individual request.
// Once processed, HTTP responses are returned as `application/http` content in
// the same sequence as the corresponding requests.
func MultipartMixed(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	var batch []*http.Request
	ct, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		elf.Log("ERROR", "Error parsing `Content-Type` header of batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ct != "multipart/mixed" {
		err = errors.New("unsupported content type, expected multipart/mixed")
		elf.Log("ERROR", "Error parsing `Content-Type` header of batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}
	boundary, ok := params["boundary"]
	if !ok {
		err = errors.New("missing multipart boundary")
		elf.Log("ERROR", "Error parsing `Content-Type` header of batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// check for optional timeout header
	tm := r.Header.Get("x-rrp-timeout")
	var timeout time.Duration
	if tm != "" {
		timeout, err = time.ParseDuration(tm + "s")
		if err != nil {
			elf.Log("ERROR", "Error parsing `x-rrp-timeout` header of batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
			http.Error(w, "invalid value for x-rrp-timeout header, expected number of seconds", http.StatusBadRequest)
			return
		}
		elf.Log("INFO", "Timeout specified is "+strconv.FormatFloat(timeout.Seconds(), 'f', 3, 64), elf.LogOptions{})

	} else {
		timeout = time.Duration(20) * time.Second // Default timeout is 20 seconds
		elf.Log("INFO", "Default timeout is "+strconv.FormatFloat(timeout.Seconds(), 'f', 3, 64), elf.LogOptions{})
	}
	mr := multipart.NewReader(r.Body, boundary)
	var urls []string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			if len(batch) < 1 {
				err = errors.New("invalid multipart content")
				elf.Log("ERROR", "Error parsing content of batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			break // finished reading multpart parts
		}
		if err != nil {
			elf.Log("ERROR", "Error parsing content of batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// check next part's content type
		pct, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if err != nil {
			elf.Log("ERROR", "Error reading next parts `Content-Type` header in batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if pct != "application/http" {
			err = errors.New("unsupported content type for multipart/mixed content, expected each part to be application/http")
			elf.Log("ERROR", "Error reading next parts `Content-Type` header in batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		pr, err := http.ReadRequest(bufio.NewReader(p))
		// we need to get the protocol from a header in the part's request
		protocol := pr.Header.Get("Forwarded")
		if protocol == "" || !strings.Contains(protocol, "proto=http") { // proto must be `http` or `https`
			err = errors.New("missing header in multipart/mixed content, expected each part to contain a Forwarded header with a valid proto value (proto=http or proto=https)")
			elf.Log("ERROR", "Missing part header in batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		parts := strings.Split(protocol, "proto=")
		if len(parts) < 2 || (parts[1] != "http" && parts[1] != "https") {
			err = errors.New("invalid proto value in Forwarded header, expected proto=http or proto=https")
			elf.Log("ERROR", "Invalid part header in batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		protocol = parts[1]
		url := protocol + "://" + pr.Host + pr.RequestURI
		urls = append(urls, url)
		// read part's body
		// NOTE: if there is no Content-Length header the body will not have been read (will be empty)
		pb, err := ioutil.ReadAll(pr.Body)
		if err != nil {
			elf.Log("ERROR", "Error parsing content of batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		request, err := http.NewRequest(r.Method, url, bytes.NewBuffer(pb))
		if err != nil {
			elf.Log("ERROR", "Error reading individual request from content in batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// add headers
		request.Header = pr.Header
		batch = append(batch, request)
	}

	responses, err := processors.ProcessBatch(batch, timeout)
	if err != nil {
		elf.Log("ERROR", "Error processing batch from batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// create some variables to keep track of the indvidual responses as we process them
	// (these are used for reporting via log output e.g. should a runtime panic occur)
	var nextIndex int
	var nextResponse *processors.BatchedResponse

	// send a multipart response back
	// (We need to buffer this in case there is an error. If we didn't and wrote
	// directly to the response stream it would implicitly set a status of 200 OK)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	defer func() {
		mw.Close()

		// Report state on any panic
		if r := recover(); r != nil {
			// TODO send this to ELF based logger via payload
			fmt.Println("Reporting state on panic", r)
			fmt.Println("Request", batch[nextIndex])
			fmt.Println("Request URL", urls[nextIndex])
			fmt.Println("Response", nextResponse)

			err = errors.New("panic while processing request")
			elf.Log("ERROR", "Panic whilst processing batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}()

	w.Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())

	var pw io.Writer

	// the individual response are sent as `application/http` as per requests
	for nextIndex, nextResponse = range responses {
		ph := make(textproto.MIMEHeader)
		ph.Set("Content-Type", "application/http")
		pw, err = mw.CreatePart(ph)
		if err != nil {
			elf.Log("ERROR", "Error whilst processing batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if nextResponse != nil {

			elf.Log("INFO", "Received "+nextResponse.Status+" from "+urls[nextIndex], elf.LogOptions{Started: time.Now().Add(nextResponse.ProcessingDuration * -1)})

			io.WriteString(pw, nextResponse.Proto+" "+nextResponse.Status+"\r\n")
			if nextResponse.Header != nil {
				nextResponse.Header.Write(pw)
				io.WriteString(pw, "\r\n")
			}
			if nextResponse.Body != nil {
				pb, err := ioutil.ReadAll(nextResponse.Body)
				if err != nil {
					elf.Log("ERROR", "Error whilst reading batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				pw.Write(pb)
				io.WriteString(pw, "\r\n")
			}
		} else {
			err = errors.New("missing response for " + urls[nextIndex])
			elf.Log("ERROR", "Error whilst processing batch/multipartmixed request", elf.LogOptions{Cause: err, Started: started})
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// write response
	w.Write(buf.Bytes())
	return
}
