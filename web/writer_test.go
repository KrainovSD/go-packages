package web

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestWriterMiddlewareCompressRoundtrip(t *testing.T) {
	var body = compressibleBody(9 << 10)
	var recorder = runWriter(&WriterMiddlewareOptions{Compress: true}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}, "gzip")
	if recorder.Code != http.StatusOK {
		t.Errorf("status: %d, expected 200", recorder.Code)
	}
	if encoding := recorder.Header().Get("Content-Encoding"); encoding != "gzip" {
		t.Errorf("Content-Encoding: %q, expected gzip", encoding)
	}
	if vary := recorder.Header().Get("Vary"); vary != "Accept-Encoding" {
		t.Errorf("Vary: %q, expected Accept-Encoding", vary)
	}
	if recorder.Header().Get("Content-Length") != "" {
		t.Errorf("Content-Length must be removed for compressed responses")
	}
	var data = gunzipBody(t, recorder)
	if !bytes.Equal(data, body) {
		t.Errorf("gunzipped body does not match: got %d bytes, expected %d", len(data), len(body))
	}
}

func TestWriterMiddlewareNoAcceptEncoding(t *testing.T) {
	var body = compressibleBody(9 << 10)
	var recorder = runWriter(&WriterMiddlewareOptions{Compress: true}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}, "")
	if recorder.Header().Get("Content-Encoding") != "" {
		t.Errorf("Content-Encoding must not be set without Accept-Encoding")
	}
	if !bytes.Equal(recorder.Body.Bytes(), body) {
		t.Errorf("body must be sent uncompressed byte-for-byte")
	}
}

func TestWriterMiddlewareDisabledGzipNotCompressed(t *testing.T) {
	var body = compressibleBody(9 << 10)
	var recorder = runWriter(&WriterMiddlewareOptions{Compress: true}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}, "gzip;q=0")
	if recorder.Header().Get("Content-Encoding") != "" {
		t.Errorf("Content-Encoding must not be set for gzip;q=0")
	}
	if !bytes.Equal(recorder.Body.Bytes(), body) {
		t.Errorf("body must be sent uncompressed byte-for-byte")
	}
}

func TestWriterMiddlewareNonCompressibleContentType(t *testing.T) {
	var body = compressibleBody(9 << 10)
	var recorder = runWriter(&WriterMiddlewareOptions{Compress: true}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(body)
	}, "gzip")
	if recorder.Header().Get("Content-Encoding") != "" {
		t.Errorf("Content-Encoding must not be set for image/png")
	}
	if !bytes.Equal(recorder.Body.Bytes(), body) {
		t.Errorf("body must be sent uncompressed byte-for-byte")
	}
}

func TestWriterMiddlewareInvalidLevelFallback(t *testing.T) {
	var levels = []int{42, -7}
	for _, level := range levels {
		var body = compressibleBody(9 << 10)
		var recorder = runWriter(&WriterMiddlewareOptions{Compress: true, CompressLevel: level}, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		}, "gzip")
		if encoding := recorder.Header().Get("Content-Encoding"); encoding != "gzip" {
			t.Errorf("level %d: Content-Encoding: %q, expected gzip", level, encoding)
		}
		var data = gunzipBody(t, recorder)
		if !bytes.Equal(data, body) {
			t.Errorf("level %d: gunzipped body does not match (raw body labeled as gzip)", level)
		}
	}
}

