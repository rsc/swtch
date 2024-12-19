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
	"strings"

	"rsc.io/swtch/servegcs"
)

func main() {
	http.HandleFunc("/.info", info)
	http.Handle("/", specialHeaders(servegcs.Handler("swtch.com", "swtch/www")))
	http.HandleFunc("/plan9port/", servegcs.RedirectHost("9fans.github.io"))
	http.HandleFunc("www.swtch.com/", servegcs.RedirectHost("swtch.com"))

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}

func info(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Go version: %s (Cloud Run)\n", runtime.Version())
}

func specialHeaders(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Content-Type", "application/wasm")
			enc := ", " + r.Header.Get("Accept-Encoding")
			if r.URL.Query().Get("ebr") == "1" && strings.Contains(enc, ", br") {
				w.Header().Set("Content-Encoding", "br")
				r.URL.Path += ".ebr"
			} else if r.URL.Query().Get("egz") == "1" && strings.Contains(enc, ", gzip") {
				w.Header().Set("Content-Encoding", "gzip")
				r.URL.Path += ".egz"
			}
		}
		h.ServeHTTP(w, r)
	}
}
