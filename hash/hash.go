package main

import (
	"fmt"
	"github.com/josvazg/slicesync"
	"os"
)

const (
	MiB = 1048576
)

func toMiB(bytes int64) float64 {
	return float64(bytes) / MiB
}

func pct(bytes, total int64) float64 {
	return float64(bytes*100) / float64(total)
}

func main() {
	if(len(os.Args)<2) {
		fmt.Printf("Usage: %d {filename}\n",os.Args[0])
		return
	}
	filename:=os.Args[1]
	fi,err:=slicesync.Hash(filename,0,slicesync.AUTOSIZE)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		return
	}
	fmt.Println(fi.Hash)
}
