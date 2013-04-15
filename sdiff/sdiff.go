package main

import (
	"fmt"
	"github.com/josvazg/slicesync"
	"os"
	"strconv"
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
	if len(os.Args) < 3 {
		fmt.Printf(
			"Usage: %v {fileurl} {local alike} (optional slice, 1MB by default)\n",
			os.Args[0])
		return
	}
	fileurl := os.Args[1]
	alike := os.Args[2]
	slice := int64(MiB)
	if len(os.Args) > 3 {
		var err error
		slice, err = strconv.ParseInt(os.Args[3], 10, 64)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	fmt.Printf("diff http://%s -> %s\n[slice=%v]\n", fileurl, alike, slice)
	diffs, err := slicesync.CalcDiffs(fileurl, alike, slice)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		return
	}
	fmt.Println(diffs.Print())
}
