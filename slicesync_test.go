package slicesync_test

import (
	"bytes"
	"fmt"
	"github.com/josvazg/slicesync"
	"hash/adler32"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testfile = `AAAAAAAAA
BBBBBBBBB
CCCCCCCCC
DDDDDDDDD
EEEEEEEEE
AAAAAAAAA
`
	Window   = 16
	likefile = `AAAAAAAAA
BBBBBBBBB
CCCCCcCCC
DDDDDDDDD
EEEeEEEEE
AAAAAAAaA
`
	host      = "localhost"
	port      = 8000
	probeFile = "a/b/c/file.txt"
	testdir   = "testdir"
)

func dieOnError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func prepare(t *testing.T) {
	dieOnError(t, os.MkdirAll(testdir, 0750))
	dieOnError(t, os.Chdir(testdir))
}

func dispose(t *testing.T) {
	dieOnError(t, os.Chdir(".."))
	dieOnError(t, os.RemoveAll(testdir))
}

func TestRolling(t *testing.T) {
	testdata, err := genTestData()
	dieOnError(t, err)
	if len(testdata) < Window {
		t.Fatal(fmt.Errorf("Datasize expected >%vbytes but got only %vbytes! (write more go code! ;-)",
			Window, len(testdata)))
	}
	a32 := adler32.New()
	ra32 := slicesync.NewRollingAdler32()
	// First load the window
	a32.Write(testdata[0:Window])
	ra32.Write(testdata[0:Window])
	if a32.Sum32() != ra32.Sum32() {
		t.Fatal(fmt.Errorf("Initial checksum expected was %x but got %x instead!", a32.Sum32(), ra32.Sum32()))
	}
	// Now roll-it and compare with full re-calculus at each position
	for i := 1; i < len(testdata)-Window; i++ {
		a32.Reset()
		a := testdata[i : i+Window]
		a32.Write(a)
		rolled := ra32.Roll32(Window, testdata[i-1], a[len(a)-1])
		if a32.Sum32() != rolled {
			t.Fatal(fmt.Errorf("Checksum at position %d expected was 0x%08x but got 0x%08x instead! a=%v old=%v",
				i, a32.Sum32(), rolled, a, testdata[i-1]))
		}
	}
}

func genTestData() (data []byte, err error) {
	finfos, err := ioutil.ReadDir(".")
	if err != nil {
		return
	}
	datasize := int64(0)
	for _, fi := range finfos {
		if strings.HasSuffix(fi.Name(), ".go") {
			datasize += fi.Size()
		}
	}
	data = make([]byte, 0, datasize) // single allocation
	for _, fi := range finfos {
		if strings.HasSuffix(fi.Name(), ".go") {
			filedata, err := ioutil.ReadFile(fi.Name())
			if err != nil {
				return nil, err
			}
			data = append(data, filedata...)
		}
	}
	return
}

var tests = []struct {
	filename   string
	start, len int64
	expected   string
	goodhash   string
}{
	{"testfile.txt", 0, 0, testfile, "sha1-6e1eb4d4daf850c250bdc9a16669c7f66915f842"},
	{"testfile.txt", 0, 10, "AAAAAAAAA\n", "adler32+md5-0dca0254f252b28c22d0bb68caf870df063b6064"},
	{"testfile.txt", 10, 10, "BBBBBBBBB\n", "adler32+md5-0e00025d961310d0926542e45d7190a22d68b48c"},
}

func TestSlices(t *testing.T) {
	prepare(t)
	hnd := &slicesync.LocalHashNDump{Dir: "."}
	err := ioutil.WriteFile("testfile.txt", ([]byte)(testfile), 0750)
	if err != nil {
		t.Fatal(err)
	}
	for i, test := range tests {
		dmp, _, err := hnd.Dump(test.filename, test.start, test.len)
		if err != nil {
			t.Fatal(err)
		}
		buf := bytes.NewBufferString("")
		io.Copy(buf, dmp)
		str := buf.String()
		if str != test.expected {
			t.Fatalf("Test #%d failed: expected '%s' but got '%s'\n%v\n",
				i, test.expected, str, test)
		}
		hsh := hash(buf.Bytes(), test.len != 0)
		if hsh != test.goodhash {
			t.Fatalf("Test #%d failed: expected hash '%s' but got '%s'\n", i, test.goodhash, hsh)
		}
	}
	dispose(t)
}

func hash(bytes []byte, sliced bool) string {
	var h slicesync.NamedHash
	if sliced {
		h = slicesync.NewSliceHasher()
	} else {
		h = slicesync.NewHasher()
	}
	h.Write(bytes)
	return fmt.Sprintf("%v-%x", h.Name(), h.Sum(nil))
}

