package reader

import (
	"io"
)

// zero copy reader
type blockReader struct {
	r    io.ReadSeeker
	addr int64 // base offset
	off  int64 // current offset
	len  int64 // limit read block range
}

// offset, current io.ReadSeeker offset
// len, total len to limit read block range
func New(r io.ReadSeeker, offset, len int64) *blockReader {
	return &blockReader{
		r:    r,
		addr: offset,
		len:  len,
	}
}

func (b *blockReader) Read(p []byte) (n int, err error) {

	if b.off-b.len >= 0 {
		return 0, io.EOF
	}
	addr := b.addr + b.off
	if _, err := b.r.Seek(int64(addr), 0); err != nil {
		return 0, err
	}

	maxLen := b.len - b.off
	if maxLen > int64(len(p)) {
		maxLen = int64(len(p))
	}

	n, err = b.r.Read(p[:maxLen])
	b.off += int64(n)

	return n, nil
}
