// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"rsc.io/swtch/qrweb"
)

func carp(f http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Content-Type", "text/plain")
				fmt.Fprintf(w, "<pre>\npanic: %s\n\n%s\n", err, debug.Stack())
			}
		}()
		f.ServeHTTP(w, req)
	})
}

func main() {
	http.Handle("/qr/frame", carp(qrweb.Frame))
	http.Handle("/qr/frames", carp(qrweb.Frames))
	http.Handle("/qr/mask", carp(qrweb.Mask))
	http.Handle("/qr/masks", carp(qrweb.Masks))
	http.Handle("/qr/arrow", carp(qrweb.Arrow))
	http.Handle("/qr/draw", carp(qrweb.Draw))
	http.Handle("/qr/bitstable", carp(qrweb.BitsTable))
	http.Handle("/qr/encode", carp(qrweb.Encode))
	http.Handle("/qr/show/", carp(qrweb.Show))
	http.Handle("/", http.FileServer(http.Dir(".")))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
