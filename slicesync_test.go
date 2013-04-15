package slicesync_test

import (
	"fmt"
	"github.com/josvazg/slicesync"
	"io/ioutil"
	"testing"
)

const (
	likefile = `AAAAAAAAA
BBBBBBBBB
CCCCCcCCC
DDDDDDDDD
EEEeEEEEE
AAAAAAAaA
`
	host      = "localhost"
	port      = 8000
	probeFile = "a/b/c/file.txt"
)

var probetests = []struct {
	base, path string
}{
	{"a/b", "c/file.txt"},
	{"", "a/b/c/file.txt"},
	{"a", "b/c/file.txt"},
	{"a/b/c", "file.txt"},
}

func TestProbe(t *testing.T) {
	slicesync.HashDir(".", slicesync.MiB, false)
	p := port + 1
	for i, pt := range probetests {
		probeUrl := fmt.Sprintf("%v:%v/%v", host, p+i, probeFile)
		go slicesync.ServeHashNDump(p+i, ".", pt.base)
		server, file, e := slicesync.Probe(probeUrl)
		if e != nil {
			t.Fatal(e)
		}
		if file != pt.path {
			t.Fatalf("Test %d: Expected path %v but got %v (server=%v)!\n", i, pt.path, file, server)
		}
	}
}

var synctests = []struct {
	filename, content  string
	slice, differences int64
}{
	{"testfile2.txt", likefile, 10, 30},   // 0
	{"testfile2.txt", likefile, 1000, 60}, // 1
}

func TestSync(t *testing.T) {
	err := ioutil.WriteFile("testfile.txt", ([]byte)(testfile), 0750)
	if err != nil {
		t.Fatal(err)
	}
	go slicesync.ServeHashNDump(port, ".", "")
	url := fmt.Sprintf("%v:%v/%v", host, port, "testfile.txt")
	for i, st := range synctests {
		err := ioutil.WriteFile(st.filename, ([]byte)(st.content), 0750)
		if err != nil {
			t.Fatal(err)
		}
		// prepares diffs to be served AFTER the file were created
		err = slicesync.HashFile(".", "testfile.txt", st.slice)
		if err != nil {
			t.Fatal(err)
		}
		err = slicesync.HashFile(".", st.filename, st.slice)
		if err != nil {
			t.Fatal(err)
		}
		diffs, err := slicesync.Slicesync(url, st.filename, "", st.slice)
		if err != nil {
			t.Fatal(err)
		}
		if diffs.Differences != st.differences {
			t.Fatalf("Test %d: Expected %d differences to sync %s, but got %d!\n",
				i, st.differences, st.filename, diffs.Differences)
		}

	}
}
