package web

import (
	"errors"
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

var defaultMaxQueries = 10000

func ParseQuery(query string) ([]Query, error) {
	var count = strings.Count(query, "&") + 1
	if count > defaultMaxQueries {
		return nil, errors.New("number of URL query parameters exceeded limit")
	}
	var queries = make([]Query, 0, count)
	var err error
	for query != "" {
		var key string
		key, query, _ = strings.Cut(query, "&")
		if strings.Contains(key, ";") {
			err = errors.New("invalid semicolon separator in query")
			continue
		}
		if key == "" {
			continue
		}
		key, value, _ := strings.Cut(key, "=")
		key, err1 := url.QueryUnescape(key)
		if err1 != nil {
			if err == nil {
				err = err1
			}
			continue
		}
		value, err1 = url.QueryUnescape(value)
		if err1 != nil {
			if err == nil {
				err = err1
			}
			continue
		}
		queries = append(queries, Query{
			Key:   key,
			Value: value,
		})
	}
	return queries, err
}
