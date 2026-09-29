package web

import (
	"compress/gzip"
	"log/slog"
	"net/http"
)

type shouldCompress = func(w http.ResponseWriter) bool

type WriterMiddlewareOptions struct {
	CompressLevel   int
	Compress        bool
	CompressMinSize int
	ShouldCompress  func(w http.ResponseWriter) bool
}

type WriterMiddleware struct {
	compressLevel  int
	compress       bool
	shouldCompress shouldCompress
}

const WriterMiddlewareID = "ksd-writer"

var prefixes = [7]string{
	"application/json",
	"text/",
	"application/javascript",
	"image/svg+xml",
	"application/xml",
	"application/graphql",
	"application/x-toml",
}

func shouldCompressFn(w http.ResponseWriter) bool {
	var contentType = w.Header()["Content-Type"]
	if len(contentType) == 0 || contentType[0] == "" {
		return false
	}
	var h = contentType[0]
	for _, p := range prefixes {
		if len(h) >= len(p) && h[:len(p)] == p {
			return true
		}
	}
	return false
}

func NewWriterMiddleware(opts *WriterMiddlewareOptions) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		var compressMinSize = opts.CompressMinSize
		if compressMinSize <= 0 {
			compressMinSize = 1 << 10
		}
		var compressLevel = resolveGzipLevel(opts.CompressLevel)
		var shouldCompress = shouldCompressFn
		if opts.ShouldCompress != nil {
			shouldCompress = opts.ShouldCompress
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var writer = &ResponseWriter{
				originalWriter:  w,
				compress:        opts.Compress && canCompress(r.Header),
				compressLevel:   compressLevel,
				compressMinSize: compressMinSize,
				shouldCompress:  shouldCompress,
				state:           statePending,
			}
			next.ServeHTTP(writer, r)
			writer.finalize()
		})
	}
}

type compressState uint8

const (
	statePending compressState = iota
	stateBuffering
	stateStreaming
	stateFinalized
)

type MiddlewarePanic struct {
	Err   error
	Stack []byte
}

type ResponseWriter struct {
	panic           *MiddlewarePanic
	err             error
	logAttrs        []slog.Attr
	buffer          []byte
	compressWriter  *gzip.Writer
	originalWriter  http.ResponseWriter
	status          int
	compressMinSize int
	compressLevel   int
	state           compressState
	closedHeader    bool
	compress        bool
	shouldCompress  shouldCompress
}

func (w *ResponseWriter) WriteHeader(statusCode int) {
	if w.state != statePending {
		return
	}
	w.status = statusCode
	if w.compress && w.shouldCompress(w) {
		w.state = stateBuffering
		return
	}
	w.state = stateStreaming
	w.writeHeader()
}

func (w *ResponseWriter) Write(b []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	switch w.state {
	case stateBuffering:
		if w.compressMinSize <= 0 || len(w.buffer)+len(b) >= w.compressMinSize {
			w.originalWriter.Header().Del("Content-Length")
			w.originalWriter.Header().Set("Content-Encoding", "gzip")
			w.originalWriter.Header().Set("Vary", "Accept-Encoding")
			w.state = stateStreaming
			w.writeHeader()
			var gz = gzipWriterPools.get(w.compressLevel)
			gz.Reset(w.originalWriter)
			w.compressWriter = gz
			if len(w.buffer) > 0 {
				w.compressWriter.Write(w.buffer)
				w.buffer = nil
			}
			return w.compressWriter.Write(b)
		}
		if w.buffer == nil {
			w.buffer = make([]byte, 0, w.compressMinSize)
		}
		w.buffer = append(w.buffer, b...)
		return len(b), nil
	case stateStreaming:
		if w.compressWriter != nil {
			return w.compressWriter.Write(b)
		}
		return w.originalWriter.Write(b)
	}
	return len(b), nil
}

func (w *ResponseWriter) writeHeader() {
	if w.closedHeader {
		return
	}
	w.closedHeader = true
	w.originalWriter.WriteHeader(w.status)
}

func (w *ResponseWriter) finalize() {
	if w.state == stateFinalized {
		return
	}
	switch w.state {
	case stateBuffering:
		w.writeHeader()
		if len(w.buffer) > 0 {
			w.originalWriter.Write(w.buffer)
		}
	case stateStreaming:
		if w.compressWriter != nil {
			w.compressWriter.Close()
			gzipWriterPools.put(w.compressLevel, w.compressWriter)
			w.compressWriter = nil
		}
	}
	w.state = stateFinalized
	w.buffer = nil
}

func (w *ResponseWriter) Status() int {
	return w.status
}

func (w *ResponseWriter) Written() bool {
	return w.closedHeader
}

func (w *ResponseWriter) Header() http.Header {
	return w.originalWriter.Header()
}

func (w *ResponseWriter) SetError(err error) {
	w.err = err
}

func (w *ResponseWriter) GetError() error {
	return w.err
}

func (w *ResponseWriter) SetPanic(err error, stack []byte) {
	w.panic = &MiddlewarePanic{
		Err:   err,
		Stack: stack,
	}
}

func (w *ResponseWriter) GetPanic() *MiddlewarePanic {
	return w.panic
}

func (w *ResponseWriter) AddLogAttrs(attrs ...slog.Attr) {
	w.logAttrs = append(w.logAttrs, attrs...)
}

func (w *ResponseWriter) GetLogAttrs() []slog.Attr {
	return w.logAttrs
}
