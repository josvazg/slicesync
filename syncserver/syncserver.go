// devsrv runs the hashndump server
package main

import (
	"fmt"
	"github.com/josvazg/slicesync"
	"os"
	"strconv"
)

// Start the server on port 8000 by default
func main() {
	port := 8000
	dir := "."
	if len(os.Args) > 1 {
		if os.Args[1] == "--help" {
			usage()
			return
		}
		var err error
		port, err = strconv.Atoi(os.Args[1])
		if err != nil {
			fmt.Printf("First argument must be '--help' or a port number but got %v!\n", os.Args[1])
			usage()
			return
		}
	}
	if len(os.Args) > 2 {
		dir = os.Args[2]
	}
	slice := int64(slicesync.MiB)
	if len(os.Args) > 3 {
		slc, err := strconv.ParseInt(os.Args[3], 10, 64)
		if err != nil {
			fmt.Println(err)
			return
		}
		slice = slc
	}
	recursive := true
	if len(os.Args) > 4 {
		recursive = !(os.Args[4] == "non-recursive")
	}
	fmt.Printf("slicesync server (Hash&Dump) listening at %v and serving %s...\n", port, dir)
	go slicesync.HashDir(".", slice, recursive)
	slicesync.ServeHashNDump(port, dir, "")
}

func usage() {
	fmt.Printf("Usage: %v [port] [dir] [slice] [-non-recursive] (or --help for this help)\n", os.Args[0])
}
