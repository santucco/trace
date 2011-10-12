// Copyright (c) 2011 Alexander Sychev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package trace provides a simple debug trace output
//
// A sample of using: 
//
// in a separate file, i.e. tracer.go:
//	const (
//		debTrace = trace.Next << iota
//		debThis
//		debThat
//		debAll = (debThat - 1) | debThat
//	)
//	var tracer trace.Tracer
//	func init() {
//		tracer.TraceLevel = trace.Frame // turn the trace of frames on
//		tracer.Prefix = "prefix: "
//		tracer.TraceSource = true // turn stack calls of trace on
//		tracer.FrameSource = true // turn stack calls of Frames on
//		tracer.CallersSource = 2  // depth of stack is 1
//	}
// in a source code:
//	func Foo(){
//		defer tracer.Exit(tracer.Enter()) //this is tracer for entrance to frame and exit from frame
//		tracer.Trace(debTrace, "output only for debTrace level, tracer: %#v", tracer)
//		tracer.Trace(debTrace|debThis, "output only for debTrace|debThis level, tracer: %#v", tracer)
//		tracer.Trace(debAll, "output only for all levels, tracer: %#v", tracer)
//	}
// trace level can be changed on the fly:
//	func AnotherFoo(){
//		defer tracer.Exit(tracer.Enter())
//		tracer.TraceLevel |= debTrace
//		Foo()
//		tracer.TraceLevel |= debThis
//		Foo()
//		tracer.TraceLevel |= debAll
//		Foo()
//	}
// a sample of produced traces:
//	prefix: main.Foo: enter
//	at /home/santucco/work/go/test/test.go:23
//	at /home/santucco/work/go/test/test.go:34
//	prefix: main.Foo: output only for debTrace level, tracer: trace.Tracer{TraceLevel:0x7, Prefix:"prefix: ", FrameSource:true, TraceSource:true, CallersSource:0x2}
//	at /home/santucco/work/go/test/test.go:25
//	at /home/santucco/work/go/test/test.go:34
//	prefix: main.Foo: output only for debTrace|debThis level, tracer: trace.Tracer{TraceLevel:0x7, Prefix:"prefix: ", FrameSource:true, TraceSource:true, CallersSource:0x2}
//	at /home/santucco/work/go/test/test.go:26
//	at /home/santucco/work/go/test/test.go:34
//	prefix: main.Foo: exit
//	at /home/santucco/work/go/test/test.go:27
//	at /home/santucco/work/go/test/test.go:34
package trace

import (
	"os"
	"fmt"
	"runtime"
)

const (
	Frame = 1 << iota // Frame is a trace level for printing frames
	Next              // Next is a origin trace level for other trace levels
)

type Tracer struct {
	TraceLevel    uint   // The current trace level
	Prefix        string // The prefix of output strings.
	FrameSource   bool   // The flag of printing frames for frame traces
	TraceSource   bool   // The flag of printing frames for traces
	CallersSource uint   // The count of printing frames
}

var outchan chan string
var donechan chan bool

func init() {
	Start()
}

// Enter prints a trace about an entrance in the frame
func (this *Tracer) Enter() uintptr {
	if (this.TraceLevel & Frame) == 0 {
		return 0
	}
	return this.trace(2, "enter", this.FrameSource)
}

// Exit in a conjunction with defer prints a trace about an exit from the frame
func (this *Tracer) Exit(pc uintptr) {
	if (this.TraceLevel&Frame) == 0 {
		return
	}
	
	if x := recover(); x != nil {
		this.trace(2, "panic exit", false)
		panic(x)
	} else {
		this.trace(2, "exit", this.FrameSource)
	}
}

// Start starts tracing
func Start() {
	if outchan != nil {
		return
	}
	outchan = make(chan string, 10)
	go func() {
		for true {
			if s, ok := <-outchan; ok {
				fmt.Fprint(os.Stderr, s)
			} else {
				break
			}
		}
		donechan <- true
	}()
}

// Stop stops all tracing and wait until all trace messages are printed
func Stop(){
	if outchan == nil {
		return
	}
	donechan = make(chan bool)
	close(outchan)
	<- donechan
	close(donechan)
	outchan = nil
	donechan = nil
}

// Trace prints a formatted message, f is a format of the message, v are interfaces with data fields.
func (this *Tracer) Trace(l uint, f string, v ...interface{}) {
	if (l&this.TraceLevel) == 0 || (l & ^this.TraceLevel) != 0 {
		return
	}
	this.trace(2, fmt.Sprintf(f, v...), this.TraceSource)
}

// TraceFunc repeatedly calls f until second result is true and prints obtained strings
func (this *Tracer) TraceFunc(l uint, f func() (string, bool)) {
	if f == nil || (l&this.TraceLevel) == 0 || (l & ^this.TraceLevel) != 0 {
		return
	}
	for s, ok := f(); ok; s, ok = f() {
		this.trace(2, s, this.TraceSource)
	}
}

func (this *Tracer) trace(c int, msg string, src bool) uintptr {
	if outchan == nil {
		return 0
	}
	pc, _, _, ok := runtime.Caller(c)
	if !ok {
		outchan <- this.Prefix + msg + "\n"
		return 0
	}
	fnc := runtime.FuncForPC(pc)
	if fnc == nil {
		outchan <- this.Prefix + msg + "\n"
		return pc
	}
	name := fnc.Name()
	if !src {
		outchan <- fmt.Sprintf("%s%s: %s\n", this.Prefix, name, msg)
		return pc
	}
	file, line := fnc.FileLine(pc)
	s := fmt.Sprintf("%s%s: %s\n\tat %s:%d\n", this.Prefix, name, msg, file, line)
	if this.CallersSource > 0 {
		i := c + 1
		c += int(this.CallersSource)
		for _, file, line, ok := runtime.Caller(i); ok && i < c; _, file, line, ok = runtime.Caller(i) {
			s += fmt.Sprintf("\tat %s:%d\n", file, line)
			i++
		}
	}

	outchan <- s
	return pc
}
