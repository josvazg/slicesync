package slicesync

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

const (
	AUTOSIZE        = 0 // Use AUTOSIZE when you don't know or care for the total file or slice size 
	MiB             = 1048576
	Version         = "1"
	SliceSyncExt    = ".slicesync"
	SlicesyncDir    = SliceSyncExt + "/"
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

// HashNDumper is the Service (local or remote) allowing slice based file synchronizations
type HashNDumper interface {
	Hash(filename string) (io.ReadCloser, error)
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
	return hashDir(dir, "", slice, recursive)
}

// hashDir performs HashDir recursive work
func hashDir(basedir, reldir string, slice int64, recursive bool) error {
	dir := filepath.Join(basedir, reldir)
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
		filename := filepath.Join(reldir, f.Name())
		if needsHashing(f, slice, dir, filename) {
			if err := HashFile(basedir, filename, slice); err != nil {
				return err
			}
		}
	}
	if recursive {
		for _, f := range fis {
			if f.IsDir() && f.Name() != SlicesyncDir {
				if err := hashDir(basedir, filepath.Join(reldir, f.Name()), slice, recursive); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// hashFile produces a ".slicesync/"+filename+".slicesync" file with the full hash dump of filename
//
// Output is as follows:
// * First there is a hash info header containing:
// ** Version: of the hash dump
// ** Filename: hashed
// ** Slice: size of each sliced block
// ** Slice Hashing: algorithm chosen for hashing
// ** Length: of the file
// * Then there are size / slice lines each with a slice hash for consecutive slices
// * And finally there is the line {File Hashing name}+": "+total file hash 
//
// (File Hashing algorithm is usually different from )
func HashFile(basedir, filename string, slice int64) (err error) {
	tmpFile := tmpSlicesyncFile(basedir, filename)
	hashFile := slicesyncFile(basedir, filename)
	done := false
	if slice <= 0 { // protection against infinite loop by bad arguments
		slice = MiB
	}
	mkdirs4File(tmpFile)
	fhdump, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY, 0750)
	if err != nil {
		return err
	}
	defer func() {
		if done {
			err = os.Rename(tmpFile, hashFile)
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
	hashDump(fhdump, file, filename, slice, fi.Size())
	done = true
	return
}

// Hash dumps a precalculated (by HashFile) file hash dump
func (hnd *LocalHashNDump) Hash(filename string) (rc io.ReadCloser, err error) {
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
	hfile := slicesyncFile(hnd.Dir, filename)
	if !isHashFileValid(f, hfile) {
		return nil, fmt.Errorf("Hash dump (file %v) not valid for %v at %v!\n", hfile, filename, hnd.Dir)
	}
	file, err := os.Open(hfile) // For read access
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

// hashDump produces a Hash dump output into the given writer for the given slice and file size
func hashDump(w io.Writer, file io.ReadCloser, filename string, slice, size int64) {
	defer file.Close()
	bufW := bufio.NewWriterSize(w, bufferSize)
	defer bufW.Flush()
	sliceHash := NewSliceHasher()
	fmt.Fprintf(bufW, "Version: %v\n", Version)
	fmt.Fprintf(bufW, "Filename: %v\n", filepath.Base(filename))
	fmt.Fprintf(bufW, "Slice: %v\n", slice)
	fmt.Fprintf(bufW, "Slice Hashing: %v\n", sliceHash.Name())
	fmt.Fprintf(bufW, "Length: %v\n", size)
	if size > 0 {
		h := NewHasher()
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

// needHashing returns true ONLY if there isn't a hash for filename at basedir
func needsHashing(f os.FileInfo, slice int64, basedir, filename string) bool {
	if !f.IsDir() && f.Size() > slice {
		return !isHashFileValid(f, slicesyncFile(basedir, filename))
	}
	return false
}

// isHashFileValid returns true if there is a valid hash dump (.slicesync) file for filename
func isHashFileValid(f os.FileInfo, hfilename string) bool {
	hdump, err := os.Lstat(hfilename)
	return err == nil && hdump != nil && !hdump.ModTime().Before(f.ModTime())
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

// tmpSlicesyncFile returns the corresponding temporary .tmp.slicesync file for filename
func tmpSlicesyncFile(basedir, filename string) string {
	return filepath.Join(slicesyncDir(basedir, filename), filepath.Base(filename)+TmpSliceSyncExt)
}

// slicesyncFile returns the corresponding .slicesync file for filename
func slicesyncFile(basedir, filename string) string {
	return filepath.Join(slicesyncDir(basedir, filename), filepath.Base(filename)+SliceSyncExt)
}

// slicesyncDir returns the .slicesync based directory location of a given file
func slicesyncDir(basedir, filename string) string {
	return filepath.Join(basedir, SlicesyncDir, filepath.Dir(filename))
}

// mkdirs4File ensures filename's dir is make if it needs to be
func mkdirs4File(filename string) {
	os.MkdirAll(filepath.Dir(filename), 0750)
}

// autopanic panic on any non-nil error
func autopanic(err error) {
	if err != nil {
		fmt.Println("Got error:", err)
		panic(err)
	}
}
