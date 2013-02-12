// devsrv runs the hashndump server
package main

import (
	"flag"
	"fmt"
	"github.com/josvazg/slicesync"
)
// Start the server on port 8000 by default
func main() {
	var port int
	flag.IntVar(&port,"port",8000,"Server port to listen from")
    flag.Parse()
	fmt.Printf("slicesync server (Hash&Dump) listening at %v...\n", port)
	slicesync.HashNDumpServer(port)
}
