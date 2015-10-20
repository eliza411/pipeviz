// +build debug

package main

import (
	"expvar"
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"time"

	"github.com/tag1consulting/pipeviz/Godeps/_workspace/src/github.com/Sirupsen/logrus"
	"github.com/tag1consulting/pipeviz/broker"
	"github.com/tag1consulting/pipeviz/represent"
	"github.com/tag1consulting/pipeviz/types/system"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)

	startTime := time.Now().UTC()
	expvar.Publish("Uptime", expvar.Func(func() interface{} { return int64(time.Since(startTime)) }))

	expvar.Publish("Goroutines", expvar.Func(func() interface{} { return runtime.NumGoroutine() }))

	// subscribe to the broker in order to report data about current graph
	c := broker.Get().Subscribe()

	// Instantiate a real, empty graph to ensure the interface type is never nil when it might be called
	var g system.CoreGraph = represent.NewGraph()
	go func() {
		g = <-c
	}()
	expvar.Publish("MsgId", expvar.Func(func() interface{} { return g.MsgId() }))

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}
