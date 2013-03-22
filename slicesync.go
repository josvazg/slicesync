package slicesync

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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

// readHeader reads the full .slicesync file/stream header checking that all is correct and returning the file size
func readHeader(r *bufio.Reader, filename string, slice int64) (size int64, err error) {
	attrs := []string{"Version", "Filename", "Slice", "Slice Hashing"}
	expectedValues := []interface{}{
		Version,
		filepath.Base(filename),
		fmt.Sprintf("%v", slice),
		newSliceHasher().Name(),
	}
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
