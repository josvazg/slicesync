package slicesync

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Diff marks a start offset and a size in bytes
type Diff struct {
	Offset, Size int64
}

// Diffs list the differences between two similar files, a remote filename and a local alike
type Diffs struct {
	Server, Filename, Alike  string
	Slice, Size, Differences int64
	Diffs                    []Diff
	Hash, AlikeHash          string
}

// mixerfn can mix local and remote data into the destfile sink
type mixerfn func(pos int64, indiff bool) (err error)

// NewDiffs creates a Diffs data type
func NewDiffs(server, filename, alike string, slice, size int64) *Diffs {
	return &Diffs{server, filename, alike, slice, size, 0, make([]Diff, 0, 10), "", ""}
}

// String shows the diffs in a json representation
func (sd *Diffs) String() string {
	bytes, err := json.Marshal(sd)
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}

// CalcDiffs returns the Diffs between remote filename and local alike or an error
//
// Algorithm:
// 1. Open local and remote bulkHash streams
// 2. Read local and remote file sizes
// 3. Build the diffs: 
//    For each slice hash pairs check if they are different or not 
//    and register and join the different areas in Diffs.Diffs with start Offset and Size
// 4. Read local and remote total file hashes
// 5. Return the Diffs
func CalcDiffs(server, filename, alike string, slice int64) (*Diffs, error) {
	return calcDiffs(server, filename, alike, slice, nil)
}

// Slicesync copies remote filename from server to local destfile, 
// using as much of local alike as possible
//
// server is the url of the server holding filename
// filename is the remote file to download
// destfile is the local destination, same as filename if empty
// alike is the local alike file to compare with and save downloads, same as destfile if empty
// slice is the size of each of the slices to sync
//
// Algorithm:
// 1. Run CalcDiffs
// 2. Download diffs
// 3. Check local & remote hash
// 4. If all is well the generated diff is returned
//
func Slicesync(server, filename, destfile, alike string, slice int64) (diffs *Diffs, err error) {
	// 1. CalcDiffs
	diffs, err = CalcDiffs(server, filename, alike, slice)
	if err != nil {
		return nil, err
	}
	// 2. Download
	_, localHash, err := Download(destfile, diffs)
	if err != nil {
		return nil, err
	}
	// 3. Check hashes
	if localHash != diffs.Hash {
		return nil, fmt.Errorf("Hash check failed: expected %v but got %v!", diffs.Hash, localHash)
	}
	// 4. If all is well the generated diff is returned
	return diffs, err
}

// Download a filename by differences into destfile
func Download(destfile string, diffs *Diffs) (downloaded int64, hash string, err error) {
	// 1. Prepare mixerfn
	file, err := os.OpenFile(destfile, os.O_CREATE|os.O_WRONLY, 0750) // For write access
	if err != nil {
		return
	}
	defer file.Close()
	h := newHasher()
	var source io.ReadCloser
	sink := io.MultiWriter(file, h)
	localHnd := &LocalHashNDump{"."}
	remoteHnd := &RemoteHashNDump{diffs.Server}
	done := int64(0)
	copyn := func(hnd HashNDumper, fromfile string, pos, toread int64) (int64, error) {
		source, _, err = hnd.Dump(fromfile, pos, toread)
		if err != nil {
			return downloaded, err
		}
		defer source.Close()
		return io.CopyN(sink, source, toread)
	}
	for _, diff := range diffs.Diffs {
		if diff.Offset > done {
			_, err := copyn(localHnd, diffs.Alike, done, diff.Offset-done)
			if err != nil {
				return downloaded, "", err
			}
			done = diff.Offset
		}
		n, err := copyn(remoteHnd, diffs.Filename, done, diff.Size)
		if err != nil {
			return downloaded, "", err
		}
		downloaded += n
		done += n
	}
	if diffs.Size > done {
		n, err := copyn(localHnd, diffs.Alike, done, diffs.Size-done)
		if err != nil {
			return downloaded, "", err
		}
		done += n
	}
	return downloaded, fmt.Sprintf("%x", h.Sum(nil)), nil
}

