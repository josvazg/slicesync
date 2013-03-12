package slicesync

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Diff marks a start offset and a size in bytes
type Diff struct {
	Offset, Size int64
}

// Diffs list the differences between two similar files, a remote filename and a local alike
type Diffs struct {
	RemoteHashNDump
	Filename, Alike          string
	Slice, Size, Differences int64
	Diffs                    []Diff
}

// NewDiffs creates a Diffs data type
func NewDiffs(server, filename, alike string, slice, size int64) *Diffs {
	return &Diffs{RemoteHashNDump{server}, filename, alike, slice, size, 0, make([]Diff, 0, 10)}
}

// String shows the diffs in a json representation
func (sd *Diffs) String() string {
	bytes, err := json.Marshal(sd)
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}

// hasback contains the response back to a remote hash request or an error
type hashback struct {
	HashInfo
	err error
}

// CalcDiffs returns the Diffs between remote filename and local alike
func CalcDiffs(server, filename, alike string, slice int64) (*Diffs, error) {
	localHnd := &LocalHashNDump{"."}
	diffs := NewDiffs(server, filename, alike, slice, AUTOSIZE)
	ch := make(chan hashback)
	indiff := false
	for pos := int64(0); pos == 0 || pos < diffs.Size; pos += slice {
		go hashnback(diffs, filename, pos, slice, ch)
		local, err := localHnd.Hash(alike, pos, slice)
		if err != nil { // exit if there is a local hash error
			return nil, err
		}
		remote := <-ch
		if remote.err != nil { // exit as well if there was a remote hash error
			return nil, remote.err
		}
		if diffs.Size == AUTOSIZE { // update the size and store in the first position
			diffs.Size = remote.Size
		}
		if indiff {
			if local.Hash == remote.Hash {
				indiff = false
			} else {
				diffs.Diffs[len(diffs.Diffs)-1].Size += slice
				diffs.Differences += slice
			}
		} else if local.Hash != remote.Hash {
			diffs.Diffs = append(diffs.Diffs, Diff{pos, slice})
			diffs.Differences += slice
			indiff = true
		}
	}
	return diffs, nil
}

// Slicesync will copy remote filename from server to local dir or over alike 
// filename  remote file to sync from
// destfile  local destination file to sync to, same as filename if omitted
// alike     is the local file to compare similar to remote, same as destfile if omitted
// slice     is the size of each slice to sync
// it returns the sync Stats or an error if anything went wrong
// Slicesync steps are as follows
// 1. Copy local alike file as target destfile (if they are not the same file)
// 2. Calculate Diffs differences between alike and (remote) filename
// 3. Calculate remote hash for filename
// 4. Download differences only
// 5. Calculate local hash
// 6. Check local hash against remote hash
// 7. Return the Diffs
//
// * Steps 1-3 are completely independent, so they are run on different goroutines
// * Step 4 (download) depends on 1 (local copy) and 2 (diffs)
// * Step 5 (local hash) depends on 4 (download)
// * Step 6 depends on 5 (local hash) and 3 (remote hash)
//
func Slicesync(server, filename, destfile, alike string, slice int64) (*Diffs, error) {
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
	// 1+2+3) localcopy + diffs + remote hash
	hashch := make(chan hashback)
	copych := make(chan error)
	diffs := NewDiffs(server, filename, alike, slice, AUTOSIZE)
	// remote hash
	go hashnback(diffs, filename, 0, AUTOSIZE, hashch)
	if slice == 0 || !exists(alike) { // no diffs
		diffs.Diffs = append(diffs.Diffs, Diff{0, AUTOSIZE})
	} else {
		// filecopy
		go func(destfile, alike string, ch chan error) {
			_, err := filecopy(dst, alike)
			ch <- err
		}(destfile, alike, copych)
		// diff
		var err error
		diffs, err = CalcDiffs(server, filename, alike, slice)
		if err != nil {
			return nil, err
		}
		err = <-copych
		if err != nil {
			return nil, err
		}
	}
	// 4) download
	downloaded, err := Download(dst, diffs)
	if err != nil {
		return nil, err
	}
	if diffs.Differences == 0 && diffs.Size == AUTOSIZE && len(diffs.Diffs) == 1 {
		diffs.Differences, diffs.Size = downloaded, downloaded
	}
	// 5) local hash
	hnd := &LocalHashNDump{"."}
	local, err := hnd.Hash(dst, 0, AUTOSIZE)
	if err != nil {
		return nil, err
	}
	// 6) check
	remote := <-hashch
	if remote.err != nil {
		return nil, remote.err
	}
	if local.Hash != remote.Hash {
		return nil, fmt.Errorf("Hash error, expected '%s' but got '%s'!", local.Hash, remote.Hash)
	}
	return diffs, nil
}

// Download gets the remote file described in Diffs and saves into dst
// (dst may exist as Download will only fill differences)
func Download(dst string, diffs *Diffs) (int64, error) {
	downloaded := int64(0)
	for _, diff := range diffs.Diffs {
		orig, err := diffs.Dump(diffs.Filename, diff.Offset, diff.Size)
		if err != nil {
			return downloaded, err
		}
		target, err := writeAt(dst, diff.Offset)
		if err != nil {
			return downloaded, err
		}
		done, err := copyNClose(target, orig)
		if err != nil {
			return downloaded, err
		}
		downloaded += done
	}
	return downloaded, nil
}

// hashnback does a RHash and returns the hashback result through the given channel
func hashnback(rhnd HashNDumper, filename string, pos, slice int64, ch chan hashback) {
	if r, err := rhnd.Hash(filename, pos, slice); err == nil {
		ch <- hashback{HashInfo{r.Size, r.Offset, r.Slice, r.Hash}, nil}
	} else {
		ch <- hashback{err: err}
	}
}

// writeAt opens a file to write at position pos, ensuring the file is big enough
func writeAt(filename string, pos int64) (io.WriteCloser, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0750) // For write access
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

// filecopy opens and copies file 'from' to file 'to' completely
func filecopy(to, from string) (int64, error) {
	fromfile, err := os.Open(from)
	if err != nil {
		return 0, err
	}
	tofile, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY, 0750) // For write access
	if err != nil {
		return 0, err
	}
	return copyNClose(tofile, fromfile)
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
