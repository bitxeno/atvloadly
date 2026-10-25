package helper

import "io"

func ReadString(r io.Reader, len int) (string, error) {
	buf := make([]byte, len)
	n := 0
	for {
		i, err := r.Read(buf)
		if err != nil {
			return "", err
		}
		n += i
		if n >= len {
			break
		}
	}
	return string(buf), nil
}

func MustReadString(r io.Reader, len int) string {
	s, err := ReadString(r, len)
	if err != nil {
		panic(err)
	}
	return s
}
