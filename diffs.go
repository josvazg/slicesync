package slicesync

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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

// calcDiffsFn returns the Diffs between remote filename and local alike or an error
func calcDiffsFn(server, filename, alike string, slice int64) (*Diffs, error)

// defaultDiffBuilder points to the currently activated calcDiffsFn function / algorithm
var CalcDiffs = naiveDiffs

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

// naiveDiffs returns the Diffs between remote filename and local alike or an error
//
// Algorithm:
// 1. Open local and remote bulkHash streams
// 2. Read local and remote file sizes
// 3. Build the diffs: 
//    For each slice hash pairs check if they are different or not 
//    and register and join the different areas in Diffs.Diffs with start Offset and Size
// 4. Read local and remote total file hashes
// 5. Return the Diffs
func naiveDiffs(server, filename, alike string, slice int64) (*Diffs, error) {
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
	if err = diffsBuilder(diffs, local, remote, lsize); err != nil {
		return nil, fmt.Errorf("DiffBuilder error: %v", err)
	}
	if diffs.Size > 0 && len(diffs.Diffs) == 0 {
		return nil, fmt.Errorf("DiffBuilder error: No differences produced to allow file reconstruction!")
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

// diffsBuilder builds the diffs from the hash streams naively, just matching blocks on the same positions
func diffsBuilder(diffs *Diffs, local, remote *bufio.Reader, lsize int64) error {
	indiff := false
	end := min(lsize, diffs.Size)
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
	if lsize < diffs.Size {
		remaining := diffs.Size - lsize
		diffs.Diffs = append(diffs.Diffs, Diff{lsize, remaining, true})
		diffs.Differences += remaining
	}
	return nil
}

// advancedDiffs builds the diffs following a similar strategy as rsync, that is,
// searching for block matches anywhere even on shifted or reshuffled content
func advancedDiffs(server, filename, alike string, slice int64) (*Diffs, error) {
	return nil, fmt.Errorf("Not implemented")
}
