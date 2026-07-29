package web

import (
	"net/http"
	"net/url"
	"strings"
)

func GetProto(r *http.Request, custom string) string {
	var proto string
	var proxyHeader = r.Header["X-Forwarded-Proto"]
	var scheme = r.URL.Scheme

	switch {
	case custom != "":
		proto = custom
	case len(proxyHeader) > 0:
		proto = proxyHeader[0]
	case scheme != "":
		proto = scheme
	case r.TLS != nil:
		proto = "https"
	default:
		proto = "http"
	}

	return proto
}

func GetHost(r *http.Request, custom string) string {
	var host string

	switch {
	case custom != "":
		host = custom
	default:
		host = r.Host
	}

	return host
}

func GetLastPath(path string) string {
	var lastSlash = strings.LastIndex(path, "/")
	if lastSlash != -1 && path != "/" {
		path = strings.Replace(path[lastSlash:], "/", "", 1)
	}
	if path == "/" {
		path = ""
	}
	return path
}

func CheckIsSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	var protoRaw = r.Header["X-Forwarded-Proto"]
	if len(protoRaw) != 0 {
		return strings.Split(protoRaw[0], ",")[0] == "https"
	}
	return r.URL.Scheme == "https"
}

func GetUserAgent(r *http.Request) string {
	// header use canonical key for all keys
	var ua = r.Header["User-Agent"]
	if len(ua) == 0 {
		return ""
	}
	return ua[0]
}

type Response struct {
	Message string `json:"message,omitempty"`
	Code    int    `json:"code,omitempty"`
	Status  int    `json:"status,omitempty"`
}

type Query struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func ParseRawQuery(rawQuery string) []Query {
	if rawQuery == "" {
		return nil
	}
	var parts = strings.Split(rawQuery, "&")
	var result = make([]Query, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		var key, value string
		if idx := strings.IndexByte(part, '='); idx >= 0 {
			key = part[:idx]
			value = part[idx+1:]
		} else {
			key = part
		}
		if decodedKey, err := url.QueryUnescape(key); err == nil {
			key = decodedKey
		}
		if decodedValue, err := url.QueryUnescape(value); err == nil {
			value = decodedValue
		}
		result = append(result, Query{Key: key, Value: value})
	}
	return result
}
