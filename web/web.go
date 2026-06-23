package web

import (
	"net"
	"net/http"
	"strings"
)

func GetProto(r *http.Request, custom string) string {
	var proto string
	var proxyHeader = r.Header[http.CanonicalHeaderKey("x-forwarded-proto")]
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

func GetClientIP(req *http.Request) string {
	var xff = req.Header.Get("X-Forwarded-For")
	if xff != "" {
		var parts = strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	var host string
	var port string
	if host, port, _ = net.SplitHostPort(req.RemoteAddr); port == "" {
		return host
	}
	return host
}

func CheckIsSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	var proto string
	if proto = r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return strings.Split(proto, ",")[0] == "https"
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
