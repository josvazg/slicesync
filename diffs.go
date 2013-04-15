package slicesync

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	MAXBUF = 1024
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

// calcDiffsFunc returns the Diffs between remote filename and local alike or an error
func calcDiffsFunc(server, filename, alike string, slice int64) (*Diffs, error)

// DefaultCalcDiffs points to the currently activated calcDiffsFunc function / algorithm
var DefaultCalcDiffs = NaiveDiffs

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

// CalcDiffs calcs the differences between a remote fileurl and a local alike file
func CalcDiffs(fileurl, alike string, slice int64) (*Diffs, error) {
	if fileurl == "" {
		return nil, fmt.Errorf("Invalid empty URL!")
	}
	server, filename, err := Probe(fileurl)
	if err != nil {
		return nil, err
	}
	return DefaultCalcDiffs(server, filename, alike, slice)
}

// NaiveDiffs returns the Diffs between remote filename and local alike or an error
//
// Algorithm:
// 1. Open local and remote Hash streams
// 2. Read local and remote file sizes
// 3. Build the diffs: 
//    For each slice hash pairs check if they are different or not 
//    and register and join the different areas in Diffs.Diffs with start Offset and Size
// 4. Read local and remote total file hashes
// 5. Return the Diffs
func NaiveDiffs(server, filename, alike string, slice int64) (*Diffs, error) {
	// local & remote streams opening
	localHnd := &LocalHashNDump{"."}
	lc, err := localHnd.Hash(alike)
	if err != nil {
		return nil, fmt.Errorf("Error opening local diff source: %v", err)
	}
	defer lc.Close()
	local := bufio.NewReader(lc)
	remoteHnd := &RemoteHashNDump{server}
	rm, err := remoteHnd.Hash(filename)
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
	hname := NewHasher().Name()
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

// AdvancedDiffs builds the diffs following a similar strategy as rsync, that is,
// searching for block matches anywhere even on shifted or reshuffled content
func AdvancedDiffs(server, filename, alike string, slice int64) (*Diffs, error) {
	fmt.Println("rhnd")
	remoteHnd := newHashNDumper(server)
	rm, err := remoteHnd.Hash(filename)
	if err != nil {
		return nil, fmt.Errorf("Error opening remote diff source: %v", err)
	}
	defer rm.Close()
	remote := bufio.NewReader(rm)
	fmt.Println("header")
	rsize, err := readHeader(remote, filename, slice)
	if err != nil {
		return nil, fmt.Errorf("Remote diff source header error: %v", err)
	}
	fmt.Print("diff")
	// diff creation
	diffs := NewDiffs(server, filename, alike, slice, rsize)
	// Open the local file
	local, err := os.Open(diffs.Alike)
	if err != nil {
		return nil, fmt.Errorf("Error opening local alike: %v", err)
	}
	// loading the moving hash window
	fmt.Println("buffered")
	blocal := bufio.NewReaderSize(local, int(slice+MAXBUF))
	firstBlock, err := blocal.Peek(int(slice))
	if err != nil {
		return nil, fmt.Errorf("Error loading the local hash window: %v", err)
	}
	// Preparing hash sink
	wh := NewRollingAdler32()
	wh.Write(firstBlock)
	pos := slice
	fmt.Println("local read...")
	t := time.Now()
	for ; pos < diffs.Size; pos++ {
		oldb, newb, err := slide(blocal, int(slice))
		if err != nil {
			return nil, fmt.Errorf("Error sliding the local hash window at %v: %v", pos, err)
		}
		wh.Roll32(uint32(slice), oldb, newb)
		if pos%(MiB*250) == 0 {
			if pos > 0 {
				d := time.Since(t)
				rel := time.Duration(diffs.Size / pos)
				estimated := d * rel
				//limit := (3 * time.Minute) / 2
				/*if estimated > limit {
					return nil, fmt.Errorf("too slow, estimated %v time is well over %v", estimated, limit)
				}*/
				fmt.Println(pos/MiB, estimated)
			} else {
				fmt.Println(pos / MiB)
			}
		}
	}
	fmt.Println(pos / MiB)
	return diffs, nil
}

// slide 
func slide(r *bufio.Reader, slice int) (oldb, newb byte, err error) {
	oldb, err = r.ReadByte()
	if err != nil {
		return
	}
	block, err := r.Peek(slice)
	if err != nil {
		return
	}
	newb = block[slice-1]
	return
}

// readHeader reads the full .slicesync file/stream header checking that all is correct and returning the file size
func readHeader(r *bufio.Reader, filename string, slice int64) (size int64, err error) {
	attrs := []string{"Version", "Filename", "Slice", "Slice Hashing"}
	expectedValues := []interface{}{
		Version,
		filepath.Base(filename),
		fmt.Sprintf("%v", slice),
		NewSliceHasher().Name(),
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

func newHashNDumper(pathOrServer string) HashNDumper {
	if pathOrServer == "." || strings.HasPrefix(pathOrServer, "/") {
		fmt.Println("Local at", pathOrServer)
		return &LocalHashNDump{pathOrServer}
	}
	fmt.Println("Remote at", pathOrServer)
	return &RemoteHashNDump{pathOrServer}
}
