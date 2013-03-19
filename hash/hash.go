package main

import (
	"flag"
	"fmt"
	"github.com/josvazg/slicesync"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

func toMiB(bytes int64) float64 {
	return float64(bytes) / slicesync.MiB
}

func pct(bytes, total int64) float64 {
	return float64(bytes*100) / float64(total)
}

func usage() {
	fmt.Printf("Usage: %v [-slice size] [filename]\n", os.Args[0])
	fmt.Printf("   or: %v [-slice size] [-r]\n", os.Args[0])
}

func main() {
	var slice int64
	var recursive bool
	if len(os.Args) < 1 {
		usage()
		return
	}
	flag.Int64Var(&slice, "slice", slicesync.MiB, "(Optional) Slice size")
	flag.BoolVar(&recursive, "r", false, "Recursive Bulkhash directory preparation")
	flag.Parse()
	if recursive || len(os.Args) == 1 {
		dir, err := filepath.Abs(".")
		if err != nil {
			fmt.Fprint(os.Stderr, err.Error()+"\n")
		}
		mode := ""
		if recursive {
			mode = " recursively"
		}
		fmt.Printf("Hashing current directory '%s'%s...\n", dir, mode)
		if err := slicesync.HashDir(dir, slice, recursive); err != nil {
			fmt.Fprint(os.Stderr, err.Error()+"\n")
		}
		return
	}
	var err error
	if len(flag.Args()) < 1 {
		usage()
		return
	}
	filename := flag.Args()[0]
	hnd := slicesync.LocalHashNDump{"."}
	if len(os.Args) > 2 {
		fmt.Printf("Hashing %v...\n", filename)
		slice, err = strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil {
			fmt.Fprint(os.Stderr, err.Error()+"\n")
		}
		r, err := hnd.BulkHash(filename, slice)
		if err != nil {
			fmt.Fprint(os.Stderr, err.Error()+"\n")
		}
		io.Copy(os.Stdout, r)
	} else {
		fmt.Printf("Hash dump (.slicesync file) for %v...\n", filename)
		fi, err := hnd.Hash(filename, 0, slicesync.AUTOSIZE)
		if err != nil {
			fmt.Fprint(os.Stderr, err.Error()+"\n")
			return
		}
		fmt.Println(fi.Hash)
	}
}
