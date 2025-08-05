// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"embed"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/mod/module"
	"google.golang.org/appengine/v2"
	"google.golang.org/appengine/v2/memcache"
)

//go:embed index.html viewer.*
var static embed.FS

func main() {
	http.HandleFunc("/.info", info)
	http.HandleFunc("/", modViewer)
	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}

func info(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Go version: %s\n", runtime.Version())
}

var deployID = os.Getenv("GAE_DEPLOYMENT_ID")
var staticHandler http.Handler = http.FileServer(http.FS(static))

func modViewer(w http.ResponseWriter, r *http.Request) {
	if i := strings.Index(r.URL.Path, "@"); i < 0 {
		staticHandler.ServeHTTP(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/") {
		http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, "/"), http.StatusSeeOther)
		return
	}

	if deployID == "" {
		w.Write(serve(r.URL.Path))
		return
	}

	ctx := appengine.NewContext(r)
	sum := sha256.Sum256([]byte(r.URL.Path + "#" + deployID))
	key := fmt.Sprintf("view.%x", sum[:])
	item, err := memcache.Get(ctx, key)
	if err != nil {
		data := serve(r.URL.Path)
		item = &memcache.Item{
			Key:        key,
			Value:      data,
			Expiration: 15 * time.Minute,
		}
		if err := memcache.Set(ctx, item); err != nil {
			log.Print(err)
		}
	}
	w.Write(item.Value)
}

func serve(urlPath string) []byte {
	i := strings.Index(urlPath, "@")
	if i < 0 {
		return []byte("Page not found.\n")
	}
	mod, file := "", ""
	j := strings.Index(urlPath[i:], "/")
	if j < 0 {
		mod, file = urlPath, ""
	} else {
		mod, file = urlPath[:i+j], urlPath[i+j+1:]
	}
	mod, vers, _ := strings.Cut(mod, "@")
	mod = strings.TrimPrefix(mod, "/")

	epath, err := module.EscapePath(mod)
	if err != nil {
		return []byte("Invalid module path: " + mod + "\n")
	}
	evers, err := module.EscapeVersion(vers)
	if err != nil {
		return []byte("Invalid module version.\n")
	}
	name := epath + "/@v/" + evers + ".zip"
	url := "https://proxy.golang.org/" + name

	req, err := http.NewRequest("HEAD", url, nil)
	req.Header.Set("Disable-Module-Fetch", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return []byte(fmt.Sprintf("%s: HTTP error: %v\n", url, err))
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return []byte(fmt.Sprintf("%s: HTTP error: %v\n", url, resp.Status))
	}
	if resp.Header.Get("Accept-Ranges") != "bytes" {
		return []byte(fmt.Sprintf("%s: bad Accept-Range: %v", url, resp.Header.Get("Accept-Ranges")))
	}
	size, err := strconv.ParseUint(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return []byte(fmt.Sprintf("%s: bad Content-Length: %v", url, err))
	}
	rc := &remoteReaderAt{
		url:  url,
		size: int64(size),
	}
	r, err := zip.NewReader(rc, int64(size))
	if err != nil {
		return []byte(fmt.Sprintf("%s: %v", url, err))
	}

	var dir []string
	full := mod + "@" + vers + "/" + file
	fslash := full
	if file != "" {
		fslash += "/"
	}
	have := make(map[string]bool)
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, fslash) {
			elem, _, _ := strings.Cut(f.Name[len(fslash):], "/")
			if !have[elem] {
				have[elem] = true
				dir = append(dir, fslash+elem)
			}
		}
	}
	if len(dir) > 0 {
		return serveDir(mod, vers, file, dir)
	}

	for _, f := range r.File {
		if f.Name == full {
			return serveFile(mod, vers, file, f)
		}
	}

	return []byte("Not found.\n")
}

type remoteReaderAt struct {
	url  string
	size int64
}

