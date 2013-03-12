package slicesync

import (
	"crypto/sha1"
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
// * Steps 4+5 can be done concurrently, taking some care:
//   * Step 4 (download) depends on 1 (local copy) and 2 (diffs)
//   * Step 5 (local hash) depends on 4 but can be done by phases just as the download progresses
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
	rhashch := make(chan hashback)
	copych := make(chan error)
	diffs := NewDiffs(server, filename, alike, slice, AUTOSIZE)
	// remote hash
	go hashnback(diffs, filename, 0, AUTOSIZE, rhashch)
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
	// short-circuit if there is nothing to download (so there is nothing to check either)
	if len(diffs.Diffs) == 0 {
		return diffs, nil
	}
	// 4+5) download sends progress updates to local hash so that it can hash what's already done
	progressch := make(chan int64)
	lhashch := make(chan hashback)
	go hashWithProgress(dst, diffs.Size, progressch, lhashch)
	downloaded, err := Download(dst, diffs, progressch)
	if err != nil {
		return nil, err
	}
	if diffs.Differences == 0 && diffs.Size == AUTOSIZE && len(diffs.Diffs) == 1 {
		diffs.Differences, diffs.Size = downloaded, downloaded
	}
	// 6) check
	remote := <-rhashch
	if remote.err != nil {
		return nil, remote.err
	}
	local := <-lhashch
	if local.err != nil {
		return nil, local.err
	}
	if local.Hash != remote.Hash {
		return nil, fmt.Errorf("Hash error, expected '%s' but got '%s'!", local.Hash, remote.Hash)
	}
	return diffs, nil
}

// Download gets the remote file described in Diffs and saves into dst
// (dst may exist as Download will only fill differences)
func Download(dst string, diffs *Diffs, progressch chan int64) (int64, error) {
	if progressch != nil {
		defer close(progressch)
	}
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
		if progressch != nil {
			progressch <- (diff.Offset + done)
		}
	}
	return downloaded, nil
}

// hashnback does a Remote Hash and returns the hashback result through the given channel
func hashnback(rhnd HashNDumper, filename string, pos, slice int64, ch chan hashback) {
	if r, err := rhnd.Hash(filename, pos, slice); err == nil {
		ch <- hashback{HashInfo{r.Size, r.Offset, r.Slice, r.Hash}, nil}
	} else {
		ch <- hashback{err: err}
	}
}

// hashWithProgress does a local Hash and returns the hashback result through the given channel,
// hash calculus advances only as progress updates get through progressch
func hashWithProgress(filename string, size int64, progressch chan int64, ch chan hashback) {
	file, err := os.Open(filename) // For read access
	if err != nil {
		ch <- hashback{err: err}
		return
	}
	defer file.Close()
	pos, progressed := int64(0), int64(0)
	h := sha1.New()
	for pos < size {
		progressed = <-progressch
		if progressed == 0 || progressed > size { // process the rest
			progressed = size
		}
		if progressed > pos {
			toread := progressed - pos
			if _, err = io.CopyN(h, file, toread); err != nil {
				ch <- hashback{err: err}
				return
			}
			pos = progressed
		}
	}
	hash := "sha1-" + fmt.Sprintf("%x", h.Sum(nil))
	ch <- hashback{HashInfo{size, 0, size, hash}, nil}
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
// if to==from it short-curcuits and returns (0,nil)
func filecopy(to, from string) (int64, error) {
	if to == from {
		return 0, nil
	}
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
