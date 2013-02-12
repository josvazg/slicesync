// devsrv runs the hashndump server
package main

import (
	"fmt"
	"github.com/josvazg/slicesync"
)

func main() {
	port:=8080
	fmt.Printf("slicesync server (Hash&Dump) listening at %v...\n", port)
	slicesync.HashNDumpServer(port)
}