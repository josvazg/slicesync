package slicesync

import (
	"fmt"
	"io"
	"os"
)

const (
	UnknownSize = -1
)

// sliceSync contains the full spec for a remote file slice synchronization of remote filename 
// on server to local destfile while using alike to compare possible already local slices 
// of size slice 
type sliceSync struct {
	server, filename, dest, alike string
	slice, size                   int64
}

// Slicesync will copy remote filename from server to local dir or over alike 
// filename  remote file to sync from
// destfile  local destination file to sync to, same as filename if omitted
// alike     is the local file to compare similar to remote, same as destfile if omitted
// slice     is the size of each slice to sync
func Slicesync(server, filename, destfile, alike string, slice int64) error {
	dst := destfile
	if dst == "" {
		dst = filename
	}
	if alike == "" {
		alike = dst
	}
	if slice == 0 || !exists(alike) {
		return Download(server, filename, dst)
	}
	sSync := &sliceSync{server, filename, dst, alike, slice, UnknownSize}
	return sSync.sync()
}

// Download gets the remote file at server completely
func Download(server, filename, destfile string) error {
	fmt.Println("Downloading " + filename + " from " + server)
	orig, err := ROpen(shortUrl(server, "", filename))
	if err != nil {
		return err
	}
	dest, err := writeAt(destfile, 0)
	if err != nil {
		return err
	}
	return copyNClose(dest, orig)
}

// string describes the sliceSync
func (s *sliceSync) string() string {
	return fmt.Sprintf("sliceSync{%s/%s -> %s (%s) slice=%v}\n",
		s.server, s.filename, s.dest, s.alike, s.slice)
}

// sync does the full synchronization
func (s *sliceSync) sync() error {
	fmt.Printf("Syncing %v\n", s)
	// first sync is special, we may not know the remote file size 
	err := s.syncSlice(0)
	if err != nil {
		return err
	}
	// size now must be known and CANNOT change!
	fmt.Println("size:", s.size)
	if s.size > s.slice {
		pos := s.slice
		for ; pos < s.size; pos += s.slice {
			err = s.syncSlice(pos)
			if err != nil {
				return err
			}
		}
	}
	// final check
	err = s.check()
	if err != nil {
		return err
	}
	return nil
}

// syncSlice does the synchronization of a remote filename slice [pos:pos+slice]
func (s *sliceSync) syncSlice(pos int64) error {
	remote, local, err := s.hashes(pos, s.slice)
	if err != nil {
		return err
	}
	var orig io.ReadCloser
	if local.Hash == remote.Hash {
		if s.alike == s.dest { // if alike is same as dest there is no need to copy anything
			fmt.Printf("%v(+%v) is fine\n", pos, s.slice)
			return nil
		}
		orig, _, err = Dump(s.alike, pos, s.slice)
		fmt.Printf("%v(+%v) local copy\n", pos, s.slice)
		if err != nil {
			return err
		}
	} else {
		orig, err = RDump(s.server, s.filename, pos, s.slice)
		fmt.Printf("%v(+%v) remote dump\n", pos, s.slice)
		if err != nil {
			return err
		}
	}
	fmt.Printf("write at %v\n", pos)
	target, err := writeAt(s.dest, pos)
	if err != nil {
		return err
	}
	return copyNClose(target, orig)
}

// check compares remote and local hash after a sync and returns error if they don't match
func (s *sliceSync) check() error {
	remote, local, err := s.hashes(0, AUTOSIZE)
	if err != nil {
		return err
	}
	if remote.Hash != local.Hash {
		return fmt.Errorf("%s/%s file size changed! (expected %v but got %v)",
			s.server, s.filename, s.size, remote.Size)
	}
	fmt.Printf("Hash are ok: %v=%v\n", remote.Hash, local.Hash)
	return nil
}

// hashes returns both the remote and local hashs
// the size is updated if unkown and checked to be constant if it was already known
func (s *sliceSync) hashes(pos, slice int64) (remote, local *FileInfo, err error) {
	remote, err = RHash(s.server, s.filename, pos, slice)
	if err != nil {
		return
	}
	if s.size == UnknownSize {
		s.size = remote.Size
	} else if s.size != remote.Size {
		err=fmt.Errorf("%s/%s file size changed! (expected %v but got %v)",
			s.server, s.filename, s.size, remote.Size)
		return
	}
	local, err = Hash(s.alike, pos, slice)
	if err != nil {
		return
	}
	return
}

// writeAt opens a file to write at position pos, ensuring the file is big enough
func writeAt(filename string, pos int64) (io.WriteCloser, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0750) // For read access.
	if err != nil {
		return nil, err
	}
	newpos, err := file.Seek(pos, os.SEEK_SET)
	if err != nil {
		return nil, err
	}
	if newpos != pos {
		fmt.Errorf("Couldn't seek right, expected position %v, but got %v! ", pos, newpos)
		return nil, err
	}
	return file, nil
}

// copyNClose copies all r to w and closes both w and r
func copyNClose(w io.WriteCloser, r io.ReadCloser) error {
	defer w.Close()
	defer w.Close()
	_, err := io.Copy(w, r)
	return err
}

// Does the file exists?
func exists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	}
	return false
}
