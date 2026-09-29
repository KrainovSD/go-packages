package web

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func BenchmarkWriterMiddlewareCompress(b *testing.B) {
	var body = compressibleBody(64 << 10)
	benchmarkWriterRun(b, benchmarkWriterHandler(&WriterMiddlewareOptions{
		Compress:      true,
		CompressLevel: gzip.DefaultCompression,
	}, body))
}

func BenchmarkWriterMiddlewareCompressMinSize(b *testing.B) {
	var body = compressibleBody(3 << 10)
	benchmarkWriterRun(b, benchmarkWriterHandler(&WriterMiddlewareOptions{
		Compress:        true,
		CompressMinSize: 4 << 10,
	}, body))
}

func BenchmarkWriterMiddlewareCompressLevels(b *testing.B) {
	var body = compressibleBody(64 << 10)
	var levels = []int{gzip.BestSpeed, 6, gzip.BestCompression}
	for _, level := range levels {
		b.Run(strconv.Itoa(level), func(b *testing.B) {
			benchmarkWriterRun(b, benchmarkWriterHandler(&WriterMiddlewareOptions{
				Compress:      true,
				CompressLevel: level,
			}, body))
		})
	}
}

func benchmarkWriterHandler(opts *WriterMiddlewareOptions, body []byte) http.Handler {
	var middleware = NewWriterMiddleware(opts)
	return middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
}

func benchmarkWriterRun(b *testing.B, handler http.Handler) {
	var request = httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var recorder = httptest.NewRecorder()
		recorder.Body = bytes.NewBuffer(make([]byte, 0, 1<<20))
		handler.ServeHTTP(recorder, request)
	}
}
