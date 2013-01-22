package slicesync

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
)

const (
	AUTOSIZE = 0
)

type LimitedReadCloser struct {
	io.LimitedReader
}

// FileInfo info to be sent back
type FileInfo struct {
	size   int64
	offset int64
	slice  int64
	hash   string
}

func (l *LimitedReadCloser) Close() error {
	return (l.R).(io.Closer).Close()
}

// Hash returns the Hash (sha-1) for a file slice or the full file
func Hash(filename string, offset, slice int64) (*FileInfo, error) {
	file, err := os.Open(filename) // For read access.
	if err != nil {
		return nil, err
	}
	defer file.Close()
	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}
	slice, err = sliceFile(file, offset, slice)
	if err != nil {
		return nil, err
	}
	hash := ""
	if slice > 0 {
		h := sha1.New()
		io.CopyN(h, file, slice)
		hash = "sha1-" + fmt.Sprintf("%x", h.Sum(nil))
	}
	return &FileInfo{fi.Size(), offset, slice, hash}, nil
}

// Dump opens a file to read just a slice of it
func Dump(filename string, offset, slice int64) (io.ReadCloser, int64, error) {
	file, err := os.Open(filename) // For read access.
	if err != nil {
		return nil, 0, err
	}
	slice, err = sliceFile(file, offset, slice)
	if err != nil {
		return nil, 0, err
	}
	return &LimitedReadCloser{io.LimitedReader{file, slice}}, slice, nil
}

// sliceFile positions to the offset pos of file and prepares to read up to slice bytes of it.
// It returns the proper slice to read before the end of the file is reached: 
// When input slice is 0 or slice would read past the file's end, it returns the remaining length 
// to read before EOF
func sliceFile(file *os.File, offset, slice int64) (int64, error) {
	if offset > 0 {
		_, err := file.Seek(offset, os.SEEK_SET)
		if err != nil {
			return 0, err
		}
	}
	fi, err := file.Stat()
	if err != nil {
		return 0, err
	}
	max := fi.Size()
	if slice == 0 || (offset+slice) > max {
		slice = max - offset
	}
	return slice, nil
}
