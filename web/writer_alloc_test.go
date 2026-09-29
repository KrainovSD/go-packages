//go:build !race

package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"runtime"
	"runtime/debug"
	"testing"
)

func allocBytesTotal(opts *WriterMiddlewareOptions, body []byte, requests int) uint64 {
	var handler = benchmarkWriterHandler(opts, body)
	var request = httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	runtime.GC()
	var percent = debug.SetGCPercent(-1)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < requests; i++ {
		var recorder = httptest.NewRecorder()
		recorder.Body = bytes.NewBuffer(make([]byte, 0, 1<<20))
		handler.ServeHTTP(recorder, request)
	}
	runtime.ReadMemStats(&after)
	debug.SetGCPercent(percent)
	return after.TotalAlloc - before.TotalAlloc
}

func TestWriterMiddlewareSteadyStateAllocs(t *testing.T) {
	var body = compressibleBody(64 << 10)
	var requests = 100
	allocBytesTotal(&WriterMiddlewareOptions{Compress: true}, body, 10)
	var plain = allocBytesTotal(&WriterMiddlewareOptions{}, body, 100)
	var compressed = allocBytesTotal(&WriterMiddlewareOptions{Compress: true}, body, 100)
	var delta = (compressed - plain) / uint64(requests)
	if delta > 64<<10 {
		t.Errorf("compressed responses allocate %d extra bytes per request, expected <= 65536", delta)
	}
}
