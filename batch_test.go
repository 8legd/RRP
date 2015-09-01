package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/8legd/RRP/servers/goji"
)

func TestBatch(t *testing.T) {

	// TODO move tests to go

	go func() {
		goji.Start("127.0.0.1:8000")
	}()

	// wait a couple of seconds for goji to start
	wait := time.NewTimer(time.Duration(2) * time.Second)
	<-wait.C

	// run our tests...

	// first check our error responses
	req, err := http.NewRequest("POST", "http://127.0.0.1:8000/batch/multipartmixed",
		bytes.NewBuffer([]byte(":(")))

	client := &http.Client{}
	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		panic(err)
	}

	fmt.Println("response Status:", res.Status)

	body, _ := ioutil.ReadAll(res.Body)
	fmt.Println("response Body:", string(body))

	os.Exit(0)

}
