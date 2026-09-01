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
	tests = append(tests, Test{header: `form-data; filename*=UTF-8''graph%3B%20%281%29.json`, expectedFieldName: "", expectedFilename: "graph; (1).json", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="file"; filename="report.pdf"`, expectedFieldName: "file", expectedFilename: "report.pdf", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="avatar"; filename="photo with spaces.jpg"`, expectedFieldName: "avatar", expectedFilename: "photo with spaces.jpg", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="document"; filename="документ.txt"`, expectedFieldName: "document", expectedFilename: "документ.txt", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="doc"; filename="file.txt"; filename*=UTF-8''%D0%B4%D0%BE%D0%BA%D1%83%D0%BC%D0%B5%D0%BD%D1%82.txt`, expectedFieldName: "doc", expectedFilename: "документ.txt", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="img"; filename="quote\".jpg"`, expectedFieldName: "img", expectedFilename: `quote".jpg`, expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="audio"; filename="file with ; semicolon.mp3"`, expectedFieldName: "audio", expectedFilename: "file with ; semicolon.mp3", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="video"; filename*=UTF-8''%D1%84%D0%B0%D0%B9%D0%BB%20%D1%81%20%D0%BF%D1%80%D0%BE%D0%B1%D0%B5%D0%BB%D0%B0%D0%BC%D0%B8.mp4`, expectedFieldName: "video", expectedFilename: "файл с пробелами.mp4", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="photos"; filename="image.jpg"; filename*=UTF-8''%D0%B8%D0%B7%D0%BE%D0%B1%D1%80%D0%B0%D0%B6%D0%B5%D0%BD%D0%B8%D0%B5.jpg`, expectedFieldName: "photos", expectedFilename: "изображение.jpg", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="field"`, expectedFieldName: "field", expectedFilename: "", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="file"; filename="C:\temp\file.txt"`, expectedFieldName: "file", expectedFilename: `C:\temp\file.txt`, expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="file"; filename="foobar"`, expectedFieldName: "file", expectedFilename: "foobar", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="file"; filename="foo[1](2).html"`, expectedFieldName: "file", expectedFilename: "foo[1](2).html", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="file"; filename="foo.html"; size=1024`, expectedFieldName: "file", expectedFilename: "foo.html", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `attachment; filename="document.pdf"`, expectedFieldName: "", expectedFilename: "document.pdf", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `inline; filename="report.pdf"`, expectedFieldName: "", expectedFilename: "report.pdf", expectedTarget: "inline"})
	tests = append(tests, Test{header: `attachment; filename="my report.pdf"`, expectedFieldName: "", expectedFilename: "my report.pdf", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; filename="отчет.pdf"`, expectedFieldName: "", expectedFilename: "отчет.pdf", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; filename="report.pdf"; filename*=UTF-8''%D0%BE%D1%82%D1%87%D0%B5%D1%82.pdf`, expectedFieldName: "", expectedFilename: "отчет.pdf", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; filename*=UTF-8''%D1%84%D0%B0%D0%B9%D0%BB%20%D1%81%20%D0%BF%D1%80%D0%BE%D0%B1%D0%B5%D0%BB%D0%B0%D0%BC%D0%B8.txt`, expectedFieldName: "", expectedFilename: "файл с пробелами.txt", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; filename="file.txt"; filename*=UTF-8''%D1%84%D0%B0%D0%B9%D0%BB.txt`, expectedFieldName: "", expectedFilename: "файл.txt", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; filename="file with spaces.txt"`, expectedFieldName: "", expectedFilename: "file with spaces.txt", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; filename=file.txt`, expectedFieldName: "", expectedFilename: "file.txt", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; filename="file.txt"; modification-date="Fri, 01 Sep 2026 12:00:00 GMT"`, expectedFieldName: "", expectedFilename: "file.txt", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; filename="file\"with\"quotes.txt"`, expectedFieldName: "", expectedFilename: `file"with"quotes.txt`, expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; filename="file;with;semicolon.txt"`, expectedFieldName: "", expectedFilename: "file;with;semicolon.txt", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment`, expectedFieldName: "", expectedFilename: "", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `form-data; name="file"; filename=""; filename*=UTF-8''%D1%84%D0%B0%D0%B9%D0%BB.txt`, expectedFieldName: "file", expectedFilename: "файл.txt", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `form-data; name="file"; filename=""`, expectedFieldName: "file", expectedFilename: "", expectedTarget: "form-data"})
	tests = append(tests, Test{header: `attachment; filename=""; filename*=UTF-8''%D0%B4%D0%BE%D0%BA.pdf`, expectedFieldName: "", expectedFilename: "док.pdf", expectedTarget: "attachment"})
	tests = append(tests, Test{header: `attachment; filename="foo bar"; filename*=UTF-8''%D0%B1%D0%B0%D1%80`, expectedFieldName: "", expectedFilename: "бар", expectedTarget: "attachment"})

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
