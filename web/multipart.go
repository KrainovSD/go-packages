package web

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
)

func ExtractFile(w http.ResponseWriter, req *http.Request, limit int64) ([]byte, string, error) {
	var file []byte
	var err error
	var contentType = req.Header.Get("content-type")
	var disposition = req.Header.Get("content-disposition")
	var target string
	var filename string
	var fieldname string
	var isMultipart = strings.Contains(contentType, "multipart/form-data")

	if target, filename, fieldname, err = ParseDisposition(disposition); err != nil {
		if !isMultipart {
			return file, filename, fmt.Errorf("parse disposition: %w", err)
		}
		target = "form-data"
		fieldname = "file"
	} else if isMultipart && target == "form-data" && fieldname == "" {
		fieldname = "file"
	}

	req.Body = http.MaxBytesReader(w, req.Body, limit)
	defer req.Body.Close()

	if target == "form-data" {
		if fieldname == "" {
			return file, filename, fmt.Errorf("bad disposition")
		}
		var formFile io.ReadCloser
		var formHeader *multipart.FileHeader
		formFile, formHeader, err = req.FormFile(fieldname)
		if err != nil {
			_, ok := err.(*http.MaxBytesError)
			if ok {
				return file, filename, ErrorRequestTooLarge

			}
			return file, filename, err
		}
		defer formFile.Close()
		if formHeader.Filename != "" {
			filename = formHeader.Filename
		}
		if file, err = io.ReadAll(formFile); err != nil {
			return file, filename, err
		}
		return file, filename, nil
	} else {
		file, err := io.ReadAll(req.Body)
		if err != nil {
			_, ok := err.(*http.MaxBytesError)
			if ok {
				return file, filename, ErrorRequestTooLarge

			}
			return file, filename, err
		}
		return file, filename, nil
	}
}

func ParseDisposition(disposition string) (string, string, string, error) {
	var target string
	var params map[string]string
	var err error
	if target, params, err = mime.ParseMediaType(disposition); err != nil {
		return "", "", "", fmt.Errorf("bad content disposition: %w", err)
	}
	return target, params["filename"], params["name"], nil
}
