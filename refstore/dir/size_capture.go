package dir

import "io"

type sizeWriter struct {
	w io.Writer
	n int
}

func (sz *sizeWriter) Write(b []byte) (int, error) {
	n, err := sz.w.Write(b)
	sz.n += n
	return n, err
}
