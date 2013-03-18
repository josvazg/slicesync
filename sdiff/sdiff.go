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
	if len(os.Args) < 4 {
		fmt.Printf(
			"Usage: %v {server} {filename} {local alike} (optional slice, 1MB by default)\n",
			os.Args[0])
		return
	}
	server := os.Args[1]
	filename := os.Args[2]
	alike := os.Args[3]
	slice := int64(MiB)
	if len(os.Args) > 4 {
		var err error
		slice, err = strconv.ParseInt(os.Args[4], 10, 64)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	fmt.Printf("diff http://%s/dump/%s -> %s\n[slice=%v]\n", server, filename, alike, slice)
	diffs, err := slicesync.CalcDiffs(server, filename, alike, slice)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		return
	}
	fmt.Println(diffs.Print())
}
