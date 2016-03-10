package main

import (
	"io"
	"log"
	"time"

	"github.com/savaki/cloudwriter"
)

func main() {
	w, err := cloudwriter.New(nil, "sample-group", "sample-stream")
	if err != nil {
		log.Fatalln(err)
	}

	w = cloudwriter.WithBatchSize(w, 1)
	defer w.Close()

	io.WriteString(w, "hello world\n")
	io.WriteString(w, "the time has come the walrus said\n")
	io.WriteString(w, "to speak of many things\n")

	time.Sleep(time.Second) // pause to give the async cloudwriter time to write
}
