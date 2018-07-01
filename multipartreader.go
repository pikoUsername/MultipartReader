// Package multipartreader helps you encode large files in MIME multipart format
// without reading the entire content into memory.

/*
魔改multipartreader
https://github.com/iikira/BaiduPCS-Go/blob/master/requester/multipartreader/multipartreader.go
iikira
Apache License 2.0
*/

package multipartreader

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
)

//MultipartReader implements io.Reader, can be used to encode large files
type MultipartReader struct {
	contentType string
	Boundary    string

	readers     []io.Reader
	length      int64
	multiReader io.Reader
	count       int64
}

//NewMultipartReader creates new MultipartReader
func NewMultipartReader() (mr *MultipartReader) {
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)

	formBody := buf.String()
	formClose := "\r\n--" + writer.Boundary() + "--\r\n"

	bodyReader := strings.NewReader(formBody)
	closeReader := strings.NewReader(formClose)

	mr = &MultipartReader{
		contentType: writer.FormDataContentType(),
		Boundary:    writer.Boundary(),
		readers:     []io.Reader{bodyReader, closeReader},
		length:      int64(len(formBody) + len(formClose)),
	}
	return
}

//AddReader adds new reader to MultipartReader
func (mr *MultipartReader) AddReader(r io.Reader, length int64) {
	i := len(mr.readers)
	mr.readers = append(mr.readers[:i-1], r, mr.readers[i-1])
	mr.length = mr.length + length
}

//AddFormReader adds new reader as form part to MultipartReader
func (mr *MultipartReader) AddFormReader(r io.Reader, name, filename string, length int64) {
	form := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n\r\n", mr.Boundary, name, filename)
	mr.AddReader(strings.NewReader(form), int64(len(form)))
	mr.AddReader(r, length)
	return
}

//AddFile adds new file to MultipartReader
func (mr *MultipartReader) AddFile(file *os.File) (err error) {
	fs, err := file.Stat()
	if err != nil {
		return
	}

	form := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n\r\n", mr.Boundary, "file", fs.Name())
	mr.AddReader(strings.NewReader(form), int64(len(form)))
	mr.AddReader(file, fs.Size())
	return
}

//SetupHTTPRequest set multiReader and headers after adding readers
func (mr *MultipartReader) SetupHTTPRequest(req *http.Request) {
	mr.multiReader = io.MultiReader(mr.readers...)

	req.Header.Add("Content-Type", mr.contentType)
	req.ContentLength = mr.length
}

//Read implements the Read method
func (mr *MultipartReader) Read(p []byte) (n int, err error) {
	n, err = mr.multiReader.Read(p)
	atomic.AddInt64(&mr.count, int64(n))
	return n, err
}

//Count returns length of read data
func (mr *MultipartReader) Count() int64 {
	return atomic.LoadInt64(&mr.count)
}
