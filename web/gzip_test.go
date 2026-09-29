package web

import (
	"compress/gzip"
	"net/http"
	"strconv"
	"testing"
)

type CanCompressTest struct {
	name      string
	encodings []string
	expected  bool
}

func TestCanCompress(t *testing.T) {
	var tests = []CanCompressTest{
		{name: "missing header", encodings: nil, expected: false},
		{name: "empty value", encodings: []string{""}, expected: false},
		{name: "gzip", encodings: []string{"gzip"}, expected: true},
		{name: "gzip uppercase", encodings: []string{"GZIP"}, expected: true},
		{name: "x-gzip", encodings: []string{"x-gzip"}, expected: true},
		{name: "with quality", encodings: []string{"gzip;q=0.8"}, expected: true},
		{name: "spaced quality", encodings: []string{"gzip; q = 0.8"}, expected: true},
		{name: "disabled", encodings: []string{"gzip;q=0"}, expected: false},
		{name: "disabled float", encodings: []string{"gzip;q=0.000"}, expected: false},
		{name: "disabled uppercase q", encodings: []string{"gzip;Q=0"}, expected: false},
		{name: "other codings", encodings: []string{"deflate, br"}, expected: false},
		{name: "other then gzip", encodings: []string{"deflate, br, gzip"}, expected: true},
		{name: "gzip then disabled", encodings: []string{"gzip;q=0, br"}, expected: false},
		{name: "invalid quality means enabled", encodings: []string{"gzip;q=abc"}, expected: true},
		{name: "wildcard not honored", encodings: []string{"*"}, expected: false},
		{name: "multiple header values", encodings: []string{"br", "gzip"}, expected: true},
		{name: "spaced disabled", encodings: []string{"gzip; q = 0"}, expected: false},
		{name: "tiny nonzero quality", encodings: []string{"gzip;q=0.001"}, expected: true},
		{name: "mixed case coding", encodings: []string{"GZip;q=0.5"}, expected: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var header = http.Header{}
			if test.encodings != nil {
				header["Accept-Encoding"] = test.encodings
			}
			var result = canCompress(header)
			if result != test.expected {
				t.Errorf("returned %v for %v, expected %v", result, test.encodings, test.expected)
			}
		})
	}
}

func TestCanCompressAllocs(t *testing.T) {
	var inputs = []string{
		"gzip",
		"deflate, gzip;q=0.5, br",
		"gzip;q=0",
		"GZip; q = 0.8, x-gzip, br",
		"deflate, br",
	}
	for _, input := range inputs {
		var header = http.Header{}
		header.Set("Accept-Encoding", input)
		var avg = testing.AllocsPerRun(100, func() { canCompress(header) })
		if avg > 0 {
			t.Errorf("canCompress allocates %.1f objects per call for %q, expected 0", avg, input)
		}
	}
}

type ResolveLevelTest struct {
	level    int
	expected int
}

func TestResolveLevel(t *testing.T) {
	var tests = []ResolveLevelTest{
		{level: 0, expected: gzip.DefaultCompression},
		{level: 42, expected: gzip.DefaultCompression},
		{level: -7, expected: gzip.DefaultCompression},
		{level: gzip.BestSpeed, expected: gzip.BestSpeed},
		{level: gzip.BestCompression, expected: gzip.BestCompression},
		{level: gzip.DefaultCompression, expected: gzip.DefaultCompression},
		{level: gzip.HuffmanOnly, expected: gzip.HuffmanOnly},
	}
	for i, test := range tests {
		t.Run(strconv.Itoa(test.level), func(t *testing.T) {
			var result = resolveGzipLevel(test.level)
			if result != test.expected {
				t.Errorf("case %d: returned %d, expected %d", i, result, test.expected)
			}
		})
	}
}
