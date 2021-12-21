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
	mr = &MultipartReader{}

	bodyReader := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(bodyReader)

	formClose := "\r\n--" + writer.Boundary() + "--\r\n"

	closeReader := bytes.NewReader([]byte(formClose))

	mr.writer = writer

	mr.boundary = writer.Boundary()
	mr.contentType = writer.FormDataContentType()
	mr.length = int64(bodyReader.Len() + len(formClose))

	mr.readers = []io.Reader{bodyReader, closeReader}

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
func (mr *MultipartReader) AddFormReader(name, filename string, length int64, r io.Reader) {
	form := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n\r\n", mr.boundary, name, filename)
	mr.AddReader(strings.NewReader(form), int64(len(form)))
	mr.AddReader(r, length)
	return
}

// https://stackoverflow.com/questions/20205796/post-data-using-the-content-type-multipart-form-data
func (mr *MultipartReader) AddValuesReader(values map[string]io.Reader) (err error) {
	for key, r := range values {
		var fw io.Writer
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		if x, ok := r.(*os.File); ok {
			if fw, err = mr.writer.CreateFormFile(key, x.Name()); err != nil {
				return
			}
		} else {
			if fw, err = mr.writer.CreateFormField(key); err != nil {
				return
			}
		}
		if _, err = io.Copy(fw, r); err != nil {
			return err
		}

	}
	return
}

// WriteFields writes multiple form fields to the multipart.Writer.
func (mr *MultipartReader) WriteFields(fields map[string]string) error {
	for key, value := range fields {
		b := strings.NewReader(value)
		w, err := mr.writer.CreateFormField(key)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, b); err != nil {
			return err
		}
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

	stat, err := fs.Stat()
	if err != nil {
		return err
	}

	form := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\n\r\n", mr.boundary, "file", fs.Name())
	mr.AddReader(strings.NewReader(form), int64(len(form)))
	mr.AddReader(fs, stat.Size())
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

func (mpr *MultipartReader) Close() error {
	return mpr.writer.Close()
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
