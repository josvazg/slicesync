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

// HashNDumpServer prepares an HTTP Server to Hash and Dump slices of files remotely
func SetupHashNDump() {
	http.HandleFunc("/favicon.ico", http.NotFound)
	http.Handle("/hash/", http.StripPrefix("/hash/", http.HandlerFunc(hash)))
	http.Handle("/dump/", http.StripPrefix("/dump/", http.HandlerFunc(dump)))
}

// HashNDumpServer prepares an HTTP Server to Hash and Dump slices of files remotely
func HashNDumpServer(port int) {
	SetupHashNDump()
	http.ListenAndServe(fmt.Sprintf(":%v", port), nil)
}

// hash is a http request handler to return file info, including hashes of file slices
func hash(w http.ResponseWriter, r *http.Request) {
	filename, offset, size, err := readArgs(w, r)
	if handleError(w, r, err) {
		return
	}
	fi, err := Hash(filename, offset, size)
	if handleError(w, r, err) {
		return
	}
	json, err := json.Marshal(fi)
	if handleError(w, r, err) {
		return
	}
	w.Write(json)
}

// dump is a http request handler to return a file slice
func dump(w http.ResponseWriter, r *http.Request) {
	filename, offset, size, err := readArgs(w, r)
	if handleError(w, r, err) {
		return
	}
	sliced := !(offset == 0 && size == 0)
	slice, size, err := Dump(filename, offset, size)
	if handleError(w, r, err) {
		return
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%v", size))
	w.Header().Set("Content-Type", "application/octet-stream")
	downfilename := filename
	if sliced {
		downfilename = fmt.Sprintf("%s(%v-%v)%s", noExt(filename), offset, size, path.Ext(filename))
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downfilename))
	io.Copy(w, slice)
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
	size := r.FormValue("size")
	o = 0
	s = AUTOSIZE
	if offset != "" {
		i, err := strconv.ParseInt(offset, 10, 64)
		if err != nil {
			return "", 0, 0, err
		}
		o = i
	}
	if size != "" {
		i, err := strconv.ParseInt(size, 10, 64)
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

// RHash returns the hash of a remote file slice
func RHash(server, filename string, pos, slice int64) (*FileInfo, error) {
	resp, err := RRead(fullUrl(server, "hash/", filename, pos, slice))
	if err != nil {
		return nil, err
	}
	//fmt.Printf("%s\n", string(resp))
	fi := FileInfo{}
	err = json.Unmarshal(resp, &fi)
	if err != nil {
		return nil, err
	}
	return &fi, nil
}

// RDump returns the hash of a remote file slice
func RDump(server, filename string, pos, slice int64) (io.ReadCloser, error) {
	return ROpen(fullUrl(server, "dump/", filename, pos, slice))
}

// ROpen opens a remote URL incoming stream
func ROpen(url string) (io.ReadCloser, error) {
	//fmt.Printf("ROpen %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Error " + resp.Status + " connecting to " + url)
	}
	return resp.Body, nil
}

// RRead opens (ROpen) a remote URL and reads the body contents into a string
func RRead(url string) ([]byte, error) {
	//fmt.Printf("RRead %s\n", url)
	r, err := ROpen(url)
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

// shortUrl returns the proper short service Url for a server, method and filename
func shortUrl(server, context, filename string) string {
	return fmt.Sprintf("http://%s/%s%s", server, context, filename)
}

// serviceUrl returns the proper service Url for a server, method, filename, pos and slice
func fullUrl(server, context, filename string, pos, slice int64) string {
	return fmt.Sprintf("http://%s/%s%s?offset=%v&size=%v",
		server, context, filename, pos, slice)
}
