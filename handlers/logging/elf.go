package logging

import (
	"fmt"
	"strconv"
	"time"
)

func Elfi() {
	fmt.Println("#Version: 1.0")
	fmt.Println("#Fields: date time event time-taken status context")
}

func Elfl(event string, timeTaken time.Duration, status string, context string) {
	//TODO data and time formatting
	//TODO switch to use `fmt`
	fmt.Println(event + "\t" + strconv.FormatFloat(timeTaken.Seconds(), 'f', 3, 64) + "\t" + status + "\t" + context)
	//log.Println("ELF::" + event + "\t" + strconv.FormatFloat(timeTaken.Seconds(),'f', 3, 64) + "\t" + status + "\t" + context)
}
