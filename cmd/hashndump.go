// devsrv runs the hashndump server
package main

import (
	"github.com/josvazg/slicesync"
)

func main() {
	slicesync.HashNDumpServer(8080)
}