func TestWriterMiddlewarePoolReuseNoLeak(t *testing.T) {
	var opts = &WriterMiddlewareOptions{Compress: true, CompressLevel: gzip.BestSpeed}
	var middleware = NewWriterMiddleware(opts)
	var run = func(handler http.HandlerFunc, acceptEncoding string) *httptest.ResponseRecorder {
		var request = httptest.NewRequest(http.MethodGet, "/test", nil)
		if acceptEncoding != "" {
			request.Header.Set("Accept-Encoding", acceptEncoding)
		}
		var recorder = httptest.NewRecorder()
		middleware(handler).ServeHTTP(recorder, request)
		return recorder
	}
	var bodyA = compressibleBody(9 << 10)
	var bodyB = bytes.Repeat([]byte("<html>other content</html>"), 128)
	var bodyC = []byte(`{"ok":true}`)
	var recorderA = run(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(bodyA)
	}, "gzip")
	var recorderB = run(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(bodyB)
	}, "gzip")
	var recorderC = run(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(bodyC)
	}, "")
	var dataA = gunzipBody(t, recorderA)
	if !bytes.Equal(dataA, bodyA) {
		t.Errorf("response A corrupted: got %d bytes, expected %d", len(dataA), len(bodyA))
	}
	var dataB = gunzipBody(t, recorderB)
	if !bytes.Equal(dataB, bodyB) {
		t.Errorf("response B corrupted: got %d bytes, expected %d", len(dataB), len(bodyB))
	}
	if !bytes.Equal(recorderC.Body.Bytes(), bodyC) {
		t.Errorf("response C corrupted: %q, expected %q", recorderC.Body.String(), string(bodyC))
	}
}

func TestResponseWriterStatusWritten(t *testing.T) {
	var opts = &WriterMiddlewareOptions{Compress: true, CompressMinSize: 1 << 10}
	var middleware = NewWriterMiddleware(opts)
	var response *ResponseWriter
	var handler = middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if writer, ok := w.(*ResponseWriter); ok {
			response = writer
		}
		if response.Status() != http.StatusCreated {
			t.Errorf("Status() during buffering: %d, expected 201", response.Status())
		}
		if response.Written() {
			t.Errorf("Written() must be false while the response is buffered")
		}
		w.Write(compressibleBody(2 << 10))
	}))
	var request = httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	var recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if !response.Written() {
		t.Errorf("Written() must be true after finalize")
	}
	if response.Status() != http.StatusCreated {
		t.Errorf("Status() after finalize: %d, expected 201", response.Status())
	}
	if recorder.Code != http.StatusCreated {
		t.Errorf("recorder code: %d, expected 201", recorder.Code)
	}
}

func TestWriterMiddlewareMinSizeSmallBodyUncompressed(t *testing.T) {
	var body = []byte(`{"ok":true}`)
	var recorder = runWriter(&WriterMiddlewareOptions{Compress: true, CompressMinSize: 1 << 10}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusCreated)
		w.Write(body)
	}, "gzip")
	if recorder.Code != http.StatusCreated {
		t.Errorf("status: %d, expected 201", recorder.Code)
	}
	if encoding := recorder.Header().Get("Content-Encoding"); encoding != "" {
		t.Errorf("Content-Encoding: %q, expected empty", encoding)
	}
	if length := recorder.Header().Get("Content-Length"); length != strconv.Itoa(len(body)) {
		t.Errorf("Content-Length: %q, expected %q (handler value must survive)", length, strconv.Itoa(len(body)))
	}
	if !bytes.Equal(recorder.Body.Bytes(), body) {
		t.Errorf("body: %q, expected %q", recorder.Body.String(), string(body))
	}
}

func TestWriterMiddlewareMinSizeLargeBodyCompressed(t *testing.T) {
	var body = compressibleBody(9 << 10)
	var recorder = runWriter(&WriterMiddlewareOptions{Compress: true, CompressMinSize: 1 << 10}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}, "gzip")
	if encoding := recorder.Header().Get("Content-Encoding"); encoding != "gzip" {
		t.Errorf("Content-Encoding: %q, expected gzip", encoding)
	}
	if recorder.Header().Get("Content-Length") != "" {
		t.Errorf("Content-Length must be removed for compressed responses")
	}
	var data = gunzipBody(t, recorder)
	if !bytes.Equal(data, body) {
		t.Errorf("gunzipped body does not match: got %d bytes, expected %d", len(data), len(body))
	}
}

func TestWriterMiddlewareMinSizeStatusPreserved(t *testing.T) {
	var body = compressibleBody(9 << 10)
	var recorder = runWriter(&WriterMiddlewareOptions{Compress: true, CompressMinSize: 1 << 10}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write(body)
	}, "gzip")
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status: %d, expected 404", recorder.Code)
	}
	var data = gunzipBody(t, recorder)
	if !bytes.Equal(data, body) {
		t.Errorf("gunzipped body does not match")
	}
}

