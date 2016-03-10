# cloudwriter

[![Build Status](https://snap-ci.com/savaki/cloudwriter/branch/master/build_image)](https://snap-ci.com/savaki/cloudwriter/branch/master)
[![GoDoc](https://godoc.org/github.com/savaki/cloudwriter?status.svg)](https://godoc.org/github.com/savaki/cloudwriter)

cloudwriter is a implementation of io.Reader that ships data to AWS CloudWatch
 
## Example 

Assuming that you've set AWS credentials via the environment, the following example
writes a simple 
 
```
package main

import (
	"io"
	"log"
	"time"

	"github.com/savaki/cloudwriter"
)

func main() {
	w, err := cloudwriter.New(nil, "sample-group", "sample-stream-{{.Timestamp}}")
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
```