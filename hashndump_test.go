package slicesync_test

import (
	"bytes"
	"github.com/josvazg/slicesync"
	"io"
	"io/ioutil"
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
}{
	{"testfile.txt", 0, 0, testfile, "sha1-6e1eb4d4daf850c250bdc9a16669c7f66915f842"},
	{"testfile.txt", 0, 10, "AAAAAAAAA\n", "adler32+md5-0dca0254f252b28c22d0bb68caf870df063b6064"},
	{"testfile.txt", 10, 10, "BBBBBBBBB\n", "adler32+md5-0e00025d961310d0926542e45d7190a22d68b48c"},
}

func TestSlices(t *testing.T) {
	hnd := &slicesync.LocalHashNDump{Dir: "."}
	err := ioutil.WriteFile("testfile.txt", ([]byte)(testfile), 0750)
	if err != nil {
		t.Fatal(err)
	}
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
			t.Fatalf("Test #%d failed: expected hash %s but got %s\n",
				i, test.goodhash, fi.Hash)
		}
	}
}
