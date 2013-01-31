package slicesync_test

import (
	"fmt"
	"github.com/josvazg/slicesync"
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
	filename, content string
	slice             int64
	downloads         int
}{
	{"testfile2.txt", likefile, 10, 3},   // 0
	{"testfile2.txt", likefile, 1000, 1}, // 1
}

func TestSync(t *testing.T) {
	server := fmt.Sprintf("localhost:%v", port)
	writeFile(t, "testfile.txt", testfile)
	go slicesync.HashNDumpServer(port)
	for i, st := range synctests {
		writeFile(t, st.filename, st.content)
		downloads, err := slicesync.Slicesync(server, filename, st.filename, "", st.slice)
		if err != nil {
			t.Fatal(err)
		}
		if downloads != st.downloads {
			t.Fatalf("Test %d: Got %d downloads to sync %s, but %d where expected!\n",
				i, downloads, st.filename, st.downloads)
		}
	}
}
