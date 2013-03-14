package slicesync

import (
	"bufio"
	"crypto/sha1"
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
// 1. Prepare the mixerfn
// 2. Run calcDiffs (just like CalcDiffs BUT passing it mixerfn to process each slice)
// 3. That mixerfn is called after each diff slice is processed:
//    It decides to copy that slice from local or remote and updates the generated hash
// 4. The diff remote hash is checked against the local dumped hash
// 5. If all is well the generated diff is returned
//
func Slicesync(server, filename, destfile, alike string, slice int64) (diffs *Diffs, err error) {
	// 1. Prepare mixerfn
	file, err := os.OpenFile(destfile, os.O_CREATE|os.O_WRONLY, 0750) // For write access
	if err != nil {
		return
	}
	defer file.Close()
	h := sha1.New()
	sink := io.MultiWriter(file, h)
	done := int64(0)
	var source io.ReadCloser
	localHnd := &LocalHashNDump{"."}
	remoteHnd := &RemoteHashNDump{server}
	// 3. That mixerfn is called after each diff slice is processed
	//    It decides to copy that slice from local or remote and updates the generated hash
	fn := func(pos int64, indiff bool) (err error) {
		if pos < done {
			return fmt.Errorf("Expected pos>%v but got %v!", done, pos)
		}
		toread := pos - done
		if indiff {
			fmt.Println("Download from", done, " to ", pos)
			source, err = remoteHnd.Dump(filename, done, toread)
		} else {
			fmt.Println("Copy from", done, " to ", pos)
			source, err = localHnd.Dump(alike, done, toread)
		}
		if err != nil {
			return
		}
		io.CopyN(sink, source, toread)
		source.Close()
		file.Sync()
		done = pos
		fmt.Println("Done ", done)
		return nil
	}
	// 2. Run calcDiffs with mixerfn (will call "3. That mixerfn..." inside for each slice)
	diffs, err = calcDiffs(server, filename, alike, slice, fn)
	if source != nil {
		source.Close()
	}
	if err != nil {
		return nil, err
	}
	// 4. The diff remote hash is checked against the local dumped hash
	localHash := fmt.Sprintf("%x", h.Sum(nil))
	if localHash != diffs.Hash {
		return nil, fmt.Errorf("Hash check failed: expected %v but got %v!", diffs.Hash, localHash)
	}
	// 5. If all is well the generated diff is returned
	return diffs, err
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
	// local & remote sizes
	lsize, err := readInt64(local)
	if err != nil {
		return nil, err
	}
	rsize, err := readInt64(remote)
	if err != nil {
		return nil, err
	}
	// diff building loop
	diffs := NewDiffs(server, filename, alike, slice, AUTOSIZE)
	if err = diffLoop(diffs, local, remote, lsize, rsize, fn); err != nil {
		return nil, err
	}
	// total hashes
	diffs.AlikeHash, err = readAttribute(local, "Final")
	if err != nil {
		return nil, err
	}
	diffs.Hash, err = readAttribute(remote, "Final")
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

// readInt64 returns the next int64 or an error
func readInt64(r *bufio.Reader) (int64, error) {
	line, err := readString(r)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(line, 10, 64)
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

// min returns the minimum int64 between a and b
func min(a, b int64) int64 {
	if b < a {
		return b
	}
	return a
}
