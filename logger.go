package cloudwriter

//	Copyright 2016 Matt Ho
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"html/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

// CloudWatchLogs is an interface that provides the minimal shape of *cloudwatchlogs.CloudWatchLogs
// and simplifies testing
type CloudWatchLogs interface {
	PutLogEvents(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error)
	CreateLogGroup(*cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error)
	CreateLogStream(*cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error)
}

type logger struct {
	client        CloudWatchLogs
	batchSize     int
	groupName     *string
	streamName    *string
	cancel        func()
	ctx           context.Context
	wg            *sync.WaitGroup
	ch            chan *cloudwatchlogs.InputLogEvent
	buffer        string
	sequenceToken *string
	debug         func(...interface{})
}

var (
	newline = []byte("\n")
)

const (
	// Maximum number of records to be saved before calling PutLogEvents
	MaxBatchSize = 1000

	// Length of time to wait with no new records before shipping what records we have
	Timeout = time.Second * 15

	nanosPerMilli = 1000000
)

// Write implements the io.Writer interface
func (l *logger) Write(p []byte) (n int, err error) {
	n = len(p)

	if n == 0 {
		return
	}

	for {
		offset := bytes.Index(p, newline)
		if offset == -1 {
			if len(p) > 0 {
				l.buffer = string(p)
			}
			break
		}

		message := string(p[0:offset])
		if len(l.buffer) > 0 {
			message = l.buffer + message
			l.buffer = ""
		}
		event := &cloudwatchlogs.InputLogEvent{
			Timestamp: aws.Int64(time.Now().UnixNano() / nanosPerMilli),
			Message:   aws.String(message),
		}

		select {
		case l.ch <- event:
		default:
		}

		p = p[offset+1:]
	}

	return
}

// Close the writer and any related go routines it may be running
func (l *logger) Close() error {
	l.cancel()
	l.wg.Wait()
	return nil
}

func errCode(err error) string {
	switch v := err.(type) {
	case awserr.Error:
		return v.Code()
	default:
		return ""
	}
}

func (l *logger) start() {
	defer l.wg.Done()

	events := make([]*cloudwatchlogs.InputLogEvent, MaxBatchSize)
	offset := 0
	interval := time.Second * 15

	publishEventsFunc := func() {
		if offset == 0 {
			l.debug("No events queued.  Nothing to publish.")
			return // no events to publish
		}

		if err := l.putLogs(events[0:offset]); err != nil {
			log.Printf("Unable to publish logs, %v\n", err)
		}

		l.debug("Successfully published", offset, "events")
	}

	timer := time.NewTimer(interval)

	for {
		timer.Reset(interval)

		select {
		case <-l.ctx.Done():
			l.debug("Closing publisher goroutine")
			return

		case <-timer.C:
			l.debug("No events received recently.  Sending what we have.")
			publishEventsFunc()
			offset = 0

		case v := <-l.ch:
			l.debug("Received event,", *v.Message)
			events[offset] = v
			offset = offset + 1

			if offset == l.batchSize {
				publishEventsFunc()
				offset = 0
			}
		}

		timer.Stop()
	}
}

func renderStreamName(streamName string) (string, error) {
	t, err := template.New("stream-name").Parse(streamName)
	if err != nil {
		return "", err
	}

	buf := &bytes.Buffer{}
	err = t.Execute(buf, map[string]interface{}{
		"Timestamp": time.Now().Unix(),
	})
	if err != nil {
		return "", err
	}

	return string(buf.Bytes()), nil
}

func (l *logger) createLogGroup() error {
	l.debug("Upserting log group,", *l.groupName)
	_, err := l.client.CreateLogGroup(&cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: l.groupName,
	})
	return err
}

func (l *logger) createLogStream() error {
	l.debug("Upserting log stream,", *l.groupName)
	_, err := l.client.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  l.groupName,
		LogStreamName: l.streamName,
	})
	return err
}

func (l *logger) putLogs(events []*cloudwatchlogs.InputLogEvent) error {
	l.debug("Publishing logs to CloudWatch")
	out, err := l.client.PutLogEvents(&cloudwatchlogs.PutLogEventsInput{
		LogEvents:     events,
		LogGroupName:  l.groupName,
		LogStreamName: l.streamName,
		SequenceToken: l.sequenceToken,
	})

	if err != nil {
		return err
	}

	l.sequenceToken = out.NextSequenceToken
	if l.sequenceToken != nil {
		l.debug("Received SequenceToken,", l.sequenceToken)
	}

	return nil
}

func region() string {
	region := os.Getenv("AWS_REGION")

	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}

	if region == "" {
		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		resp, err := ctxhttp.Get(ctx, http.DefaultClient, "http://169.254.169.254/latest/meta-data/placement/availability-zone")
		if err == nil {
			defer resp.Body.Close()

			data, err := ioutil.ReadAll(resp.Body)
			if err == nil {
				region = strings.TrimSpace(string(data))
				if len(region) > 0 {
					region = region[0 : len(region)-1]
				}
			}
		}
	}

	if region == "" {
		region = "us-east-1"
	}

	return region
}

// New instantiates a new io.WriteCloser instance that asynchronously writes records
// to CloudWatchLogs.  cloudwriter assumes that records will be divided using a newline
// character.
//
// client is an optional instance of *cloudwatchlogs.CloudWatchLogs
//
// streamName supports go template style interpolation with {{ .Timestamp }}
//
func New(client CloudWatchLogs, groupName, streamName string) (io.WriteCloser, error) {
	if client == nil {
		cfg := &aws.Config{Region: aws.String(region())}

		client = cloudwatchlogs.New(session.New(cfg))
	}

	interpolatedStreamName, err := renderStreamName(streamName)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	l := &logger{
		client:     client,
		batchSize:  MaxBatchSize,
		groupName:  aws.String(groupName),
		streamName: aws.String(interpolatedStreamName),
		cancel:     cancel,
		ctx:        ctx,
		wg:         &sync.WaitGroup{},
		ch:         make(chan *cloudwatchlogs.InputLogEvent, 4096),
		debug:      func(...interface{}) {},
	}

	if err := l.createLogGroup(); err != nil && errCode(err) != "ResourceAlreadyExistsException" {
		return nil, err
	}
	if err := l.createLogStream(); err != nil && errCode(err) != "ResourceAlreadyExistsException" {
		return nil, err
	}

	l.wg.Add(1)
	go l.start()

	return l, nil
}

// The default batch size is MaxBatchSize.  While this should be suitable for most
// cases, you have the option of changing this.
func WithBatchSize(w io.WriteCloser, batchSize int) io.WriteCloser {
	switch v := w.(type) {
	case *logger:
		v.batchSize = batchSize
		return v
	default:
		return w
	}
}

// For testing, enables debug messages to be printed.
func WithDebug(w io.WriteCloser, debug func(...interface{})) io.WriteCloser {
	switch v := w.(type) {
	case *logger:
		v.debug = debug
		return v
	default:
		return w
	}
}
