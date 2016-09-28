// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ninefansnet

import (
	"net/http"

	"rsc.io/go-import-redirector/godoc"
)

func init() {
	http.HandleFunc("/", root)
	http.HandleFunc("/archive/", archive)
	http.Handle("/go/", godoc.Redirect("git", "9fans.net/go", "https://github.com/9fans/go"))
}

func root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	http.Redirect(w, r, "http://plan9.bell-labs.com/plan9/", http.StatusFound)
}

func archive(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(404)
	w.Write([]byte(`<html>Sorry, but the 9fans.net archive that was here is no longer available.

<p>See <a href="https://9p.io/wiki/plan9/mailing_lists/">https://9p.io/wiki/plan9/mailing_lists/</a> for alternatives.
`))
}
