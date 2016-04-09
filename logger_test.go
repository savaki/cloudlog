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
	"io"
	"log"
	"regexp"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

var (
	testGroupName  = "blah-group"
	testStreamName = "blah-stream"
)

func Log(args ...interface{}) {
	log.Println(args...)
}

type Mock struct {
	Inputs []*cloudwatchlogs.PutLogEventsInput
}

func (m *Mock) PutLogEvents(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
	if m.Inputs == nil {
		m.Inputs = []*cloudwatchlogs.PutLogEventsInput{}
	}
	m.Inputs = append(m.Inputs, in)
	return &cloudwatchlogs.PutLogEventsOutput{}, nil
}

func (m *Mock) CreateLogGroup(*cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	return &cloudwatchlogs.CreateLogGroupOutput{}, nil
}

func (m *Mock) CreateLogStream(*cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	return &cloudwatchlogs.CreateLogStreamOutput{}, nil
}

type NoOp struct{}

func (n NoOp) PutLogEvents(in *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
	return &cloudwatchlogs.PutLogEventsOutput{}, nil
}

func (n NoOp) CreateLogGroup(*cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	return &cloudwatchlogs.CreateLogGroupOutput{}, nil
}

func (n NoOp) CreateLogStream(*cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	return &cloudwatchlogs.CreateLogStreamOutput{}, nil
}

func TestLogSimpleEvent(t *testing.T) {
	mock := &Mock{}
	w, err := New(mock, testGroupName, testStreamName,
		BatchSize(1),
		Debug(Log),
	)
	if err != nil {
		t.Error(err)
		return
	}
	defer w.Close()

	io.WriteString(w, "hello world\n")
	time.Sleep(time.Millisecond * 50) // just need to give it to the other go-routine for a moment

	if v := len(mock.Inputs); v != 1 {
		t.Errorf("expected 1 event; got %v", v)
	}
	if v := len(mock.Inputs[0].LogEvents); v != 1 {
		t.Errorf("expected 1 event; got %v", v)
	}
}

func TestLogMultiline(t *testing.T) {
	mock := &Mock{}
	w, err := New(mock, testGroupName, testStreamName,
		BatchSize(2),
		Debug(Log),
	)
	if err != nil {
		t.Error(err)
		return
	}
	defer w.Close()

	io.WriteString(w, "hello\nworld\n")
	time.Sleep(time.Millisecond * 50) // just need to give it to the other go-routine for a moment

	if v := len(mock.Inputs); v != 1 {
		t.Errorf("expected 1 event; got %v", v)
	}
	if v := len(mock.Inputs[0].LogEvents); v != 2 {
		t.Errorf("expected 2 events; got %v", v)
	}
}

func TestLogPartial(t *testing.T) {
	mock := &Mock{}
	w, err := New(mock, testGroupName, testStreamName,
		BatchSize(1),
		Debug(Log),
	)
	if err != nil {
		t.Error(err)
		return
	}
	defer w.Close()

	io.WriteString(w, "hello ")
	io.WriteString(w, "world\n")
	time.Sleep(time.Millisecond * 50) // just need to give it to the other go-routine for a moment

	if v := len(mock.Inputs); v != 1 {
		t.Errorf("expected 1 event; got %v", v)
	}
	if v := len(mock.Inputs[0].LogEvents); v != 1 {
		t.Errorf("expected 1 events; got %v", v)
	}
	if v := *mock.Inputs[0].LogEvents[0].Message; v != "hello world" {
		t.Errorf("expected 'hello world' events; got '%v'", v)
	}
}

func BenchmarkWriteToClosedWriter(t *testing.B) {
	mock := &Mock{}
	w, err := New(mock, testGroupName, testStreamName)
	if err != nil {
		t.Error(err)
		return
	}

	w.Close()

	for i := 0; i < t.N; i++ {
		io.WriteString(w, "hello ")
	}
}

func BenchmarkLogger(t *testing.B) {
	noOp := NoOp{}
	w, err := New(noOp, testGroupName, testStreamName)
	if err != nil {
		t.Error(err)
		return
	}

	defer w.Close()

	for i := 0; i < t.N; i++ {
		io.WriteString(w, "hello world")
	}
}

func TestStripLastChar(t *testing.T) {
	region := "us-east-1a"

	if region[0:len(region)-1] != "us-east-1" {
		t.Error("expected to strip off the last rune")
	}
}

func TestRenderStreamName(t *testing.T) {
	streamName, err := renderStreamName("test-{{.Timestamp}}")
	if err != nil {
		t.Error(err)
		return
	}

	pattern := regexp.MustCompile(`test-\d+`)
	if !pattern.MatchString(streamName) {
		t.Error("expected stream name to have been interpolated")
	}
}
