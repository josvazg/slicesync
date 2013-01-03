package slicesync

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
)

const (
	AUTOSIZE = 0
)

type LimitedReadCloser struct {
	io.LimitedReader
}

func (l *LimitedReadCloser) Close() error {
	return (l.R).(io.Closer).Close()
}

// Hash returns the Hash (sha-1) for a file slice or the full file
func Hash(filename string, offset, size int64) (string, error) {
	file, err := os.Open(filename) // For read access.
	if err != nil {
		return "", err
	}
	defer file.Close()
	size, err = sliceFile(file, offset, size)
	if err != nil {
		return "", err
	}
	h := sha1.New()
	io.CopyN(h, file, size)
	return "sha1-" + fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Dump opens a file to read just a slice of it
func Dump(filename string, offset, size int64) (io.ReadCloser, int64, error) {
	file, err := os.Open(filename) // For read access.
	if err != nil {
		return nil, 0, err
	}
	size, err = sliceFile(file, offset, size)
	if err != nil {
		return nil, 0, err
	}
	return &LimitedReadCloser{io.LimitedReader{file, size}}, size, nil
}

// sliceFile positions to the offset pos of file and prepares to read up to size bytes of it.
// It returns the proper size to read before the end of the file is reached: 
// When input size is 0 or size would read past the file's end, it returns the remaining length 
// to read before EOF
func sliceFile(file *os.File, offset, size int64) (int64, error) {
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
	if size == 0 || (offset+size) > max {
		size = max - offset
	}
	return size, nil
}

// HashNDumpServer prepares an HTTP Server to Hash and Dump slices of files remotely
func SetupHashNDump() {
	http.HandleFunc("/hash", hash)
	http.HandleFunc("/dump", dump)
	http.HandleFunc("/size", size)
}

// HashNDumpServer prepares an HTTP Server to Hash and Dump slices of files remotely
func HashNDumpServer(port int) {
	SetupHashNDump()
	http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
}

// hash is a http request handler to return hashes of file slices
func hash(w http.ResponseWriter, r *http.Request) {
	filename, offset, size, err := readArgs(w, r)
	if handleError(w, r, err) {
		return
	}
	hsh, err := Hash(filename, offset, size)
	if handleError(w, r, err) {
		return
	}
	io.WriteString(w, hsh)
}

// dump is a http request handler to return a file slice
func dump(w http.ResponseWriter, r *http.Request) {
	filename, offset, size, err := readArgs(w, r)
	if handleError(w, r, err) {
		return
	}
	sliced := !(offset == 0 && size == 0)
	slice, size, err := Dump(filename, offset, size)
	if handleError(w, r, err) {
		return
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%v", size))
	w.Header().Set("Content-Type", "application/octet-stream")
	downfilename := filename
	if sliced {
		downfilename = fmt.Sprintf("%s(%v-%v)%s", noExt(filename), offset, size, path.Ext(filename))
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downfilename))
	io.Copy(w, slice)
}

// size is a http request handler to return a file size
func size(w http.ResponseWriter, r *http.Request) {
	filename, _, _, err := readArgs(w, r)
	if handleError(w, r, err) {
		return
	}
	file, err := os.Open(filename) // For read access.
	if handleError(w, r, err) {
		return
	}
	fi, err := file.Stat()
	if handleError(w, r, err) {
		return
	}
	io.WriteString(w,fmt.Sprintf("%v",fi.Size()))
}

// noExt returns the name without the extension
func noExt(filename string) string {
	return filename[0 : len(filename)-len(path.Ext(filename))]
}

// readArgs reads request args for hash & dump
func readArgs(w http.ResponseWriter, r *http.Request) (f string, o, s int64, e error) {
	filename := r.FormValue("filename")
	if filename == "" {
		return "", 0, 0, fmt.Errorf("Expected filename argument!")
	}
	offset := r.FormValue("offset")
	size := r.FormValue("size")
	o = 0
	s = AUTOSIZE
	if offset != "" {
		i, err := strconv.ParseInt(offset, 10, 64)
		if err!=nil {
			return "", 0, 0, err
		}
		o = i
	}
	if size != "" {
		i, err := strconv.ParseInt(size, 10, 64)
		if err!=nil {
			return "", 0, 0, err
		}
		s = i
	}
	return filename, o, s, nil
}

// handleError displays err (if not nil) on Stderr and (if possible) displays a web error page
// it also returns true if the error was found and handled and false if err was nil
func handleError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}
