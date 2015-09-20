package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/8legd/RRP/servers/goji"
)

const tick = "\u2713"
const cross = "\u2717"

type testResponse struct {
	Err        error
	StatusCode int
	Header     *http.Header
	Body       []byte
}

func postBatchMultipartMixed(client *http.Client, requestID string, content string) testResponse {
	req, err := http.NewRequest("POST", "http://127.0.0.1:8000/batch/multipartmixed", bytes.NewBuffer([]byte(content)))

	var res *http.Response
	var statusCode int
	var header *http.Header
	var body []byte

	if err == nil {
		if requestID != "" {
			req.Header.Set("X-Request-Id", requestID)
		}
		res, err = client.Do(req)
		defer res.Body.Close()
		statusCode = res.StatusCode
		header = &res.Header
		body, err = ioutil.ReadAll(res.Body)
	}

	return testResponse{err, statusCode, header, body}

}

func TestBatchMultipartMixed(t *testing.T) {

	// TODO move tests to go - probably makes sense to write a go client to RRP first

	go func() {
		goji.Start("127.0.0.1:8000")
	}()

	t.Log("Once the `goji` web framework has started on 127.0.0.1:8000")
	{
		// wait a second for it to start
		wait := time.NewTimer(time.Duration(1) * time.Second)
		<-wait.C
	}

	t.Log("We should be able to make requests to batch/multipartmixed")
	{
		client := &http.Client{}
		requestId := "9d429a48-27ea-4954-8a25-592db4c9538b"
		t.Logf("\tWhen sending a `X-Request-Id` header of `%s`", requestId)
		{
			res := postBatchMultipartMixed(client, requestId, ":)")
			if res.Header.Get("X-Request-Id") == requestId {
				t.Log("\t\tShould receive this in the response header", tick)
			} else {
				t.Errorf("\t\tShould receive this in the response header, but received `%s` %v", res.Header.Get("X-Request-Id"), cross)
			}

		}
	}

	t.Log("TODO: continue moving test scripts to Go - pending writing of go client")

}
