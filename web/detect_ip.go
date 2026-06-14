package web

import (
	"net"
	"net/http"
	"strings"
)

type ipHeader struct {
	name     string
	commaSep bool
	rfc7239  bool
}

var ipHeaders = []ipHeader{
	{name: "CF-Connecting-IP"},
	{name: "True-Client-IP"},
	{name: "Forwarded", rfc7239: true},
	{name: "X-Real-IP"},
	{name: "X-Forwarded-For", commaSep: true},
	{name: "X-Client-IP"},
	{name: "X-Forwarded", commaSep: true},
	{name: "X-Cluster-Client-IP"},
}

func DetectIP(req *http.Request) string {
	for _, h := range ipHeaders {
		raw := req.Header.Get(h.name)
		if raw == "" {
			continue
		}

		if h.rfc7239 {
			if ip := parseForwardedHeader(raw); ip != "" {
				return ip
			}
			continue
		}

		if h.commaSep {
			if ip := parseFirstIP(raw); ip != "" {
				return ip
			}
			continue
		}

		candidate := strings.TrimSpace(raw)
		if net.ParseIP(candidate) != nil {
			return candidate
		}
	}

	var host, _, parseErr = net.SplitHostPort(req.RemoteAddr)
	if parseErr != nil {
		host = req.RemoteAddr
	}
	return host
}

func parseFirstIP(value string) string {
	candidate, _, _ := strings.Cut(value, ",")
	candidate = strings.TrimSpace(candidate)
	if net.ParseIP(candidate) != nil {
		return candidate
	}
	return ""
}

func parseForwardedHeader(value string) string {
	firstElement, _, _ := strings.Cut(value, ",")

	lower := strings.ToLower(firstElement)
	forIdx := strings.Index(lower, "for=")
	if forIdx < 0 {
		return ""
	}

	ipPart := firstElement[forIdx+4:]

	if semiIdx := strings.Index(ipPart, ";"); semiIdx >= 0 {
		ipPart = ipPart[:semiIdx]
	}
	ipPart = strings.TrimSpace(ipPart)

	if len(ipPart) >= 2 && ipPart[0] == '"' && ipPart[len(ipPart)-1] == '"' {
		ipPart = ipPart[1 : len(ipPart)-1]
	}
	if len(ipPart) >= 2 && ipPart[0] == '[' && ipPart[len(ipPart)-1] == ']' {
		ipPart = ipPart[1 : len(ipPart)-1]
	}

	if net.ParseIP(ipPart) != nil {
		return ipPart
	}
	return ""
}
