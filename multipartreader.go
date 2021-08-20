// Package multipartreader helps you encode large files in MIME multipart format
// without reading the entire content into memory.

// p.s - multipartreader + multipartstreamer

package multipartreader

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
)

// MultipartReader implements io.Reader, can be used to encode large files
type MultipartReader struct {
	contentType string
	boundary    string

	writer      *multipart.Writer
	readers     []io.Reader
	length      int64
	multiReader io.Reader
	count       int64
}

// New creates new MultipartReader
func New() (mr *MultipartReader) {
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)

	formBody := buf.String()
	formClose := "\r\n--" + writer.Boundary() + "--\r\n"

	bodyReader := strings.NewReader(formBody)
	closeReader := strings.NewReader(formClose)

	mr = &MultipartReader{
		writer:      writer,
		contentType: writer.FormDataContentType(),
		boundary:    writer.Boundary(),
		readers:     []io.Reader{bodyReader, closeReader},
		length:      int64(len(formBody) + len(formClose)),
	}
	return
}

// SetBoundary method is multipart.Writer.SetBoundary copy
func (w *MultipartReader) SetBoundary(boundary string) (err error) {
	err = w.writer.SetBoundary(boundary)
	return
}

// AddReader adds new reader to MultipartReader
func (mr *MultipartReader) AddReader(r io.Reader, length int64) {
	i := len(mr.readers)
	mr.readers = append(mr.readers[:i-1], r, mr.readers[i-1])
	mr.length = mr.length + length
}

// AddFormReader adds new reader as form part to MultipartReader
func (mr *MultipartReader) AddFormReader(r io.Reader, name, filename string, length int64) {
	form := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n\r\n", mr.boundary, name, filename)
	mr.AddReader(strings.NewReader(form), int64(len(form)))
	mr.AddReader(r, length)
	return
}

// WriteFields writes multiple form fields to the multipart.Writer.
func (m *MultipartReader) AddFields(fields map[string]string) error {
	var err error

	for key, value := range fields {
		err = m.writer.WriteField(key, value)
		if err != nil {
			return err
		}
	}

	return nil
}

// AddFile adds new file to MultipartReader
func (mr *MultipartReader) AddFile(file *os.File) (err error) {
	fs, err := file.Stat()
	if err != nil {
		return
	}

	form := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n\r\n", mr.boundary, "file", fs.Name())
	mr.AddReader(strings.NewReader(form), int64(len(form)))
	mr.AddReader(file, fs.Size())
	return
}

// SetupHTTPRequest set multiReader and headers after adding readers
func (mr *MultipartReader) SetupRequest(req *http.Request) {
	req.Body = mr.GetCloseReader()
	req.Header.Add("Content-Type", mr.contentType)
	req.ContentLength = mr.length
}

// Read implements the Read method
func (mpr *MultipartReader) Read(p []byte) (n int, err error) {
	mr := mpr.GetMultiReader()
	n, err = mr.Read(p)
	atomic.AddInt64(&mpr.count, int64(n))
	return n, err
}

// Count returns length of read data
func (mr *MultipartReader) Count() int64 {
	return atomic.LoadInt64(&mr.count)
}

// ContentType returns contentType
func (mr *MultipartReader) ContentType() string {
	return mr.contentType
}

func (mr *MultipartReader) GetMultiReader() io.Reader {
	if mr.multiReader == nil {
		mr.multiReader = io.MultiReader(mr.readers...)
	}
	return mr.multiReader
}

func (mr *MultipartReader) GetCloseReader() io.ReadCloser {
	reader := mr.GetMultiReader()
	return ioutil.NopCloser(reader)
}
