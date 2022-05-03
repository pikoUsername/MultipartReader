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
	multiReader io.Reader
	count       int64
}

// New creates new MultipartReader
func New() (mr *MultipartReader) {
	mr = &MultipartReader{}

	bodyReader := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(bodyReader)

	formClose := "\r\n--" + writer.Boundary() + "--\r\n"

	closeReader := bytes.NewReader([]byte(formClose))

	mr.writer = writer

	mr.boundary = writer.Boundary()
	mr.contentType = writer.FormDataContentType()

	mr.readers = []io.Reader{bodyReader, closeReader}

	return
}

// SetBoundary method is multipart.Writer.SetBoundary copy
func (w *MultipartReader) SetBoundary(boundary string) (err error) {
	err = w.writer.SetBoundary(boundary)
	return
}

// AddReader adds new reader to MultipartReader
func (mr *MultipartReader) AddReader(r io.Reader) {
	i := len(mr.readers)
	mr.readers = append(mr.readers[:i-1], r, mr.readers[i-1])
}

// AddFormReader adds new reader as form part to MultipartReader
func (mr *MultipartReader) AddFormReader(name, filename string, r io.Reader) (err error) {
	var fw io.Writer
	if fw, err = mr.writer.CreateFormFile(name, filename); err != nil {
		return
	}
	if _, err = io.Copy(fw, r); err != nil {
		return
	}
	mr.length += length
	return
}

// https://stackoverflow.com/questions/20205796/post-data-using-the-content-type-multipart-form-data

// WriteFields writes multiple form fields to the multipart.Writer.
func (mr *MultipartReader) WriteFields(fields map[string]string) error {
	for key, value := range fields {
		form := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"%s\"\r\n\r\n", mr.boundary, key)
		mr.AddReader(strings.NewReader(form))
		mr.AddReader(strings.NewReader(value + "\r\n"))
	}

	return nil
}

// AddFile adds new file to MultipartReader
func (mr *MultipartReader) WriteFile(key, filename string) (err error) {
	fs, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fs.Close()

	form := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n\r\n", mr.boundary, "file", fs.Name())
	mr.AddReader(strings.NewReader(form))
	mr.AddReader(fs)
	return
}

// SetupHTTPRequest set multiReader and headers after adding readers
func (mr *MultipartReader) SetupRequest(req *http.Request) {
	req.Body = mr.GetCloseReader()
	req.Header.Add("Content-Type", mr.contentType)
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

func (mr *MultipartReader) Boundary() string {
	return mr.boundary
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
