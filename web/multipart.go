package web

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
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

	if target, filename, fieldname, err = ParseDisposition(disposition); err != nil {
		if strings.Contains(contentType, "multipart/form-data") {
			target = "form-data"
			fieldname = "file"
		} else {
			return file, filename, fmt.Errorf("parse disposition: %w", err)
		}
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

var FILE_NAME []rune = []rune(`filename="`)
var FILE_ENCODED_NAME []rune = []rune(`filename*=UTF-8''`)
var FIELD_NAME []rune = []rune(`name="`)

func ParseDisposition(disposition string) (string, string, string, error) {
	var target string
	var filename strings.Builder
	var fieldname strings.Builder

	before, after, found := strings.Cut(disposition, ";")
	if !found {
		return target, filename.String(), fieldname.String(), fmt.Errorf("bad content disposition")
	}
	target = strings.TrimSpace(before)

	var fileNameRune = 0
	var fileEncodedNameRune = 0
	var fieldNameRune = 0
	var cursor = 0
	var matching = false
	var afterRunes = []rune(after)
	var encoded = false

	for cursor < len(afterRunes) {
		var letter = afterRunes[cursor]
		if letter == ';' {
			goto CLEAN
		}
		// fileName
		if !matching && fileNameRune == 0 && letter == FILE_NAME[0] {
			cursor++
			matching = true
			fileNameRune++
			continue
		}
		if fileNameRune > 0 {
			if fileNameRune >= len(FILE_NAME) {
				if fieldNameRune == len(FILE_NAME) {
					fieldNameRune++
					filename = strings.Builder{}
				}

				if letter == '"' {
					fileNameRune = 0
					cursor++
					encoded = false
					continue
				}

				filename.WriteRune(letter)
				cursor++
				continue
			}
			if letter == FILE_NAME[fileNameRune] {
				cursor++
				fileNameRune++
				continue
			} else {
				cursor -= fileNameRune
				matching = false
				fileNameRune = -1
				continue
			}
		}
		// fileDecodedName
		if !matching && fileEncodedNameRune == 0 && letter == FILE_ENCODED_NAME[0] {
			cursor++
			matching = true
			fileEncodedNameRune++
			continue
		}
		if fileEncodedNameRune > 0 {
			if fileEncodedNameRune >= len(FILE_ENCODED_NAME) {
				if fileEncodedNameRune == len(FILE_ENCODED_NAME) {
					fileEncodedNameRune++
					filename = strings.Builder{}
				}
				if letter == ';' {
					fileEncodedNameRune = 0
					cursor++
					encoded = true
					continue
				}

				filename.WriteRune(letter)
				cursor++
				continue
			}
			if letter == FILE_ENCODED_NAME[fileEncodedNameRune] {
				cursor++
				fileEncodedNameRune++
				continue
			} else {
				cursor -= fileEncodedNameRune
				matching = false
				fileEncodedNameRune = -1
				continue
			}
		}
		// fieldName
		if !matching && fieldNameRune == 0 && letter == FIELD_NAME[0] {
			cursor++
			matching = true
			fieldNameRune++
			continue
		}
		if fieldNameRune > 0 {
			if fieldNameRune >= len(FIELD_NAME) {
				if fieldNameRune == len(FIELD_NAME) {
					fieldNameRune++
					fieldname = strings.Builder{}
				}
				if letter == '"' {
					fieldNameRune = 0
					cursor++
					continue
				}

				fieldname.WriteRune(letter)
				cursor++
				continue
			}
			if letter == FIELD_NAME[fieldNameRune] {
				cursor++
				fieldNameRune++
				continue
			} else {
				cursor -= fieldNameRune
				matching = false
				fieldNameRune = -1
				continue
			}
		}

		if !matching && letter == ' ' {
			cursor++
			continue
		}

		matching = true
		cursor++
		continue

	CLEAN:
		fileNameRune = 0
		fileEncodedNameRune = 0
		fieldNameRune = 0
		matching = false
		cursor++
	}

	if fieldNameRune > 0 {
		encoded = false
	} else if fileEncodedNameRune > 0 {
		encoded = true
	}

	if encoded {
		var err error
		var name string
		if name, err = url.PathUnescape(filename.String()); err != nil {
			return target, name, fieldname.String(), fmt.Errorf("error decode filename: %w", err)

		}
		return target, name, fieldname.String(), nil
	}

	return target, filename.String(), fieldname.String(), nil
}