func TestWriterMiddlewareMinSizeEmptyBodyNoGzip(t *testing.T) {
	var statuses = []int{http.StatusNoContent, http.StatusNotModified, http.StatusOK}
	for _, status := range statuses {
		var recorder = runWriter(&WriterMiddlewareOptions{Compress: true, CompressMinSize: 1 << 10}, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
		}, "gzip")
		if recorder.Code != status {
			t.Errorf("status %d: code %d", status, recorder.Code)
		}
		if encoding := recorder.Header().Get("Content-Encoding"); encoding != "" {
			t.Errorf("status %d: Content-Encoding: %q, expected empty", status, encoding)
		}
		if recorder.Body.Len() != 0 {
			t.Errorf("status %d: body must be empty, got %d bytes", status, recorder.Body.Len())
		}
	}
}

func TestWriterMiddlewareMinSizeNegativeTreatedAsDefault(t *testing.T) {
	var body = []byte(`{"ok":true}`)
	var recorder = runWriter(&WriterMiddlewareOptions{Compress: true, CompressMinSize: -100}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}, "gzip")
	if encoding := recorder.Header().Get("Content-Encoding"); encoding != "" {
		t.Errorf("Content-Encoding must be empty")
	}
	if !bytes.Equal(recorder.Body.Bytes(), body) {
		t.Errorf("body does not match")
	}
}

func TestWriterMiddlewareMinSizePanicRecovered(t *testing.T) {
	var goalkeeper = NewGoalkeeperMiddleware()
	var opts = &WriterMiddlewareOptions{Compress: true, CompressMinSize: 1 << 10}
	var middleware = NewWriterMiddleware(opts)
	var request = httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	var body = bytes.Repeat([]byte("a"), 100)
	var wrapped = middleware(goalkeeper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
		panic("boom")
	})))
	var rec = httptest.NewRecorder()
	wrapped.ServeHTTP(rec, request)
	if rec.Header().Get("Content-Encoding") != "" {
		t.Errorf("Content-Encoding must be empty")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("first WriteHeader must win: %d, expected 200", rec.Code)
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), body) {
		t.Errorf("partial body lost: %q", rec.Body.String())
	}
	var wrappedNoWrite = middleware(goalkeeper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		panic("boom")
	})))
	var recNoWrite = httptest.NewRecorder()
	wrappedNoWrite.ServeHTTP(recNoWrite, request)
	if recNoWrite.Code != http.StatusInternalServerError {
		t.Errorf("code: %d, expected 500", recNoWrite.Code)
	}
	if recNoWrite.Header().Get("Content-Encoding") != "" {
		t.Errorf("Content-Encoding must be empty for a below-threshold response")
	}
}

func runWriter(opts *WriterMiddlewareOptions, handler http.HandlerFunc, acceptEncoding string) *httptest.ResponseRecorder {
	var middleware = NewWriterMiddleware(opts)
	var wrapped = middleware(handler)
	var request = httptest.NewRequest(http.MethodGet, "/test", nil)
	if acceptEncoding != "" {
		request.Header.Set("Accept-Encoding", acceptEncoding)
	}
	var recorder = httptest.NewRecorder()
	wrapped.ServeHTTP(recorder, request)
	return recorder
}

func gunzipBody(t *testing.T, recorder *httptest.ResponseRecorder) []byte {
	t.Helper()
	var reader, err = gzip.NewReader(recorder.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	var data []byte
	if data, err = io.ReadAll(reader); err != nil {
		t.Fatalf("read gunzipped body: %v", err)
	}
	if err = reader.Close(); err != nil {
		t.Fatalf("close gzip reader: %v", err)
	}
	return data
}

func compressibleBody(size int) []byte {
	var template = []byte(`{"key":"value","nested":{"a":1,"b":"text data"}}`)
	var count = size / len(template)
	return bytes.Repeat(template, count)
}
