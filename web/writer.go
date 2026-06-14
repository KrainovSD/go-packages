package web

import (
	"compress/gzip"
	"encoding/json"
	"net/http"
	"strings"
)

type WriterMiddlewareOptions struct {
	GzipLevel int
	Gzip      bool
}

type WriterMiddleware struct {
	gzipLevel int
	gzip      bool
}

func WriterMiddlewareCreate(opts *WriterMiddlewareOptions) *WriterMiddleware {
	return &WriterMiddleware{
		gzipLevel: opts.GzipLevel,
		gzip:      opts.Gzip,
	}
}

func (m *WriterMiddleware) Register(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var writer = NewResponseWriter(w, strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && m.gzip, m.gzipLevel)
		next.ServeHTTP(writer, r)
		if gzipWriter, ok := writer.(*ResponseWriter); ok {
			if gzipWriter.compressWriter != nil {
				gzipWriter.compressWriter.Close()
			}
		}
	})
}

type ResponseWriter struct {
	compressWriter *gzip.Writer
	originalWriter http.ResponseWriter
	status         int
	err            string
	closedHeader   bool
	canCompress    bool
	gzipLevel      int
}

func NewResponseWriter(originalWriter http.ResponseWriter, canCompress bool, gzipLevel int) http.ResponseWriter {

	return &ResponseWriter{
		originalWriter: originalWriter,
		status:         0,
		canCompress:    canCompress,
		gzipLevel:      gzipLevel,
	}
}

func (g *ResponseWriter) Status() int {
	return g.status
}

func (g *ResponseWriter) Error() string {
	return g.err
}

func (g *ResponseWriter) Written() bool {
	return g.closedHeader
}

func (g *ResponseWriter) Write(b []byte) (int, error) {
	if !g.closedHeader {
		g.WriteHeader(http.StatusOK)
	}
	if g.status >= 400 {
		var response WebErrorResponse
		_ = json.Unmarshal(b, &response)
		g.err = response.Description
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
	if strings.HasPrefix(g.originalWriter.Header().Get("Content-Type"), "application/json") && g.canCompress {
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
	if g.gzipLevel != 0 {
		level = g.gzipLevel
	}
	// should close after write
	if gz, err = gzip.NewWriterLevel(g.originalWriter, level); err != nil {
		return
	}
	g.compressWriter = gz
}
