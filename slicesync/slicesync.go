package main

import (
	"flag"
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

func usage() {
	fmt.Printf("Usage: %v [-to destination] [-alike localAlike] [-slice bytes, default=1MB] {fileurl}\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	var to, alike string
	var slice int64
	flag.StringVar(&to, "to", "", "(Optional) Local destination")
	flag.StringVar(&alike, "alike", "", "(Optional) Local similar, previous or look-alike file")
	flag.Int64Var(&slice, "slice", MiB, "(Optional) Slice size")
	flag.Parse()
	if len(flag.Args()) < 2 {
		usage()
		return
	}
	fileurl := flag.Arg(0)
	if fileurl == "" {
		usage()
		return
	}
	d := ""
	if to != "" {
		d = "->" + to
	}
	a := ""
	if alike != "" {
		a = fmt.Sprintf("(alike='%s')\n", alike)
	}
	fmt.Printf("slicesync\nhttp://%s %s\n%s[slice=%v]\n", fileurl, d, a, slice)
	diffs, err := slicesync.Slicesync(fileurl, d, alike, slice)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		return
	}
	fmt.Printf("Done with %v downloads %v%% downloaded\n", len(diffs.Diffs), pct(diffs.Differences, diffs.Size))
	fmt.Printf("%fMiB downloaded of %fMiB total\n", toMiB(diffs.Differences), toMiB(diffs.Size))
}
