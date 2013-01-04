package slicesync_test

import (
	"fmt"
	"github.com/josvazg/slicesync"
	"io"
	"os"
	"testing"
)

var tests = []struct {
	filename   string
	start, len int64
}{{"hashndump.go", 0, 0},
	{"hashndump.go", 0, 10},
	{"hashndump.go", 10, 10},
}

func TestSlices(t *testing.T) {
	for i := 0; i < len(tests); i++ {
		hsh, err := slicesync.Hash(tests[i].filename, tests[i].start, tests[i].len)
		if err != nil {
			t.Fatal(err)
		}
		dmp, _, err := slicesync.Dump(tests[i].filename, tests[i].start, tests[i].len)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("\n==========================================")
		io.Copy(os.Stdout, dmp)
		fmt.Println("\n==========================================")
		fmt.Printf("\nHash for %s[%d:%d] = %s\n",
			tests[i].filename, tests[i].start, tests[i].len, hsh)
	}
}
