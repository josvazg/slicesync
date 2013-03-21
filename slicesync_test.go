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
	filename = "testfile.txt"
	port     = 8000
)

var synctests = []struct {
	filename, content  string
	slice, differences int64
}{
	{"testfile2.txt", likefile, 10, 30},   // 0
	{"testfile2.txt", likefile, 1000, 60}, // 1
}

func TestSync(t *testing.T) {
	server := fmt.Sprintf("localhost:%v", port)
	err := ioutil.WriteFile("testfile.txt", ([]byte)(testfile), 0750)
	if err != nil {
		t.Fatal(err)
	}
	go slicesync.HashNDumpServer(port, ".")
	for i, st := range synctests {
		err := ioutil.WriteFile(st.filename, ([]byte)(st.content), 0750)
		if err != nil {
			t.Fatal(err)
		}
		diffs, err := slicesync.Slicesync(server, filename, st.filename, "", st.slice)
		if err != nil {
			t.Fatal(err)
		}
		if diffs.Differences != st.differences {
			t.Fatalf("Test %d: Expected %d differences to sync %s, but got %d!\n",
				i, st.differences, st.filename, diffs.Differences)
		}
	}
}
