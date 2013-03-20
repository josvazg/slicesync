package slicesync

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	AUTOSIZE        = 0 // Use AUTOSIZE when you don't know or care for the total file or slice size 
	MiB             = 1048576
	Version         = "0.0.1"
	SliceSyncExt    = ".slicesync"
	TmpSliceSyncExt = ".tmp" + SliceSyncExt
	bufferSize      = 1024
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
	BulkHash(filename string, slice int64) (io.ReadCloser, error)
	Dump(filename string, offset, slice int64) (io.ReadCloser, int64, error)
}

// LocalHashNDump implements the HashNDump Service locally
type LocalHashNDump struct {
	Dir string
}

// HashDir prepares the hashes of all files in the given directory, recursively if asked to
// Blocking single threaded function (no go-routines), for quite a heavy background process
// It returns any error it encounters in the process
func HashDir(dir string, slice int64, recursive bool) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s is not a Directory!", dir)
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	fis, err := d.Readdir(0)
	if err != nil {
		return err
	}
	for _, f := range fis {
		if needsHashing(f, slice, dir) {
			if err := HashFile(filepath.Join(dir, f.Name()), slice); err != nil {
				return err
			}
		}
	}
	if recursive {
		for _, f := range fis {
			if f.IsDir() {
				if err := HashDir(filepath.Join(dir, f.Name()), slice, recursive); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// hashFile produces a filename+".slicesync" file with the full bulkhash dump of filename
func HashFile(filename string, slice int64) (err error) {
	done := false
	if slice <= 0 { // protection against infinite loop by bad arguments
		slice = MiB
	}
	fhdump, err := os.OpenFile(filename+TmpSliceSyncExt, os.O_CREATE|os.O_WRONLY, 0750)
	if err != nil {
		return err
	}
	defer func() {
		if done {
			err = os.Rename(filename+TmpSliceSyncExt, filename+SliceSyncExt)
		}
	}()
	defer fhdump.Close()
	file, err := os.Open(filename) // For read access
	if err != nil {
		return err
	}
	defer file.Close()
	fi, err := file.Stat()
	if err != nil {
		return err
	}
	bulkHashDump(fhdump, file, filename, slice, fi.Size())
	done = true
	return
}

// BulkHash calculates the file hash and all hashes of size slice and writes them to w
//
// Output is as follows:
// first text line is the file size
// then there are size / slice lines each with a slice hash for consecutive slices
// finally there is the line "Final: "+total file hash
//
// Post initialization errors are dumped in-line starting as a "Error: "... line
// Nothing more is sent after an error occurs and is dumped to w
func (hnd *LocalHashNDump) BulkHash(filename string, slice int64) (rc io.ReadCloser, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
		}
	}()
	f, err := os.Lstat(filename)
	autopanic(err)
	fullfilename := calcpath(hnd.Dir, filename)
	if !hasBulkHashFile(f, slice, hnd.Dir) { // generate the bulkhash dump now
		file, err := os.Open(fullfilename) // For read access
		autopanic(err)
		if slice <= 0 { // protection against infinite loop by bad arguments
			slice = MiB
		}
		fi, err := file.Stat()
		autopanic(err)
		r, w := io.Pipe()
		/*fhdump, err := os.OpenFile(fullfilename, os.O_CREATE|os.O_WRONLY, 0750)
		autopanic(err)*/
		go func() {
			defer w.Close()
			//defer fhdump.Close()
			//bulkHashDump(io.MultiWriter(w, fhdump), file, filename, slice, fi.Size())
			bulkHashDump(w, file, filename, slice, fi.Size())
		}()
		return r, nil
	}
	// re-read the pre-generated bulkhash dump
	file, err := os.Open(fullfilename + SliceSyncExt) // For read access
	autopanic(err)
	r, w := io.Pipe()
	go func() {
		defer file.Close()
		defer w.Close()
		bufW := bufio.NewWriterSize(w, bufferSize)
		defer bufW.Flush()
		io.Copy(bufW, file)
	}()
	return r, nil
}

// bulkHashDump produces BulkHash output into the given writer for the given slice and file size
func bulkHashDump(w io.Writer, file io.ReadCloser, filename string, slice, size int64) {
	defer file.Close()
	bufW := bufio.NewWriterSize(w, bufferSize)
	defer bufW.Flush()
	sliceHash := newSliceHasher()
	fmt.Fprintf(bufW, "Version: %v\n", Version)
	fmt.Fprintf(bufW, "Filename: %v\n", filepath.Base(filename))
	fmt.Fprintf(bufW, "Slice: %v\n", slice)
	fmt.Fprintf(bufW, "Slice Hashing: %v\n", sliceHash.Name())
	fmt.Fprintf(bufW, "Length: %v\n", size)
	if size > 0 {
		h := newHasher()
		hashSink := io.MultiWriter(h, sliceHash)
		readed := int64(0)
		var err error
		for pos := int64(0); pos < size; pos += readed {
			toread := slice
			if toread > (size - pos) {
				toread = size - pos
			}
			readed, err = io.CopyN(hashSink, file, toread)
			if err != nil {
				fmt.Fprintf(bufW, "Error:%s\n", err)
				return
			}
			fmt.Fprintf(bufW, "%s\n", base64.StdEncoding.EncodeToString(sliceHash.Sum(nil)))
			sliceHash.Reset()
			bufW.Flush()
		}
		fmt.Fprintf(bufW, "%v: %x\n", h.Name(), h.Sum(nil))
	}
}

// needHashing returns true ONLY if there isn't a f.Name()+".slicesync" older than f.Name() itself
func needsHashing(f os.FileInfo, slice int64, dir string) bool {
	if !f.IsDir() && f.Size() > slice && !strings.HasSuffix(f.Name(), SliceSyncExt) {
		return !hasBulkHashFile(f, slice, dir)
	}
	return false
}

// hasBulkHashFile returns true if there is a valid bulkhash .slicesync file for filename
func hasBulkHashFile(f os.FileInfo, slice int64, dir string) bool {
	hdump, err := os.Lstat(filepath.Join(dir, f.Name()+SliceSyncExt))
	return err == nil && hdump != nil && hdump.ModTime().After(f.ModTime())
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
	hi = doHash(calcpath(hnd.Dir, filename), offset, slice, autoHasher(offset, slice))
	return
}

// Dump opens a file to read just a slice of it
func (hnd *LocalHashNDump) Dump(filename string, offset, slice int64) (
	rc io.ReadCloser, n int64, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
		}
	}()
	lrc := dump(calcpath(hnd.Dir, filename), offset, slice)
	return lrc, lrc.N, nil
}

