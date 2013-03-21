package slicesync_test

import (
	"fmt"
	"github.com/josvazg/slicesync"
	"hash/adler32"
	"io/ioutil"
	"strings"
	"testing"
)

const (
	Window = 16
)

func TestRolling(t *testing.T) {
	testdata, err := genTestData()
	dieOnError(t, err)
	if len(testdata) < Window {
		t.Fatal(fmt.Errorf("Datasize expected >%vbytes but got only %vbytes! (write more go code! ;-)",
			Window, len(testdata)))
	}
	a32 := adler32.New()
	ra32 := slicesync.NewRollingAdler32()
	// First load the window
	a32.Write(testdata[0:Window])
	ra32.Write(testdata[0:Window])
	if a32.Sum32() != ra32.Sum32() {
		t.Fatal(fmt.Errorf("Initial checksum expected was %x but got %x instead!", a32.Sum32(), ra32.Sum32()))
	}
	// Now roll-it and compare with full re-calculus at each position
	for i := 1; i < len(testdata)-Window; i++ {
		a32.Reset()
		a := testdata[i : i+Window]
		a32.Write(a)
		rolled := ra32.Roll32(Window, testdata[i-1], a[len(a)-1])
		if a32.Sum32() != rolled {
			t.Fatal(fmt.Errorf("Checksum at position %d expected was 0x%08x but got 0x%08x instead! a=%v old=%v",
				i, a32.Sum32(), rolled, a, testdata[i-1]))
		}
	}
}

func dieOnError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func genTestData() (data []byte, err error) {
	finfos, err := ioutil.ReadDir(".")
	if err != nil {
		return
	}
	datasize := int64(0)
	for _, fi := range finfos {
		if strings.HasSuffix(fi.Name(), ".go") {
			datasize += fi.Size()
		}
	}
	data = make([]byte, 0, datasize) // single allocation
	for _, fi := range finfos {
		if strings.HasSuffix(fi.Name(), ".go") {
			filedata, err := ioutil.ReadFile(fi.Name())
			if err != nil {
				return nil, err
			}
			data = append(data, filedata...)
		}
	}
	return
}