var refreshtests = []struct {
	files, deleted []string
}{
	{[]string{"f1.txt", "f2.txt", "f3.txt"}, []string{}},
	{[]string{"f1.txt", "f2.txt"}, []string{"f3.txt"}},
	{[]string{"dir/f1.txt", "dir/f2.txt", "dir/f3.txt"}, []string{}},
	{[]string{"dir/f1.txt", "dir/f2.txt"}, []string{"dir/f3.txt"}},
	{[]string{}, []string{"dir/f1.txt", "dir/f2.txt", "dir/f3.txt"}},
}

func TestRefresh(t *testing.T) {
	prepare(t)
	dieOnError(t, os.MkdirAll("dir", 0750))
	for i, rt := range refreshtests {
		for _, filename := range rt.files {
			err := ioutil.WriteFile(filename, ([]byte)(testfile), 0750)
			dieOnError(t, err)
		}
		for _, filename := range rt.deleted {
			err := os.RemoveAll(filename)
			dieOnError(t, err)
		}
		err := slicesync.HashDir(".", 10, true)
		dieOnError(t, err)
		for _, filename := range rt.files {
			if !slicesync.IsHashFileValid(".", filename) {
				hfilename := slicesync.SlicesyncFile(".", filename)
				t.Fatalf("Test %d: Expected %v file has no valid hash dump at %v!\n", i, filename, hfilename)
			}
		}
		for _, filename := range rt.deleted {
			hfilename := slicesync.SlicesyncFile(".", filename)
			if _, err := os.Stat(hfilename); err == nil {
				t.Fatalf("Test %d: Unexpected %v hash dump for file %v!\n", i, hfilename, filename)
			}
		}
		//fmt.Printf("TestRefresh %v OK!\n", i)
	}
	dispose(t)
}

func fn(dir string, fi os.FileInfo) error {
	path := filepath.Join(dir, fi.Name())
	fmt.Println(path)
	if fi.IsDir() {
		if e := foreachFileInDir(path, fn); e != nil {
			return e
		}
	}
	return nil
}

var probetests = []struct {
	base, path string
}{
	{"a/b", "c/file.txt"},
	{"", "a/b/c/file.txt"},
	{"a", "b/c/file.txt"},
	{"a/b/c", "file.txt"},
}

func TestProbe(t *testing.T) {
	prepare(t)
	slicesync.HashDir(".", slicesync.MiB, false)
	p := port + 1
	wserver := slicesync.NewHashNDumpServer(p, ".", "/")
	go wserver.ListenAndServe()
	for i, pt := range probetests {
		wserver.Handler = slicesync.SetupHashNDumpServer(".", pt.base)
		probeUrl := fmt.Sprintf("%v:%v/%v", host, p, probeFile)
		//fmt.Println(p, " -> base='", pt.base, "'")
		server, file, e := slicesync.Probe(probeUrl)
		dieOnError(t, e)
		if file != pt.path {
			t.Fatalf("Test %d: Expected path %v but got %v (server=%v)!\n", i, pt.path, file, server)
		}
	}
	dispose(t)
}

func foreachFileInDir(dir string, fn func(dir string, fi os.FileInfo) error) (e error) {
	d, e := os.Open(dir)
	if e != nil {
		return e
	}
	defer d.Close()
	for fis, e := d.Readdir(3); e == nil && len(fis) > 0; fis, e = d.Readdir(3) {
		for _, fi := range fis {
			if e = fn(dir, fi); e != nil {
				return e
			}
		}
	}
	if e != io.EOF {
		return
	}
	return nil
}

var synctests = []struct {
	filename, content  string
	slice, differences int64
}{
	{"testfile2.txt", likefile, 10, 30},   // 0
	{"testfile2.txt", likefile, 1000, 60}, // 1
}

func TestSync(t *testing.T) {
	prepare(t)
	err := ioutil.WriteFile("testfile.txt", ([]byte)(testfile), 0750)
	dieOnError(t, err)
	go slicesync.ServeHashNDump(port, ".", "")
	url := fmt.Sprintf("%v:%v/%v", host, port, "testfile.txt")
	for i, st := range synctests {
		err := ioutil.WriteFile(st.filename, ([]byte)(st.content), 0750)
		dieOnError(t, err)
		// prepares diffs to be served AFTER the files were created
		err = slicesync.HashFile(".", "testfile.txt", st.slice)
		dieOnError(t, err)
		err = slicesync.HashFile(".", st.filename, st.slice)
		dieOnError(t, err)
		diffs, err := slicesync.Slicesync(url, st.filename, "", st.slice)
		dieOnError(t, err)
		if diffs.Differences != st.differences {
			t.Fatalf("Test %d: Expected %d differences to sync %s, but got %d!\n",
				i, st.differences, st.filename, diffs.Differences)
		}
	}
	dispose(t)
}
