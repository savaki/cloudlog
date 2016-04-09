package main

import (
	"io"
	"log"
	"time"

	"github.com/savaki/cloudwriter"
)

func main() {
	w, err := cloudwriter.New(nil, "sample-group", "sample-stream-{{.Timestamp}}",
		cloudwriter.BatchSize(1),
		cloudwriter.Interval(500*time.Millisecond),
	)
	if err != nil {
		log.Fatalln(err)
	}
	defer w.Close()

	io.WriteString(w, "hello world\n")
	io.WriteString(w, "the time has come the walrus said\n")
	io.WriteString(w, "to speak of many things\n")

	time.Sleep(time.Second) // pause to give the async cloudwriter time to write
}
