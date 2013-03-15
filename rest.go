package slicesync

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
)

// -- Server Side --

// HashNDumpServer prepares an HTTP Server to Hash and Dump slices of files remotely
func HashNDumpServer(port int, dir string) {
	SetupHashNDump(&LocalHashNDump{dir})
	http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
}

// HashNDumpServer prepares an HTTP Server to Hash and Dump slices of files remotely
func SetupHashNDump(hnd *LocalHashNDump) {
	http.HandleFunc("/favicon.ico", http.NotFound)
	http.Handle("/hash/", http.StripPrefix("/hash/", hasher(hnd)))
	http.Handle("/bulkhash/", http.StripPrefix("/bulkhash/", bulkhasher(hnd)))
	http.Handle("/dump/", http.StripPrefix("/dump/", dumper(hnd)))
}

// hasher returns a rest/http request handler to return hash info, including hashes of file slices
func hasher(hnd *LocalHashNDump) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filename, offset, slice, err := readArgs(w, r)
		if handleError(w, r, err) {
			return
		}
		hi, err := hnd.Hash(filename, offset, slice)
		if handleError(w, r, err) {
			return
		}
		json, err := json.Marshal(hi)
		if handleError(w, r, err) {
			return
		}
		w.Write(json)
	})
}

// bulkhasher returns a rest/http request handler to return bulkhash stream
func bulkhasher(hnd *LocalHashNDump) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filename, _, slice, err := readArgs(w, r)
		if handleError(w, r, err) {
			return
		}
		in, err := hnd.BulkHash(filename, slice)
		if handleError(w, r, err) {
			return
		}
		io.Copy(w, in)
	})
}

// dumper returns a rest/http request handler to return a file slice (or the entire file)
func dumper(hnd *LocalHashNDump) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filename, offset, slice, err := readArgs(w, r)
		if handleError(w, r, err) {
			return
		}
		sliced := !(offset == 0 && slice == 0)
		sliceData, N, err := hnd.Dump(filename, offset, slice)
		if handleError(w, r, err) {
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%v", N))
		w.Header().Set("Content-Type", "application/octet-stream")
		downfilename := filename
		if sliced {
			downfilename = fmt.Sprintf("%s(%v-%v)%s",
				noExt(filename), offset, slice, path.Ext(filename))
		}
		w.Header().Set("Content-Disposition",
			fmt.Sprintf("attachment; filename=\"%s\"", downfilename))
		io.Copy(w, sliceData)
	})
}

// noExt returns the name without the extension
func noExt(filename string) string {
	return filename[0 : len(filename)-len(path.Ext(filename))]
}

// readArgs reads request args for hash & dump
func readArgs(w http.ResponseWriter, r *http.Request) (f string, o, s int64, e error) {
	filename := r.URL.Path
	if filename != "" && filename[0] == '/' {
		filename = filename[1:]
	}
	if filename == "" {
		return "", 0, 0, fmt.Errorf("Expected filename argument!")
	}
	offset := r.FormValue("offset")
	slice := r.FormValue("slice")
	o = 0
	s = AUTOSIZE
	if offset != "" {
		i, err := strconv.ParseInt(offset, 10, 64)
		if err != nil {
			return "", 0, 0, err
		}
		o = i
	}
	if slice != "" {
		i, err := strconv.ParseInt(slice, 10, 64)
		if err != nil {
			return "", 0, 0, err
		}
		s = i
	}
	return filename, o, s, nil
}

// handleError displays err (if not nil) on Stderr and (if possible) displays a web error page
// it also returns true if the error was found and handled and false if err was nil
func handleError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}

// -- Client Side --

// RemoteHashNDump implements HashNDumper service remotely through a REST service
type RemoteHashNDump struct {
	Server string
}

// Hash returns the hash of a remote file slice
func (rhnd *RemoteHashNDump) Hash(filename string, pos, slice int64) (*HashInfo, error) {
	resp, err := read(fullUrl(rhnd.Server, "hash/", filename, pos, slice))
	if err != nil {
		return nil, err
	}
	//fmt.Printf("%s\n", string(resp))
	hi := HashInfo{}
	err = json.Unmarshal(resp, &hi)
	if err != nil {
		return nil, err
	}
	return &hi, nil
}

// BulkHash returns the remote stream of hash slices
func (rhnd *RemoteHashNDump) BulkHash(filename string, slice int64) (io.ReadCloser, error) {
	r, _, e := open(bulkUrl(rhnd.Server, "bulkhash/", filename, slice))
	return r, e
}

// Dump returns the hash of a remote file slice
func (rhnd *RemoteHashNDump) Dump(filename string, pos, slice int64) (io.ReadCloser, int64, error) {
	rc, r, err := open(fullUrl(rhnd.Server, "dump/", filename, pos, slice))
	if err != nil {
		return nil, 0, err
	}
	N, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, 0, err
	}
	return rc, N, err
}

// read opens (ROpen) a remote URL and reads the body contents into a byte slice
func read(url string) ([]byte, error) {
	//fmt.Printf("RRead %s\n", url)
	r, _, err := open(url)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	buf := make([]byte, 512)
	readed, err := r.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:readed], nil
}

// open a remote URL incoming stream
func open(url string) (io.ReadCloser, *http.Response, error) {
	//fmt.Printf("ROpen %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("Error " + resp.Status + " connecting to " + url)
	}
	return resp.Body, resp, nil
}

// fullUrl returns the proper service Url for a server, method, filename, pos and slice
func fullUrl(server, context, filename string, pos, slice int64) string {
	return fmt.Sprintf("http://%s/%s%s?offset=%v&slice=%v",
		server, context, filename, pos, slice)
}

// bulkUrl returns the bulk url service Url for bulkhash
func bulkUrl(server, context, filename string, slice int64) string {
	return fmt.Sprintf("http://%s/%s%s?slice=%v", server, context, filename, slice)
}
