package slicesync

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

const (
	AUTOSIZE = 0 // Use AUTOSIZE when you don't know or care for the total file or slice size 
)

// LimitedReadCloser reads just N bytes from a reader and allows to close it as well
type LimitedReadCloser struct {
	io.LimitedReader
}

// Close is the Closer interface implementation
func (l *LimitedReadCloser) Close() error {
	return (l.R).(io.Closer).Close()
}

// HasInfo info to be sent back
type HashInfo struct {
	Size   int64
	Offset int64
	Slice  int64
	Hash   string
}

// HashNDumper is the Service (local or remote) allowing slice based file synchronizations
type HashNDumper interface {
	Hash(filename string, offset, slice int64) (*HashInfo, error)
	Dump(filename string, offset, slice int64) (io.ReadCloser, error)
}

// LocalHashNDump implements the HashNDump Service locally
type LocalHashNDump struct {
	Dir string
}

// Hash returns the Hash (sha-1) for a file slice or the full file
// slice size of 0=AUTOSIZE means "rest of the file"
func (hnd *LocalHashNDump) Hash(filename string, offset, slice int64) (
	hi *HashInfo, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
		}
	}()
	hi = hash(calcpath(hnd.Dir, filename), offset, slice)
	return
}

// Dump opens a file to read just a slice of it
func (hnd *LocalHashNDump) Dump(filename string, offset, slice int64) (
	rc io.ReadCloser, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
		}
	}()
	rc = dump(calcpath(hnd.Dir, filename), offset, slice)
	return
}

// calcpath joins dir to filename to get a full path and panics if the result is not within dir
func calcpath(dir, filename string) string {
	fullpath, err := filepath.Abs(filepath.Join(dir, filename))
	autopanic(err)
	fullpath = filepath.Clean(fullpath)
	fulldir, err:= filepath.Abs(dir)
	autopanic(err)
	if !filepath.HasPrefix(fullpath, fulldir) {
		panic(fmt.Errorf("Illegal filename %s, not within %s!", filename, fulldir))
	}
	return fullpath
}

// hash is the internal function that calculates the local hash of the given slice of filename
func hash(filename string, offset, slice int64) *HashInfo {
	file, err := os.Open(filename) // For read access
	autopanic(err)
	defer file.Close()
	fi, err := file.Stat()
	autopanic(err)
	toread := sliceFile(file, fi.Size(), offset, slice)
	hash := ""
	if toread > 0 {
		h := sha1.New()
		_, err = io.CopyN(h, file, toread)
		autopanic(err)
		hash = "sha1-" + fmt.Sprintf("%x", h.Sum(nil))
	}
	return &HashInfo{fi.Size(), offset, toread, hash}
}

// dump is the internal function that opens the file to read just a slice of it
func dump(filename string, offset, slice int64) io.ReadCloser {
	file, err := os.Open(filename) // For read access
	autopanic(err)
	fi, err := file.Stat()
	autopanic(err)
	toread:=sliceFile(file, fi.Size(), offset, slice)
	return &LimitedReadCloser{io.LimitedReader{file, toread}}
}

// sliceFile positions to the offset pos of file and prepares to read up to slice bytes of it.
// It returns the proper slice to read before the end of the file is reached: 
// When input slice is 0 or slice would read past the file's end, it returns the remaining length 
// to read before EOF
func sliceFile(file *os.File, max, offset, slice int64) int64 {
	if offset > 0 {
		_, err := file.Seek(offset, os.SEEK_SET)
		autopanic(err)
	}
	toread:=slice
	if slice == AUTOSIZE || (offset+slice) > max {
		toread = max - offset
	}
	return toread
}

// autopanic panic on any non-nil error
func autopanic(err error) {
	if err != nil {
		fmt.Println("Got error:",err)
		panic(err)
	}
}