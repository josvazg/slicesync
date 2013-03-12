package main

import (
	"flag"
	"fmt"
	"github.com/josvazg/slicesync"
	"os"
	"path/filepath"
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
	var to, alike string
	var slice int64
	if len(os.Args) < 3 {
		fmt.Printf("Usage: %v {server} {filename} "+
			"[-to destination] [-alike localAlike] [-slice bytes, default=1MB]\n",
			os.Args[0])
		return
	}
	server := os.Args[1]
	filename := os.Args[2]
	flag.StringVar(&to, "to", "", "(Optional) Local destination")
	flag.StringVar(&alike, "alike", "", "(Optional) Local similar, previous or look-alike file")
	flag.Int64Var(&slice, "slice", 10485760, "(Optional) Slice size")
	flag.Parse()
	if server == "" || filename == "" {
		flag.PrintDefaults()
		return
	}
	d := to
	if d == "" {
		d = filename
	}
	a := ""
	if alike != "" {
		a = fmt.Sprintf("(alike='%s')", alike)
	}
	d, err := filepath.Abs(d)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		return
	}
	fmt.Printf("slicesync\nhttp://%s/dump/%s -> %s \nalike='%s'\n[slice=%v]\n", 
		server, filename, d, a, slice)
	diffs, err := slicesync.Slicesync(server, filename, to, alike, slice)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		return
	}
	fmt.Printf("Done with %v downloads %v%% downloaded\n", len(diffs.Diffs), pct(diffs.Differences, diffs.Size))
	fmt.Printf("%fMiB downloaded of %fMiB total\n", toMiB(diffs.Differences), toMiB(diffs.Size))
}
