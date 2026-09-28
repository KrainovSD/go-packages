package web

import (
	"path/filepath"
	"strings"
)

type StaticClassifierOptions struct {
	Enabled           bool
	DynamicExtensions []string
	Custom            func(path string) bool
}

func NewStaticClassifier(opts *StaticClassifierOptions) func(path string) bool {
	if opts == nil || !opts.Enabled {
		return func(_ string) bool {
			return false
		}
	}
	if opts.Custom != nil {
		return opts.Custom
	}
	var dynamic = make(map[string]struct{}, len(opts.DynamicExtensions))
	for _, ext := range opts.DynamicExtensions {
		dynamic[staticExtensionKey(ext)] = struct{}{}
	}
	return func(path string) bool {
		var ext = filepath.Ext(path)
		if ext == "" {
			return false
		}
		if _, ok := dynamic[staticExtensionKey(ext)]; ok {
			return false
		}
		return true
	}
}

func staticExtensionKey(ext string) string {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return strings.ToLower(ext)
}
