// Hashndump uses Russ's devweb for the development server
package main

import (
	"code.google.com/p/rsc/devweb/slave"
	"github.com/josvazg/slicesync"
)

func main() {
	slicesync.SetupHashNDump()
	slave.Main()
}
