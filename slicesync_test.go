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
	slice    = 10
)

var synctests = []struct {
	filename, content string
}{
	{"testfile2.txt", likefile},
}

func TestSync(t *testing.T) {
	server := fmt.Sprintf("localhost:%v", port)
	writeFile(t, "testfile.txt", testfile)
	go slicesync.HashNDumpServer(port)
	for _, st := range synctests {
		writeFile(t, st.filename, st.content)
		slicesync.Slicesync(server, filename, st.filename, "", slice)
	}
}