// calcpath joins dir to filename to get a full path and panics if the result is not within dir
func calcpath(dir, filename string) string {
	fullpath, err := filepath.Abs(filepath.Join(dir, filename))
	autopanic(err)
	fullpath = filepath.Clean(fullpath)
	fulldir, err := filepath.Abs(dir)
	autopanic(err)
	if !filepath.HasPrefix(fullpath, fulldir) {
		panic(fmt.Errorf("Illegal filename %s, not within %s!", filename, fulldir))
	}
	return fullpath
}

// doHash is the internal function that calculates the local hash of the given slice of filename
func doHash(filename string, offset, slice int64, h namedHash) *HashInfo {
	file, err := os.Open(filename) // For read access
	autopanic(err)
	defer file.Close()
	fi, err := file.Stat()
	autopanic(err)
	toread := sliceFile(file, fi.Size(), offset, slice)
	hash := ""
	if toread > 0 {
		h.Reset()
		_, err = io.CopyN(h, file, toread)
		autopanic(err)
		hash = fmt.Sprintf("%v-%x", h.Name(), h.Sum(nil))
	}
	return &HashInfo{fi.Size(), offset, toread, hash}
}

// dump is the internal function that opens the file to read just a slice of it
func dump(filename string, offset, slice int64) *LimitedReadCloser {
	file, err := os.Open(filename) // For read access
	autopanic(err)
	fi, err := file.Stat()
	autopanic(err)
	toread := sliceFile(file, fi.Size(), offset, slice)
	return &LimitedReadCloser{io.LimitedReader{R: file, N: toread}}
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
	toread := slice
	if slice == AUTOSIZE || (offset+slice) > max {
		toread = max - offset
	}
	return toread
}

// autopanic panic on any non-nil error
func autopanic(err error) {
	if err != nil {
		fmt.Println("Got error:", err)
		panic(err)
	}
}
