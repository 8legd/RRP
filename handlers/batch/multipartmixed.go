package batch

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/8legd/RRP/processors"
)

// MultipartMixed handles a batch of HTTP requests in `multipart/mixed` format.
// Each part contains `application/http` content representing an individual request.
// Once processed, HTTP responses are returned as `application/http` content in
// the same sequence as the corresponding requests.
func MultipartMixed(w http.ResponseWriter, r *http.Request) {
	var batch []*http.Request
	ct, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ct != "multipart/mixed" {
		err = errors.New("unsupported content type, expected multipart/mixed")
		log.Println(err)
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}
	// check for optional timeout header
	tm := r.Header.Get("x-batchproxy-timeout")
	var timeout time.Duration
	if tm != "" {
		timeout, err = time.ParseDuration(tm + "s")
		if err != nil {
			log.Println(err)
			http.Error(w, "invalid value for x-batchproxy-timeout header, expected number of seconds", http.StatusBadRequest)
			return
		}
	} else {
		timeout = time.Duration(20) * time.Second // Default timeout is 20 seconds
	}
	boundary, ok := params["boundary"]
	if !ok {
		err = errors.New("missing multipart boundary")
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	mr := multipart.NewReader(r.Body, boundary)
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			if len(batch) < 1 {
				err = errors.New("invalid multipart content")
				log.Println(err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			break // finished reading multpart parts
		}
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// check next part's contet type
		pct, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if pct != "application/http" {
			err = errors.New("unsupported content type for multipart/mixed content, expected each part to be application/http")
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		pr, err := http.ReadRequest(bufio.NewReader(p))
		// we need to get the protocol from a header in the part's request
		protocol := pr.Header.Get("Forwarded")
		if protocol == "" || !strings.Contains(protocol, "proto=http") { // proto must be `http` or `https`
			err = errors.New("missing header in multipart/mixed content, expected each part to contain a Forwarded header with a valid proto value (proto=http or proto=https)")
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		parts := strings.Split(protocol, "proto=")
		if len(parts) < 2 || (parts[1] != "http" && parts[1] != "https") {
			err = errors.New("invalid proto value in Forwarded header, expected proto=http or proto=https")
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		protocol = parts[1]
		url := protocol + "://" + pr.Host + pr.RequestURI
		// read part's body
		// NOTE: if there is no Content-Length header the body will not have been read (will be empty)
		pb, err := ioutil.ReadAll(pr.Body)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		request, err := http.NewRequest(r.Method, url, bytes.NewBuffer(pb))
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		// add headers
		request.Header = pr.Header
		batch = append(batch, request)
	}
	responses, err := processors.ProcessBatch(batch, timeout)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// send a multipart response back
	mw := multipart.NewWriter(w)
	defer mw.Close()
	w.Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())

	var pw io.Writer
	var pb []byte

	// the individual response are sent as `application/http` as per requests
	for _, next := range responses {
		ph := make(textproto.MIMEHeader)
		ph.Set("Content-Type", "application/http")
		pw, err = mw.CreatePart(ph)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		io.WriteString(pw, next.Proto+" "+next.Status+"\r\n")
		if next.Header != nil {
			next.Header.Write(pw)
			io.WriteString(pw, "\r\n")
		}
		if next.Body != nil {
			pb, err = ioutil.ReadAll(next.Body)
			if err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			pw.Write(pb)
			io.WriteString(pw, "\r\n")
		}
	}
}
