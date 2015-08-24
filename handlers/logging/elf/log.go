package elf

import (
	"fmt"
	"strconv"
	"time"
)

func init() {
	fmt.Println("#Version: 1.0")
	fmt.Println("#Fields: date time event message parent-event time-taken payload cause")
}

// Log sends the specified parameters to standard output in ELF format (http://www.w3.org/TR/WD-logfile.html)
// #Version: 1.0
// #Fields: date time event message parent-event time-taken payload cause
//
// `date` and `time` fields are generated when calling the Log function
// `event` and `message` fields are required
// all other fields are optional and provided using `LogOptions`
func Log(event string, message string, options LogOptions) {
	now := time.Now()
	parentEvent := "-"
	if options.ParentEvent != "" {
		parentEvent = options.ParentEvent
	}
	timeTaken := "-"
	if !options.Started.IsZero() {
		timeTaken = strconv.FormatFloat(time.Since(options.Started).Seconds(), 'f', 3, 64)
	}
	payload := "-"
	if options.Payload != "" {
		payload = options.Payload
	}
	cause := "-"
	if options.Cause != nil {
		// TODO stacktrace etc...
		cause = options.Cause.Error()
	}
	fmt.Printf("%d-%02d-%02d\t%02d:%02d:%02d\t%s\t%s\t%s\t%s\t%s\t%s\n",
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second(),
		event, message, parentEvent, timeTaken, payload, cause)
}

// Optional parameters for Log function
type LogOptions struct {
	ParentEvent string
	Started     time.Time
	Payload     string
	Cause       error
}
