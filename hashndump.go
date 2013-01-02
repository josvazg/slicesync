package slicesync

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
)

type LimitedReadCloser struct {
	io.LimitedReader
}

func (l *LimitedReadCloser) Close() error {
	return (l.R).(io.Closer).Close()
}

// Hash returns the Hash (sha-1) for a file slice or the full file
func Hash(filename string, start, len int64) (string, error) {
	file, err := os.Open(filename) // For read access.
	if err != nil {
		return "", err
	}
	defer file.Close()
	len, err = sliceFile(file, start, len)
	if err != nil {
		return "", err
	}
	h := sha1.New()
	io.CopyN(h, file, len)
	return "sha1-" + fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Dump opens a file to read just a slice of it
func Dump(filename string, start, len int64) (io.ReadCloser, error) {
	file, err := os.Open(filename) // For read access.
	if err != nil {
		return nil, err
	}
	len, err = sliceFile(file, start, len)
	if err != nil {
		return nil, err
	}
	return &LimitedReadCloser{io.LimitedReader{file, len}}, nil
}

// sliceFile positions to the start pos of file and prepares to read up to len bytes of it.
// It returns the proper length to read before the end of the file is reached: 
// When input len is 0 or len would read past the file's end, it returns the remaining length 
// to read before EOF
func sliceFile(file *os.File, start, len int64) (int64, error) {
	if start > 0 {
		_, err := file.Seek(start, os.SEEK_SET)
		if err != nil {
			return 0, err
		}
	}
	fi, err := file.Stat()
	if err != nil {
		return 0, err
	}
	max := fi.Size()
	if len == 0 || (start+len) > max {
		len = max - start
	}
	return len, nil
}
