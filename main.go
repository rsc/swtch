// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Swtch is the web server for swtch.com.
//
// Usage:
//
//	swtch [-addr address] [-tls]
//
package main

import (
	"flag"
	"log"
	"net/http"
)

var (
	addr     = flag.String("addr", ":http", "serve http on `address`")
	serveTLS = flag.Bool("tls", false, "serve https on :443")
	webroot  = flag.String("webroot", "./web", "serve files from `dir`")
	host     = "swtch.com"
	fs       http.Handler
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("swtch: ")
	flag.Parse()
	fs = http.FileServer(http.Dir(*webroot))
	http.HandleFunc("swtch.com/", tls)
	http.HandleFunc("www.swtch.com/", redirect)
	if *serveTLS {
		go func() {
			log.Fatal(http.ListenAndServeTLS(":https", host+".crt", host+".key", nil))
		}()
	}
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func tls(w http.ResponseWriter, req *http.Request) {
	if req.TLS == nil {
		redirect(w, req)
		return
	}
	fs.ServeHTTP(w, req)
}

func redirect(w http.ResponseWriter, req *http.Request) {
	req.URL.Host = "swtch.com"
	req.URL.Scheme = "https"
	http.Redirect(w, req, req.URL.String(), 302)
}
