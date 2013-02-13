package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"os"
	"github.com/josvazg/slicesync"
)

const (
	MiB=1048576
)

func main() {
	var server,filename,dest,alike string
	var slice int64
	flag.StringVar(&server,"server","localhost:8000","Server to sync from")
	flag.StringVar(&filename,"filename","","Remote filename to sync from")
    flag.StringVar(&dest,"dest","","(Optional) Local destination")
    flag.StringVar(&alike,"alike","","(Optional) Local similar, previous or look-alike file")
    flag.Int64Var(&slice,"slice",10485760,"(Optional) Slice size")
    flag.Parse()
    if server=="" || filename=="" {
    	flag.PrintDefaults()
    	return
    }
    d:=dest
    if d=="" {
    	d=filename
    }
    a:=""
    if alike!="" {
    	a=fmt.Sprintf("(alike='%s')",alike)
    }
    d,err:=filepath.Abs(d)
    if err!=nil {
    	fmt.Fprint(os.Stderr, err.Error()+"\n")
    	return
    }
	fmt.Printf("slicesync\nhttp://%s/dump/%s -> %s %s\n[slice=%v]\n", server, filename, d, a, slice)
	stats,err:=slicesync.Slicesync(server,filename,dest,alike,slice)
	if err!=nil {
    	fmt.Fprint(os.Stderr, err.Error()+"\n")
    	return
    }
    fmt.Printf("Done with %v%% downloads\n",(stats.Downloaded*100/stats.Size))
    fmt.Printf("%ddownloads of %vMiB (max slice)\n",stats.Downloads, (slice/MiB))
    fmt.Printf("%vMiB downloaded of %vMiB total\n",(stats.Downloaded/MiB), (stats.Size/MiB))
}
