package slicesync

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const (
	UnknownSize = -1
)

// Stats contains all the relevant stats after a slice synchronization
type Stats struct {
	Downloaded, Size int64
	Downloads        int
}

// Diff marks a start offset and a size in bytes
type Diff struct {
	Offset, Size int64
}

// Diffs list the differences between two similar files, a remote filename and a local alike
type Diffs struct {
	Server, Filename, Alike  string
	Slice, Size, Differences int64
	Diffs                    []Diff
}

// String shows the diffs in a json representation
func (sd *Diffs) String() string {
	bytes,err:=json.Marshal(sd)
	if err!=nil {
		return err.Error()
	}
	return string(bytes)
}

// sliceSync contains the full spec for a remote file slice synchronization of remote filename 
// on server to local destfile while using alike to compare possible already local slices 
// of size slice 
type sliceSync struct {
	Stats
	server, filename, dest, alike string
	slice                         int64
}

// hasback contains the response back to a remote hash request or an error
type hashback struct {
	fi  *FileInfo
	err error
}

// CalcDiffs returns the Diffs between remote filename and local alike
func CalcDiffs(server, filename, alike string, slice int64) (*Diffs, error) {
	sdiffs := &Diffs{server, filename, alike, slice, UnknownSize, 0, make([]Diff, 0, 10)}
	ch := make(chan hashback)
	indiff := false
	for pos := int64(0); pos == 0 || pos < sdiffs.Size; pos += slice {
		go Hashback(server, filename, pos, slice, ch)
		local, err := Hash(alike, pos, slice)
		if err != nil { // exit if there is a local hash error
			return nil, err
		}
		remote := <-ch
		if remote.err != nil { // exit as well if there was a remote hash error
			return nil, remote.err
		}
		if sdiffs.Size == UnknownSize { // update the size and store in the first position
			sdiffs.Size = remote.fi.Size
		}
		if indiff {
			if local.Hash == remote.fi.Hash {
				indiff = false
			} else {
				sdiffs.Diffs[len(sdiffs.Diffs)-1].Size += slice
				sdiffs.Differences+=slice
			}
		} else if local.Hash != remote.fi.Hash {
			sdiffs.Diffs = append(sdiffs.Diffs, Diff{pos, slice})
			sdiffs.Differences+=slice
			indiff = true
		}
	}
	return sdiffs, nil
}

// Hashback does a RHash and returns the hashback result through the given channel
func Hashback(server, filename string, pos, slice int64, ch chan hashback) {
	remote, err := RHash(server, filename, pos, slice)
	ch <- hashback{remote, err}
}

// Slicesync will copy remote filename from server to local dir or over alike 
// filename  remote file to sync from
// destfile  local destination file to sync to, same as filename if omitted
// alike     is the local file to compare similar to remote, same as destfile if omitted
// slice     is the size of each slice to sync
// it returns the sync Stats or an error if anything went wrong
// Slicesync steps are as follows
// 1. Copy local alike file as target destfile (if they are not the same file)
// 2. GenerateSliceDiff differences between alike and (remote) filename
// 3. Calculate remote hash for filename
// 4. Download differences only
// 5. Calculate local hash
// 6. Check local hash against remote hash
// 7. Return the Diffs
//
// * Steps 1-3 are completely independent, so they are run on different goroutines
// * Step 4 (download) depends on 2 (diffs)
// * Step 5 (local hash) depends on 4 (download)
// * Step 6 depends on 5 (local hash) and 3 (remote hash)
//
func Slicesync(server, filename, destfile, alike string, slice int64) (*Stats, error) {	
	dst := destfile
	if dst == "" {
		dst = filename
	}
	if alike != "" && !exists(alike) {
		return nil, fmt.Errorf("alike file '%s' does not exist!", alike)
	}
	if alike == "" {
		alike = dst
	}
	sSync := &sliceSync{server: server, filename: filename, dest: dst, alike: alike,
		slice: slice, Stats: Stats{Size: UnknownSize}}
	if slice == 0 || !exists(alike) {
		bytes, err := sSync.download()
		if err != nil {
			return nil, err
		}
		return &Stats{bytes, bytes, 1}, nil
	}
	err := sSync.sync()
	return &sSync.Stats, err
}