func (r *remoteReaderAt) ReadAt(b []byte, off int64) (int, error) {
	n := r.size - off
	if n > int64(len(b)) {
		n = int64(len(b))
	}
	b = b[:n]
	req, err := http.NewRequest("GET", r.url, nil)
	req.Header.Set("Disable-Module-Fetch", "true")
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+n-1))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != 206 {
		resp.Body.Close()
		return 0, fmt.Errorf("%s: %s", r.url, resp.Status)
	}
	if resp.Header.Get("Content-Length") != fmt.Sprint(n) {
		resp.Body.Close()
		return 0, fmt.Errorf("%s: bad Content-Length: %v != %v", r.url, resp.Header.Get("Content-Length"), n)
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return 0, fmt.Errorf("%s: reading body: %v", r.url, err)
	}
	if int64(len(data)) != n {
		return 0, fmt.Errorf("%s: unexpected data length %v != %v", r.url, len(data), n)
	}
	return copy(b, data), nil
}

func printHeader(buf *bytes.Buffer, mod, vers, file string) {
	e := html.EscapeString
	buf.WriteString("<!DOCTYPE html>\n<head>\n")
	buf.WriteString("<script src=\"/viewer.js\"></script>\n")
	buf.WriteString("<link rel=\"stylesheet\" href=\"/viewer.css\">\n")
	buf.WriteString("<script src=\"/viewer.js\"></script>\n")
	fmt.Fprintf(buf, `<title>%s@%s/%s - go module viewer</title>`, e(mod), e(vers), e(file))
	buf.WriteString("\n</head><body onload=\"highlight()\"><pre>\n")
	fmt.Fprintf(buf, `<b><a href="/%s@%s/">%s@%s</a>`, e(mod), e(vers), e(mod), e(vers))
	f := ""
	for _, elem := range strings.Split(file, "/") {
		f += "/" + elem
		fmt.Fprintf(buf, `/<a href="/%s@%s%s">%s</a>`, e(mod), e(vers), e(f), e(elem))
	}
	fmt.Fprintf(buf, `</b> <small>(<a href="/">about</a>)</small>`)
	fmt.Fprintf(buf, "\n\n")
}

func serveDir(mod, vers, file string, dir []string) []byte {
	var buf bytes.Buffer
	e := html.EscapeString
	printHeader(&buf, mod, vers, file)
	for _, file := range dir {
		// Note: file is the full path including mod@vers.
		fmt.Fprintf(&buf, "<a href=\"/%s\">%s</a>\n", e(file), e(path.Base(file)))
	}
	return buf.Bytes()
}

var nl = []byte("\n")

func serveFile(mod, vers, file string, zf *zip.File) []byte {
	if zf.UncompressedSize64 > 32<<20 || zf.CompressedSize64 > 32<<20 {
		return []byte("Too big.")
	}
	rc, err := zf.Open()
	if err != nil {
		return []byte("i/o error: " + err.Error())
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return []byte("i/o error: " + err.Error())
	}
	if !isText(data) {
		return data
	}

	var buf bytes.Buffer
	e := html.EscapeString
	printHeader(&buf, mod, vers, file)
	n := 1 + bytes.Count(data, nl)
	wid := len(fmt.Sprintf("%d", n))
	wid = (wid+2+7)&^7 - 2
	n = 1
	for len(data) > 0 {
		var line []byte
		line, data, _ = bytes.Cut(data, nl)
		fmt.Fprintf(&buf, "<span id=\"L%d\">%*d  %s\n</span>", n, wid, n, e(string(line)))
		n++
	}
	return buf.Bytes()
}

// isText reports whether a significant prefix of s looks like correct UTF-8;
// that is, if it is likely that s is human-readable text.
func isText(s []byte) bool {
	const max = 1024 // at least utf8.UTFMax
	if len(s) > max {
		s = s[0:max]
	}
	for i, c := range string(s) {
		if i+utf8.UTFMax > len(s) {
			// last char may be incomplete - ignore
			break
		}
		if c == 0xFFFD || c < ' ' && c != '\n' && c != '\t' && c != '\f' && c != '\r' {
			// decoding error or control character - not a text file
			return false
		}
	}
	return true
}
