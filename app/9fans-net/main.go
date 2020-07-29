// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"

	"rsc.io/go-import-redirector/godoc"
)

func root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	http.Redirect(w, r, "http://9p.io/plan9/", http.StatusFound)
}

func archive(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(404)
	w.Write([]byte(`<html>Sorry, but the 9fans.net archive that was here is no longer available.

<p>See <a href="https://9p.io/wiki/plan9/mailing_lists/">https://9p.io/wiki/plan9/mailing_lists/</a> for alternatives.
`))
}

func main() {
	http.HandleFunc("/.info", info)
	http.HandleFunc("/", root)
	http.HandleFunc("/archive/", archive)
	http.Handle("/go/", godoc.Redirect("git", "9fans.net/go", "https://github.com/9fans/go"))
	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}

func info(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Go version: %s\n", runtime.Version())
}
