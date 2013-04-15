package main

import (
	"flag"
	"fmt"
	"github.com/josvazg/slicesync"
	"io"
	"os"
	"path/filepath"
	"time"
)

// toMiB translates bytes into MiBytes
func toMiB(bytes int64) float64 {
	return float64(bytes) / slicesync.MiB
}

// pct calculates a percentage from bytes vs total
func pct(bytes, total int64) float64 {
	return float64(bytes*100) / float64(total)
}

// usage displays command usage information
func usage() {
	fmt.Printf("Usage: %v filename\n", os.Args[0])
	fmt.Printf("   or: %v [-bulkhash filename]\n", os.Args[0])
	fmt.Printf("   or: %v [-dir directory] [-slice size] [-service] [-r]\n", os.Args[0])
	fmt.Printf("   or: %v [-help]\n\n", os.Args[0])
	flag.PrintDefaults()
}

// mode returns a translation of recursive into text
func mode(recursive bool) string {
	if recursive {
		return " recursive"
	}
	return ""
}

// exitOnError displays an error and exits if err is not null
func exitOnError(err error) {
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		os.Exit(-1)
	}
}

// hashOnce hashes a directory once and finishes
func hashOnce(dir string, slice int64, recursive bool) {
	dir, err := filepath.Abs(dir)
	exitOnError(err)
	fmt.Printf("Hashing directory '%s'%s...\n", dir, mode(recursive))
	if err := slicesync.HashDir(dir, slice, recursive); err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
	}
}

// hashingService watches and re-hashes a directory every second or so
func hashingService(dir string, slice int64, recursive bool) {
	dir, err := filepath.Abs(dir)
	exitOnError(err)
	fmt.Printf("Watching and Hashing directory '%s'%s...\n", dir, mode(recursive))
	for {
		if err := slicesync.HashDir(dir, slice, recursive); err != nil {
			fmt.Fprint(os.Stderr, err.Error()+"\n")
		}
		time.Sleep(1 * time.Second)
	}
}

// hashAFile calculates the whole file hash and displays it, just like shasum
func hashAFile(filename string) {
	fmt.Printf("Hashing %v...\n", filename)
	hnd := slicesync.LocalHashNDump{"."}
	hi, err := hnd.Hash(filename, 0, slicesync.AUTOSIZE)
	exitOnError(err)
	fmt.Println(hi.Hash)
}

// bulkHash produces the .slicesync bulkhash dump for the given filename
func bulkHash(filename string, slice int64) {
	fmt.Printf("Hash dump (.slicesync file) for %v...\n", filename)
	hnd := slicesync.LocalHashNDump{"."}
	r, err := hnd.BulkHash(filename, slice)
	exitOnError(err)
	io.Copy(os.Stdout, r)
}

func main() {
	var slice int64
	var recursive bool
	var service bool
	var bulkhash string
	var dir string
	var help bool
	flag.Int64Var(&slice, "slice", slicesync.MiB, "(Optional) Slice size")
	flag.BoolVar(&recursive, "r", false, "Recursive Bulkhash directory preparation")
	flag.BoolVar(&service, "service", false, "Service process to repeatedly prepare Bulkhash on this directory")
	flag.StringVar(&bulkhash, "bulkhash", "", "Generate a bulkhash dump of the given file")
	flag.StringVar(&dir, "dir", ".", "Directory base of generated bulkhashes")
	flag.BoolVar(&help, "help", false, "Show command help")
	flag.Parse()
	if help {
		usage()
	} else if service {
		hashingService(dir, slice, recursive)
	} else if recursive || len(flag.Args()) == 0 {
		hashOnce(dir, slice, recursive)
	} else if bulkhash != "" {
		bulkHash(bulkhash, slice)
	} else {
		hashAFile(flag.Args()[0])
	}
}
