package elf

import (
	"fmt"
	"testing"
	"time"
)

func TestLog(t *testing.T) {
	// TODO capture standard output to check all is as expected

	started := time.Now()

	Log("START", "Started thing", LogOptions{})
	Log("REQUEST", "Handling Request", LogOptions{})
	Log("READ", "Reading Request Body", LogOptions{
		Tags:    "SOMETAG",
		Payload: "extra information",
		Started: started,
	})
	err := fmt.Errorf("oops")
	Log("ERROR", "Unexpected error", LogOptions{Cause: err, Started: started})

}
