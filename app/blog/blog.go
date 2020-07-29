// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"

	"rsc.io/swtch/servegcs"
)

var httpAddr = flag.String("http", "localhost:8080", "HTTP listen address")

func main() {
	http.Handle("/", servegcs.Handler("research.swtch.com", "swtch/www-blog"))
	http.Handle("/feeds/posts/default", http.RedirectHandler("/feed.atom", http.StatusFound))

	flag.Parse()

	if os.Getenv("GAE_ENV") == "standard" {
		log.Println("running in App Engine Standard mode")
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		*httpAddr = ":" + port
	}
	l, err := net.Listen("tcp", *httpAddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("serving", *httpAddr)
	log.Fatal(http.Serve(l, nil))
}
