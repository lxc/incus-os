package rest

import (
	"io"
)

type countWrapper struct {
	io.ReadCloser

	n int
}

func (w *countWrapper) Read(p []byte) (int, error) {
	n, err := w.ReadCloser.Read(p)
	w.n += n

	return n, err
}
