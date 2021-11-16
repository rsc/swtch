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
	http.Handle("/", servegcs.Handler("swtch.com", "swtch/www"))
	http.HandleFunc("/plan9port/", servegcs.RedirectHost("9fans.github.io"))
	http.HandleFunc("www.swtch.com/", servegcs.RedirectHost("swtch.com"))

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}

func info(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Go version: %s (Cloud Run)\n", runtime.Version())
}
