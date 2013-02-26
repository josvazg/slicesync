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
		fmt.Printf(
			"Usage: %v {server} {filename} {local alike} (optional slice, 1MB by default)\n",
			os.Args[0])
		return
	}
	server:=os.Args[1]
	filename:=os.Args[2]
	alike:=os.Args[3]
	slice:=int64(MiB)
	if len(os.Args)>4 {
		fmt.Scanf(os.Args[4],"%d",slice)
	}
	fmt.Printf("diff\nhttp://%s/dump/%s -> %s [slice=%v]\n", server, filename, alike, slice)
	diffs,err:=slicesync.Diffs(server,filename,alike,slice)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		return
	}
	fmt.Println(diffs)
}