// calcDiffs returns the Diffs as specified by CalcDiffs but it also admits a mixer
// when a non-nil mixer is passed to calcDiffs it is used to post-process each evaluated slice
func calcDiffs(server, filename, alike string, slice int64, fn mixerfn) (*Diffs, error) {
	// local & remote streams opening
	localHnd := &LocalHashNDump{"."}
	lc, err := localHnd.BulkHash(alike, slice)
	if err != nil {
		return nil, err
	}
	defer lc.Close()
	local := bufio.NewReader(lc)
	remoteHnd := &RemoteHashNDump{server}
	rm, err := remoteHnd.BulkHash(filename, slice)
	if err != nil {
		return nil, err
	}
	defer rm.Close()
	remote := bufio.NewReader(rm)
	// local & remote headers
	lsize, err := readHeader(local, alike, slice)
	if err != nil {
		return nil, err
	}
	rsize, err := readHeader(remote, filename, slice)
	if err != nil {
		return nil, err
	}
	// diff building loop
	diffs := NewDiffs(server, filename, alike, slice, AUTOSIZE)
	if err = diffLoop(diffs, local, remote, lsize, rsize, fn); err != nil {
		return nil, err
	}
	// total hashes
	diffs.AlikeHash, err = readAttribute(local, hasherName())
	if err != nil {
		return nil, err
	}
	diffs.Hash, err = readAttribute(remote, hasherName())
	if err != nil {
		return nil, err
	}
	return diffs, nil
}

// diffLoop builds the diffs from the hash streams
func diffLoop(diffs *Diffs, local, remote *bufio.Reader, lsize, rsize int64, fn mixerfn) error {
	indiff := false
	end := min(lsize, rsize)
	segment := diffs.Slice
	for pos := int64(0); pos < end; pos += segment {
		if pos+segment > end {
			segment = end - pos
		}
		localHash, err := readString(local)
		if err != nil {
			return err
		}
		remoteHash, err := readString(remote)
		if err != nil {
			return err
		}
		if indiff {
			if localHash == remoteHash {
				indiff = false
			} else {
				diffs.Diffs[len(diffs.Diffs)-1].Size += segment
				diffs.Differences += segment
			}
		} else if localHash != remoteHash {
			diffs.Diffs = append(diffs.Diffs, Diff{pos, segment})
			diffs.Differences += segment
			indiff = true
		}
		if fn != nil {
			fn(pos+segment, indiff)
		}
	}
	if lsize < rsize {
		remaining := rsize - lsize
		diffs.Diffs = append(diffs.Diffs, Diff{lsize, remaining})
		diffs.Differences += remaining
	}
	return nil
}

// readHeader reads the full .slicesync file/stream header checking that all is correct and returning the file size
func readHeader(r *bufio.Reader, filename string, slice int64) (size int64, err error) {
	attrs := []string{"Version", "Filename", "Slice"}
	expectedValues := []interface{}{Version, filename, fmt.Sprintf("%v", slice)}
	for n, attr := range attrs {
		val, err := readAttribute(r, attr)
		if err != nil {
			return 0, err
		}
		if val != expectedValues[n] {
			return 0, fmt.Errorf("%s mismacth: Expecting %s but got %s!", attr, expectedValues[n], val)
		}
	}
	return readInt64Attribute(r, "Length")
}

// readString returns the next string or an error
func readString(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(line, "Error:") {
		return "", fmt.Errorf(line)
	}
	return strings.Trim(line, " \n"), nil
}

// readAttribute returns the next attribute named name or an error
func readAttribute(r *bufio.Reader, name string) (string, error) {
	data, err := readString(r)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(data, name+":") {
		return "", fmt.Errorf(name+": expected, but got %s!", data)
	}
	return strings.Trim(data[len(name)+1:], " \n"), nil
}

// readInt64Attribute reads an int64 attritubte from the .slicesync text header
func readInt64Attribute(r *bufio.Reader, name string) (int64, error) {
	line, err := readAttribute(r, name)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(line, 10, 64)
}

// min returns the minimum int64 between a and b
func min(a, b int64) int64 {
	if b < a {
		return b
	}
	return a
}
