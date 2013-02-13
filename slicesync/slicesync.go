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
	var server, filename, dest, alike string
	var slice int64
	flag.StringVar(&server, "server", "localhost:8000", "Server to sync from")
	flag.StringVar(&filename, "filename", "", "Remote filename to sync from")
	flag.StringVar(&dest, "dest", "", "(Optional) Local destination")
	flag.StringVar(&alike, "alike", "", "(Optional) Local similar, previous or look-alike file")
	flag.Int64Var(&slice, "slice", 10485760, "(Optional) Slice size")
	flag.Parse()
	if server == "" || filename == "" {
		flag.PrintDefaults()
		return
	}
	d := dest
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
	fmt.Printf("slicesync\nhttp://%s/dump/%s -> %s %s\n[slice=%v]\n", server, filename, d, a, slice)
	stats, err := slicesync.Slicesync(server, filename, dest, alike, slice)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		return
	}
	fmt.Printf("Done with %f%% downloads\n", pct(stats.Downloaded, stats.Size))
	fmt.Printf("%ddownloads of %fMiB (max slice)\n", stats.Downloads, toMiB(slice))
	fmt.Printf("%fMiB downloaded of %fMiB total\n", toMiB(stats.Downloaded), toMiB(stats.Size))
}
