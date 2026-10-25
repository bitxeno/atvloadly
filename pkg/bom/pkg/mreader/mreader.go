// streaming multiple io.Reader as one
package mreader

import (
	"errors"
	"io"
	"strings"
)

type mreader struct {
	r []io.ReadCloser
	i int
}

func New() *mreader {
	return &mreader{r: []io.ReadCloser{}}
}

func (r *mreader) Add(re io.ReadCloser) {
	r.r = append(r.r, re)
}

func (r *mreader) Read(p []byte) (int, error) {
	if r.i >= len(r.r) {
		return 0, io.EOF
	}
	n, err := r.r[r.i].Read(p)
	if err == io.EOF {
		r.i++
	}
	if err != nil && err != io.EOF {
		return n, err
	}
	if len(p) > n {
		n2, err2 := r.Read(p[n:])
		n += n2
		err = err2
	}
	return n, err
}

func (r *mreader) Close() error {
	var err []string
	for _, v := range r.r {
		if e := v.Close(); e != nil {
			err = append(err, e.Error())
		}
	}
	return errors.New(strings.Join(err, ","))
}
