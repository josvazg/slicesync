package slicesync

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// -- Server Side --

// SetupHashNDumpServer prepares a Handler for a HashNDumpServer
func SetupHashNDumpServer(dir, prefix string) http.Handler {
	if !strings.HasSuffix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	//fmt.Println("prefix:", prefix)
	smux := http.NewServeMux()
	smux.HandleFunc("/favicon.ico", http.NotFound)
	smux.Handle(prefix, filter(http.StripPrefix(prefix, http.FileServer(http.Dir(dir)))))
	//fmt.Printf("smux=%#v\n", smux)
	return smux
}

// NewHashNDumpServer creates a new NewHashNDumpServer with the setup from SetupHashNDumpServer(dir,prefix)
func NewHashNDumpServer(port int, dir, prefix string) *http.Server {
	return &http.Server{Addr: fmt.Sprintf(":%v", port), Handler: SetupHashNDumpServer(dir, prefix)}
}

// ServeHashNDump runs an HTTP Server created from NewHashNDumpServer to download Hashes and slice Dumps slices of files
func ServeHashNDump(port int, dir, prefix string) {
	NewHashNDumpServer(port, dir, prefix).ListenAndServe()
}

func filter(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//fmt.Println("url=", r.URL)
		h.ServeHTTP(w, r)
	})
}

// -- Client Side --

// RemoteHashNDump implements HashNDumper service remotely through HTTP GET requests
type RemoteHashNDump struct {
	Server string
}

// Hash returns the remote stream of hash slices
func (rhnd *RemoteHashNDump) Hash(filename string) (io.ReadCloser, error) {
	r, _, e := get(calcUrl(rhnd.Server, SlicesyncFile(".", filename)), 0, 0)
	return r, e
}

// Dump returns the contents of a remote slice of the file (or the full file)
func (rhnd *RemoteHashNDump) Dump(filename string, pos, slice int64) (io.ReadCloser, int64, error) {
	rc, r, err := get(calcUrl(rhnd.Server, filename), pos, slice)
	if err != nil {
		return nil, 0, err
	}
	N, err := strconv.ParseInt(r.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, 0, err
	}
	return rc, N, err
}

// probe detects the server base url and separates the server url and the filename for a given remote file url
func Probe(probedUrl string) (server, filename string, err error) {
	if !strings.Contains(probedUrl, "://") {
		probedUrl = "http://" + probedUrl
	}
	u, err := url.Parse(probedUrl)
	if err != nil {
		return "", "", err
	}
	//fmt.Println("Probe ", u)
	fullpath := u.Path
	u.Path = path.Join("/", SlicesyncDir)
	//fmt.Println("Testing ", u.String())
	err = head(u.String() + "/")
	if err == nil {
		u.Path = "/"
		server = u.String()
		filename = fullpath[1:]
		//fmt.Println("Probe Result -> ", server, filename)
		return
	}
	for candidate := path.Dir(fullpath); len(candidate) > 0 && candidate != "/"; candidate = path.Dir(candidate) {
		u.Path = path.Join(candidate, "/", SlicesyncDir)
		//fmt.Println("Testing ", u.String())
		err = head(u.String() + "/")
		if err == nil {
			u.Path = candidate
			server = u.String()
			filename = fullpath[len(candidate)+1:]
			//fmt.Println("Probe Result *-> ", server, filename)
			return
		}
	}
	u.Path = "/"
	err = fmt.Errorf("Remote server %s does not seem to support slicesync! (last error was %v)", u, err)
	return
}

// head tries to access a url and returns an error if something is wrong or nil if all was fine
func head(url string) error {
	r, err := http.DefaultClient.Head(url)
	if err != nil {
		return err
	}
	if r.StatusCode != 200 {
		return fmt.Errorf("Unexpected status %v!", r.Status)
	}
	return nil
}

// get a remote URL incoming stream
func get(url string, pos, slice int64) (io.ReadCloser, *http.Response, error) {
	//fmt.Printf("get %s\n", url)
	get, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	if pos != 0 || slice != 0 {
		get.Header.Add("Range", fmt.Sprintf("bytes=%v-%v", pos, pos+slice-1))
	}
	http.DefaultClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return fmt.Errorf("Check url %v, redirection should not be required!", url)
	}
	resp, err := http.DefaultClient.Do(get)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return nil, nil, fmt.Errorf("Error " + resp.Status + " connecting to " + url)
	}
	return resp.Body, resp, nil
}

// calcUrl returns the Url for the remote file
func calcUrl(server, filename string) string {
	return fmt.Sprintf("%s%s", server, filename)
}
