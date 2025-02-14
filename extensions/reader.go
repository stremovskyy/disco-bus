package extensions

import (
	"bytes"
	"compress/gzip"
)

type ResettableGzipReader struct {
	gr *gzip.Reader
}

func (r *ResettableGzipReader) Reset(p []byte) error {
	var err error
	if r.gr == nil {
		r.gr, err = gzip.NewReader(bytes.NewReader(p))
	} else {
		err = r.gr.Reset(bytes.NewReader(p))
	}
	return err
}

func (r *ResettableGzipReader) Read(p []byte) (int, error) {
	return r.gr.Read(p)
}

func (r *ResettableGzipReader) Close() error {
	return r.gr.Close()
}
