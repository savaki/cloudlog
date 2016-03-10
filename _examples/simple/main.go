package main

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/savaki/cloudlog"
	"github.com/savaki/cloudwriter"
)

func main() {
	w, err := cloudwriter.New(nil, "sample", fmt.Sprintf("logs-%v", time.Now().Unix()))
	if err != nil {
		log.Fatalln(err)
	}

	w = cloudlog.WithBatchSize(w, 1)
	defer w.Close()

	io.WriteString(w, "hello world\n")
	io.WriteString(w, "the time has come the walrus said\n")
	io.WriteString(w, "to speak of many things\n")

	time.Sleep(time.Second)
}
