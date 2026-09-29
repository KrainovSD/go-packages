package web

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

const gzipLevelOffset = 2 // -2..9 -> 0..11
const gzipLevelCount = 12

type gzipWriterPoolsType struct {
	pools *[gzipLevelCount]sync.Pool
}

var gzipWriterPools = newGzipPools()

func newGzipPools() gzipWriterPoolsType {
	var pools [gzipLevelCount]sync.Pool
	for i := range pools {
		var level = i - gzipLevelOffset
		pools[i].New = func() any {
			var gz *gzip.Writer
			var err error
			if gz, err = gzip.NewWriterLevel(io.Discard, level); err != nil {
				return gzip.NewWriter(io.Discard)
			}
			return gz
		}
	}
	return gzipWriterPoolsType{
		pools: &pools,
	}
}

func (p *gzipWriterPoolsType) getPool(level int) *sync.Pool {
	if level < gzip.HuffmanOnly || level > gzip.BestCompression {
		return &p.pools[gzip.DefaultCompression+gzipLevelOffset]
	}
	return &p.pools[level+gzipLevelOffset]
}

func (p *gzipWriterPoolsType) get(level int) *gzip.Writer {
	return p.getPool(level).Get().(*gzip.Writer)
}

func (p *gzipWriterPoolsType) put(level int, gzip *gzip.Writer) {
	if gzip == nil {
		return
	}
	gzip.Reset(io.Discard)
	p.getPool(level).Put(gzip)
}

func resolveGzipLevel(level int) int {
	if level == 0 || level < gzip.HuffmanOnly || level > gzip.BestCompression {
		return gzip.DefaultCompression
	}
	return level
}

func canCompress(header http.Header) bool {
	var encodings = header["Accept-Encoding"]
	for _, value := range encodings {
		if acceptGzip(value) {
			return true
		}
	}
	return false
}

func acceptGzip(value string) bool {
	for value != "" {
		var token = value
		if idx := strings.IndexByte(value, ','); idx >= 0 {
			token = value[:idx]
			value = value[idx+1:]
		} else {
			value = ""
		}
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		var params string
		if idx := strings.IndexByte(token, ';'); idx >= 0 {
			params = token[idx+1:]
			token = strings.TrimSpace(token[:idx])
		}
		if !strings.EqualFold(token, "gzip") && !strings.EqualFold(token, "x-gzip") {
			continue
		}
		if !qualityDisabled(params) {
			return true
		}
	}
	return false
}

func qualityDisabled(params string) bool {
	for params != "" {
		var param = params
		if idx := strings.IndexByte(params, ';'); idx >= 0 {
			param = params[:idx]
			params = params[idx+1:]
		} else {
			params = ""
		}
		param = strings.TrimSpace(param)
		var eq = strings.IndexByte(param, '=')
		if eq < 0 {
			continue
		}
		var name = strings.TrimSpace(param[:eq])
		if name != "q" && name != "Q" {
			continue
		}
		return isZeroQuality(strings.TrimSpace(param[eq+1:]))
	}
	return false
}

func isZeroQuality(value string) bool {
	var digits = 0
	for i := 0; i < len(value); i++ {
		var c = value[i]
		if c == '0' {
			digits++
			continue
		}
		if c == '.' {
			continue
		}
		return false
	}
	return digits > 0
}
