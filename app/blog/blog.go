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

	"rsc.io/swtch/servegcs"
)

func main() {
	http.HandleFunc("/.info", info)
	http.Handle("/", servegcs.Handler("research.swtch.com", "swtch/www-blog"))
	http.Handle("/feeds/posts/default", http.RedirectHandler("/feed.atom", http.StatusFound))

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}

func info(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Go version: %s (Cloud Run)\n", runtime.Version())
}
