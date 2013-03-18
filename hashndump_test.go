package slicesync_test

import (
	"bytes"
	"github.com/josvazg/slicesync"
	"io"
	"os"
	"testing"
)

const (
	testfile = `AAAAAAAAA
BBBBBBBBB
CCCCCCCCC
DDDDDDDDD
EEEEEEEEE
AAAAAAAAA
`
)

var tests = []struct {
	filename   string
	start, len int64
	expected   string
	goodhash   string
}{{"testfile.txt", 0, 0, testfile, "sha1-6e1eb4d4daf850c250bdc9a16669c7f66915f842"},
	{"testfile.txt", 0, 10, "AAAAAAAAA\n", "sha1-bf6492720d4179ce7d10d82f80b6ec61d871177d"},
	{"testfile.txt", 10, 10, "BBBBBBBBB\n", "sha1-4c2589d96f40deefe9b6faa049e96488361fad9d"},
}

func TestSlices(t *testing.T) {
	hnd := &slicesync.LocalHashNDump{Dir: "."}
	writeFile(t, "testfile.txt", testfile)
	for i, test := range tests {
		dmp, _, err := hnd.Dump(test.filename, test.start, test.len)
		if err != nil {
			t.Fatal(err)
		}
		buf := bytes.NewBufferString("")
		io.Copy(buf, dmp)
		str := buf.String()
		if str != test.expected {
			t.Fatalf("Test #%d failed: expected '%s' but got '%s'\n%v\n",
				i, test.expected, str, test)
		}
		fi, err := hnd.Hash(test.filename, test.start, test.len)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Hash != test.goodhash {
			t.Fatalf("Test #%d failed: expected SHA1 hash %s but got %s\n",
				i, test.goodhash, fi.Hash)
		}
	}
}

func writeFile(t *testing.T, filename, content string) {
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	n, err := io.WriteString(file, content)
	if err != nil || n != len(content) {
		t.Fatal(err)
	}
}
