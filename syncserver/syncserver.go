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
	port:=8000
	dir:="."
	if len(os.Args)>1 {
		if os.Args[1]=="--help" {
			usage()
			return
		}
		var err error
		port,err=strconv.Atoi(os.Args[1])
		if err!=nil {
			fmt.Println("First argument must be '--help' or a port number but got %v!",os.Args[1])
			usage()
			return 
		}
	}
	if len(os.Args)>2 {
		dir=os.Args[2]
	}
	fmt.Printf("slicesync server (Hash&Dump) listening at %v and serving %s...\n", port, dir)
	slicesync.HashNDumpServer(port, dir)
}

func usage() {
	fmt.Println("Usage: %v [port] [dir] (or --help for this help)\n",os.Args[0])
}