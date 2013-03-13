package main

import (
	"fmt"
	"github.com/josvazg/slicesync"
	"os"
	"strconv"
)

func toMiB(bytes int64) float64 {
	return float64(bytes) / slicesync.MiB
}

func pct(bytes, total int64) float64 {
	return float64(bytes*100) / float64(total)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %v {filename} [slice]\n", os.Args[0])
		return
	}
	var err error
	filename := os.Args[1]
	slice := int64(slicesync.AUTOSIZE)
	hnd := slicesync.LocalHashNDump{"."}
	if len(os.Args) > 2 {
		slice, err = strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil {
			fmt.Fprint(os.Stderr, err.Error()+"\n")
		}
		hnd.BulkHash(os.Stdout, filename, slice)
	} else {		
		fi, err := hnd.Hash(filename, 0, slicesync.AUTOSIZE)
		if err != nil {
			fmt.Fprint(os.Stderr, err.Error()+"\n")
			return
		}
		fmt.Println(fi.Hash)
	}
}
