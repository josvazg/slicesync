package slicesync

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Diff specifies a different or equal data segment of size Size
// Offset marks the starting position on the remote or local file for this segment
type Diff struct {
	Offset, Size int64
	Different    bool
}

// Diffs list the differences between two similar files, a remote filename and a local alike
type Diffs struct {
	Server, Filename, Alike  string
	Slice, Size, Differences int64
	Diffs                    []Diff
	Hash, AlikeHash          string
}

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

// Print shows the Diffs ina Pretty-Printed json format
func (sd *Diffs) Print() string {
	dst := bytes.NewBufferString("")
	if err := json.Indent(dst, ([]byte)(sd.String()), "", "  "); err != nil {
		return err.Error()
	}
	return dst.String()
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
	// local & remote streams opening
	localHnd := &LocalHashNDump{"."}
	lc, err := localHnd.BulkHash(alike, slice)
	if err != nil {
		return nil, fmt.Errorf("Error opening local diff source: %v", err)
	}
	defer lc.Close()
	local := bufio.NewReader(lc)
	remoteHnd := &RemoteHashNDump{server}
	rm, err := remoteHnd.BulkHash(filename, slice)
	if err != nil {
		return nil, fmt.Errorf("Error opening remote diff source: %v", err)
	}
	defer rm.Close()
	remote := bufio.NewReader(rm)
	// local & remote headers
	lsize, err := readHeader(local, alike, slice)
	if err != nil {
		return nil, fmt.Errorf("Local diff source header error: %v", err)
	}
	rsize, err := readHeader(remote, filename, slice)
	if err != nil {
		return nil, fmt.Errorf("Remote diff source header error: %v", err)
	}
	// diff building loop
	diffs := NewDiffs(server, filename, alike, slice, rsize)
	if err = diffLoop(diffs, local, remote, lsize, rsize); err != nil {
		return nil, fmt.Errorf("Diff loop error: %v", err)
	}
	// total hashes
	hname := newHasher().Name()
	diffs.AlikeHash, err = readAttribute(local, hname)
	if err != nil {
		return nil, fmt.Errorf("Local file hash error: %v", err)
	}
	diffs.Hash, err = readAttribute(remote, hname)
	if err != nil {
		return nil, fmt.Errorf("Remote file hash error: %v", err)
	}
	return diffs, nil
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
// 1. CalcDiffs
// 2. DownloadDiffs
// 3. Check local & remote hash
// 4. If all is well the generated diff is returned
//
func Slicesync(server, filename, destfile, alike string, slice int64) (diffs *Diffs, err error) {
	if destfile == "" {
		destfile = filename
	}
	if alike == "" {
		alike = destfile
	}
	// 0. Bypass process and Download directly if there is no alike file
	if !exists(alike) {
		downloaded, err := Download(destfile, "http://"+server+"/dump/"+filename)
		if err != nil {
			return nil, fmt.Errorf("Direct Download error: %v", err)
		}
		diffs := NewDiffs(server, filename, "", slice, downloaded)
		diffs.Differences = downloaded
		return diffs, nil
	}
	// 1. CalcDiffs
	diffs, err = CalcDiffs(server, filename, alike, slice)
	if err != nil {
		return nil, fmt.Errorf("Error calculating differences: %v", err)
	}
	// 2. DownloadDiffs
	_, localHash, err := DownloadDiffs(destfile, diffs)
	if err != nil {
		return nil, fmt.Errorf("Download error: %v", err)
	}
	// 3. Check hashes
	if localHash != diffs.Hash {
		return nil, fmt.Errorf("Hash check failed: expected %v but got %v!", diffs.Hash, localHash)
	}
	// 4. If all is well the generated diff is returned
	return diffs, err
}

// DownloadDiffs downloads a filename by differences into destfile
func DownloadDiffs(destfile string, diffs *Diffs) (downloaded int64, hash string, err error) {
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
	for _, diff := range diffs.Diffs {
		if diff.Different {
			source, _, err = remoteHnd.Dump(diffs.Filename, diff.Offset, diff.Size)
		} else {
			source, _, err = localHnd.Dump(diffs.Alike, diff.Offset, diff.Size)
		}
		if err != nil {
			return downloaded, "", err
		}
		n, err := io.CopyN(sink, source, diff.Size)
		source.Close()
		if err != nil {
			return downloaded, "", err
		}
		if n != diff.Size {
			return downloaded, "", fmt.Errorf("Expected to copy %v but copied %v instead!", diff.Size, n)
		}
		downloaded += n
		done += n
	}
	return downloaded, fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Download simply downloads a URL to destfile (no hash calculus is done or returned)
func Download(destfile, url string) (downloaded int64, err error) {
	r, _, err := open(url)
	if err != nil {
		return
	}
	defer r.Close()
	w, err := os.OpenFile(destfile, os.O_CREATE|os.O_WRONLY, 0750) // For write access
	if err != nil {
		return
	}
	defer w.Close()
	return io.Copy(w, r)
}

// diffLoop builds the diffs from the hash streams
func diffLoop(diffs *Diffs, local, remote *bufio.Reader, lsize, rsize int64) error {
	indiff := false
	end := min(lsize, rsize)
	segment := diffs.Slice
	start := int64(0)
	pos := int64(0)
	for ; pos < end; pos += segment {
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
		if !indiff && localHash != remoteHash { // diff starts
			if len(diffs.Diffs) == 0 && pos > 0 { // special Diffs array init case
				diffs.Diffs = append(diffs.Diffs, Diff{start, 0, false})
			}
			if len(diffs.Diffs) > 0 { // only if there is a non-different segment before...
				size := pos - start
				diffs.Diffs[len(diffs.Diffs)-1].Size = size
			}
			start = pos
			diffs.Diffs = append(diffs.Diffs, Diff{start, 0, true})
			diffs.Differences += segment
			indiff = true
		} else if indiff && localHash == remoteHash { // diffs ends
			size := pos - start
			diffs.Diffs[len(diffs.Diffs)-1].Size = size
			diffs.Differences += size
			start = pos
			diffs.Diffs = append(diffs.Diffs, Diff{start, 0, false})
			indiff = false
		}
	}
	if len(diffs.Diffs) == 0 {
		diffs.Diffs = append(diffs.Diffs, Diff{0, pos, false})
	} else {
		diffs.Diffs[len(diffs.Diffs)-1].Size = pos - start
	}
	if lsize < rsize {
		remaining := rsize - lsize
		diffs.Diffs = append(diffs.Diffs, Diff{lsize, remaining, true})
		diffs.Differences += remaining
	}
	return nil
}

// readHeader reads the full .slicesync file/stream header checking that all is correct and returning the file size
func readHeader(r *bufio.Reader, filename string, slice int64) (size int64, err error) {
	attrs := []string{"Version", "Filename", "Slice", "Slice Hashing"}
	expectedValues := []interface{}{Version, filepath.Base(filename), fmt.Sprintf("%v", slice), newHasher().Name()}
	for n, attr := range attrs {
		val, err := readAttribute(r, attr)
		if err != nil {
			return 0, err
		}
		if val != expectedValues[n] {
			return 0, fmt.Errorf("%s mismatch: Expecting %s but got %s!", attr, expectedValues[n], val)
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

// readInt64Attribute reads an int64 attribute from the .slicesync text header
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

// Does the file exist?
func exists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	}
	return false
}
