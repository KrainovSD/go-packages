package web

import (
	"strconv"
	"testing"
)

type Test struct {
	name                                                string
	header                                              string
	expectedFieldName, expectedFilename, expectedTarget string
}

func TestParseDisposition(t *testing.T) {
	var tests []Test
	tests = append(tests, Test{header: `attachment; filename="test.png"`, expectedFieldName: "", expectedFilename: "test.png", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; filename="test.png";`, expectedFieldName: "", expectedFilename: "test.png", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; filename="test.png"; filename*=UTF-8''test.png`, expectedFieldName: "", expectedFilename: "test.png", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; name="test"; filename="test.png"`, expectedFieldName: "test", expectedFilename: "test.png", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; name="test"; filename*=UTF-8''test.png`, expectedFieldName: "test", expectedFilename: "test.png", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; filename*=UTF-8''%D0%9B%D0%B5%D1%87%D0%B5%D0%BD%D0%B8%D0%B5.jpeg`, expectedFieldName: "", expectedFilename: "Лечение.jpeg", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `form-data; filename*=UTF-8''graph%3B%20(1).json`, expectedFieldName: "", expectedFilename: "graph; (1).json", expectedTarget: "form-data"})

	for i, test := range tests {
		var name string
		if test.name != "" {
			name = test.name
		} else {
			name = strconv.Itoa(i)
		}
		t.Run(name, func(t *testing.T) {
			target, filename, fieldname, err := ParseDisposition(test.header)
			if err != nil {
				t.Errorf("returned error: %v", err)
			}
			if target != test.expectedTarget {
				t.Errorf("returned target: %s, expected target: %s", target, test.expectedTarget)
			}
			if filename != test.expectedFilename {
				t.Errorf("returned filename: %s, expected filename: %s", filename, test.expectedFilename)
			}
			if fieldname != test.expectedFieldName {
				t.Errorf("returned fieldname: %s, expected fieldname: %s", fieldname, test.expectedFieldName)
			}
		})
	}

}
