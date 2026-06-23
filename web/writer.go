package web

import (
	"compress/gzip"
	"net/http"
	"strings"
)

type WriterMiddlewareOptions struct {
	CompressLevel int
	Compress      bool
	NeedCompress  func(header http.Header) bool
}

type WriterMiddleware struct {
	compressLevel int
	compress      bool
	needCompress  func(header http.Header) bool
}

func CreateWriterMiddleware(opts *WriterMiddlewareOptions) *WriterMiddleware {
	return &WriterMiddleware{
		compressLevel: opts.CompressLevel,
		compress:      opts.Compress,
		needCompress:  opts.NeedCompress,
	}
}

func (m *WriterMiddleware) Register(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var writer = NewResponseWriter(&ResponseWriterOptions{
			OriginalWriter: w,
			Compress:       m.compress && canCompress(w.Header()),
			CompressLevel:  m.compressLevel,
			NeedCompress:   m.needCompress,
		})
		next.ServeHTTP(writer, r)
		if writer, ok := writer.(*ResponseWriter); ok {
			if writer.compressWriter != nil {
				writer.compressWriter.Close()
			}
		}
	})
}

func canCompress(header http.Header) bool {
	var encoding = header["Accept-Encoding"]
	if len(encoding) == 0 {
		return false
	}
	return strings.Contains(encoding[0], "gzip")
}

type MiddlewarePanic struct {
	Err   error
	Stack []byte
}

type ResponseWriter struct {
	panic          *MiddlewarePanic
	err            error
	compressWriter *gzip.Writer
	originalWriter http.ResponseWriter
	status         int
	closedHeader   bool
	compress       bool
	compressLevel  int
	needCompress   func(header http.Header) bool
}

type ResponseWriterOptions struct {
	OriginalWriter http.ResponseWriter
	Compress       bool
	CompressLevel  int
	NeedCompress   func(header http.Header) bool
}

func NewResponseWriter(opts *ResponseWriterOptions) http.ResponseWriter {
	return &ResponseWriter{
		originalWriter: opts.OriginalWriter,
		status:         0,
		compress:       opts.Compress,
		compressLevel:  opts.CompressLevel,
		needCompress:   opts.NeedCompress,
	}
}

func (g *ResponseWriter) Status() int {
	return g.status
}

func (g *ResponseWriter) Written() bool {
	return g.closedHeader
}

func (g *ResponseWriter) Write(b []byte) (int, error) {
	if !g.closedHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.compressWriter != nil {
		return g.compressWriter.Write(b)
	}
	return g.originalWriter.Write(b)
}

func (g *ResponseWriter) WriteHeader(statusCode int) {
	if g.Written() {
		return
	}
	if g.compress && (g.needCompress == nil || g.needCompress(g.originalWriter.Header())) {
		g.SetGzipWriter()
		g.originalWriter.Header().Set("Content-Encoding", "gzip")
		g.originalWriter.Header().Set("Vary", "Accept-Encoding")
	}
	g.closedHeader = true
	g.originalWriter.WriteHeader(statusCode)
	g.status = statusCode
}

func (g *ResponseWriter) Header() http.Header {
	return g.originalWriter.Header()
}

func (g *ResponseWriter) SetGzipWriter() {
	var gz *gzip.Writer
	var err error
	var level = gzip.DefaultCompression
	if g.compressLevel != 0 {
		level = g.compressLevel
	}
	// should close after write
	if gz, err = gzip.NewWriterLevel(g.originalWriter, level); err != nil {
		return
	}
	g.compressWriter = gz
}

func (g *ResponseWriter) SetError(err error) {
	g.err = err
}

func (g *ResponseWriter) GetError() error {
	return g.err
}

func (g *ResponseWriter) SetPanic(err error, stack []byte) {
	g.panic = &MiddlewarePanic{
		Err:   err,
		Stack: stack,
	}
}

func (g *ResponseWriter) GetPanic() *MiddlewarePanic {
	return g.panic
}
