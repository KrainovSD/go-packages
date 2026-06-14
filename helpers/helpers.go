package helpers

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func RandomHex(length int) (string, error) {
	bytes := make([]byte, (length+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

func RandomBase64(length int) (string, error) {
	bytes := make([]byte, (length+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}

func LoadEnvFile(path string) error {
	var file *os.File
	var err error
	var scanner *bufio.Scanner
	var line string
	var parts []string
	var key string
	var value string

	if file, err = os.Open(path); err != nil {
		return err
	}

	defer file.Close()

	scanner = bufio.NewScanner(file)
	for scanner.Scan() {
		line = scanner.Text()
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		parts = strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key = strings.TrimSpace(parts[0])
		value = strings.TrimSpace(parts[1])
		os.Setenv(key, value)
	}

	return scanner.Err()
}

func URLJoin(elem ...string) string {
	size := 0
	for _, e := range elem {
		size += len(e)
	}
	if size == 0 {
		return ""
	}
	buf := make([]byte, 0, size+len(elem)-1)
	for _, e := range elem {
		if len(buf) > 0 || e != "" {
			if len(buf) > 0 {
				buf = append(buf, '/')
			}
			buf = append(buf, strings.Trim(e, "/")...)
		}
	}
	return string(buf)
}

func Base64ToString(str string) (string, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", fmt.Errorf("decoding Base64 string: %w", err)
	}

	return string(decodedBytes), nil
}

func GetMaxThreads() int {
	return runtime.NumCPU()
}

func RandomFileName(name string) (string, error) {
	var hex string
	var err error
	if hex, err = RandomHex(16); err != nil {
		return hex, err
	}
	var ext string
	if dot := strings.LastIndex(name, "."); dot != -1 {
		ext = name[dot+1:]
		if len(ext) > 0 {
			ext = "." + ext
		}
	}

	return fmt.Sprintf("file_%d_%s%s", time.Now().Unix(), hex, ext), err
}

func ParseEnvSlice(env string) []string {
	var slice = strings.Split(env, ",")
	for i := range slice {
		slice[i] = strings.TrimSpace(slice[i])
	}
	if len(slice) == 1 && slice[0] == "" {
		slice = nil
	}
	return slice
}

func ParseEnvInt(env string) *int {
	var res int
	var err error
	if res, err = strconv.Atoi(env); err != nil {
		return nil
	}
	return &res
}

func TruncateString(str string, length int) string {
	var r = []rune(str)
	if len(r) <= length {
		return str
	}
	return string(r[:length])
}