// Download gets the remote file at server completely
func (s *sliceSync) download() (int64, error) {
	//fmt.Println("Downloading " + filename + " from " + server)
	orig, err := ROpen(shortUrl(s.server, "/dump/", s.filename))
	if err != nil {
		return 0, err
	}
	dest, err := writeAt(s.dest, 0)
	if err != nil {
		return 0, err
	}
	done, err := copyNClose(dest, orig)
	if err != nil {
		return done, err
	}
	fmt.Println("Download done. Now Checking...")
	return done, s.check()
}

// string describes the sliceSync
func (s *sliceSync) string() string {
	return fmt.Sprintf("sliceSync{%s/%s -> %s (%s) slice=%v}\n",
		s.server, s.filename, s.dest, s.alike, s.slice)
}

// sync does the full synchronization
func (s *sliceSync) sync() error {
	//fmt.Printf("Syncing %v\n", s)
	// first sync is special, we may not know the remote file size 
	downloaded, bytes, err := s.syncSlice(0)
	if err != nil {
		return err
	}
	if downloaded {
		s.Downloads++
		s.Downloaded += bytes
	}
	// size now must be known and CANNOT change!
	//fmt.Println("size:", s.size)
	if s.Size > s.slice {
		pos := s.slice
		for ; pos < s.Size; pos += s.slice {
			downloaded, bytes, err = s.syncSlice(pos)
			if downloaded {
				s.Downloads++
				s.Downloaded += bytes
			}
			if err != nil {
				return err
			}
		}
	}
	// final check
	return s.check()
}

// syncSlice does the synchronization of a remote filename slice [pos:pos+slice]
func (s *sliceSync) syncSlice(pos int64) (bool, int64, error) {
	downloaded := false
	remote, local, err := s.hashes(pos, s.slice, s.alike)
	if err != nil {
		return false, 0, err
	}
	var orig io.ReadCloser
	if local.Hash == remote.Hash {
		if s.alike == s.dest { // if alike is same as dest there is no need to copy anything
			//fmt.Printf("%v(+%v) is fine\n", pos, s.slice)
			return false, 0, nil
		}
		orig, _, err = Dump(s.alike, pos, s.slice)
		//fmt.Printf("%v(+%v) local copy\n", pos, s.slice)
		if err != nil {
			return false, 0, err
		}
	} else {
		orig, err = RDump(s.server, s.filename, pos, s.slice)
		//fmt.Printf("%v(+%v) remote dump\n", pos, s.slice)
		if err != nil {
			return false, 0, err
		}
		downloaded = true
	}
	//fmt.Printf("write at %v\n", pos)
	target, err := writeAt(s.dest, pos)
	if err != nil {
		return false, 0, err
	}
	action := "Copy"
	if downloaded {
		action = "Download"
	}
	fmt.Printf("%d %s...\n", (pos / s.slice), action)
	bytes, err := copyNClose(target, orig)
	fmt.Printf("%d %s done\n", (pos / s.slice), action)
	return downloaded, bytes, err
}

// check compares remote and local hash after a sync and returns error if they don't match
func (s *sliceSync) check() error {
	remote, local, err := s.hashes(0, s.Size, s.dest)
	if err != nil {
		return err
	}
	if remote.Hash != local.Hash {
		return fmt.Errorf("%s/%s file hash is wrong! (expected %s but got %s)",
			s.server, s.dest, remote.Hash, local.Hash)
	}
	//fmt.Printf("Hash are ok: %v=%v\n", remote.Hash, local.Hash)
	return nil
}

// hashes returns both the remote and local hashs
// the size is updated if unkown and checked to be constant if it was already known
func (s *sliceSync) hashes(pos, slice int64, lfile string) (remote, local *FileInfo, err error) {
	ch := make(chan hashback)
	go func(ch chan hashback) {
		remote, err := RHash(s.server, s.filename, pos, slice)
		ch <- hashback{remote, err}
	}(ch)
	local, err = Hash(lfile, pos, slice)
	if err != nil {
		return
	}
	hof := <-ch
	remote = hof.fi
	err = hof.err
	if err != nil {
		return
	}
	if s.Size == UnknownSize {
		s.Size = remote.Size
	} else if s.Size != remote.Size {
		err = fmt.Errorf("%s/%s file size changed! (expected %v but got %v)",
			s.server, s.filename, s.Size, remote.Size)
		return
	}
	fmt.Printf("%v hash pair done\n", (pos / slice))
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
func copyNClose(w io.WriteCloser, r io.ReadCloser) (int64, error) {
	defer w.Close()
	defer w.Close()
	return io.Copy(w, r)
}

// Does the file exist?
func exists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	}
	return false
}
