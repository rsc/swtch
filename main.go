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
	"strings"
	"time"

	"rsc.io/cloud"
	"rsc.io/cloud/diskcache"
	"rsc.io/cloud/google/gcs"
	"rsc.io/cloud/google/metaflag"
)

var (
	addr     = flag.String("addr", ":http", "serve http on `address`")
	host     = flag.String("host", "swtch.com", "serve URLs on `hostname`")
	expire   = flag.Duration("expire", 10*time.Minute, "expire cloud cache entries after duration `d`")
	serveTLS = flag.Bool("tls", true, "serve HTTPS on :443")
	gcscache = flag.String("gcscache", "./gcscache", "cache Google Cloud Storage files in `dir`")
	webroot  = flag.String("web", "gs://swtch/www", "serve files from `dir`")
	certdir  = flag.String("certs", "gs://swtch/certs", "read certificates from `dir`")
	fs       http.Handler
	cache    *diskcache.Cache
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("swtch: ")
	metaflag.Init()
	flag.Parse()

	if !strings.HasPrefix(*webroot, "gs://") {
		log.Fatal("-webroot argument must be a gs:// URL")
	}
	loader, err := gcs.NewLoader("/")
	if err != nil {
		log.Fatal(err)
	}

	cache, err = diskcache.New(*gcscache, loader)
	if err != nil {
		log.Fatal(err)
	}
	if *expire != 0 {
		cache.SetExpiration(*expire)
	}

	fs = http.FileServer(cloud.Dir(cache, strings.TrimPrefix(*webroot, "gs://")))
	http.HandleFunc(*host+"/", defaultHandler)
	if *host != "" && !strings.HasPrefix(*host, "www.") {
		http.HandleFunc("www."+*host+"/", redirect)
	}
	if *serveTLS {
		go func() {
			dir := strings.TrimPrefix(*certdir, "gs://")
			log.Fatal(cloud.ServeHTTPS(":https", cache, dir+"/"+*host+".crt", dir+"/"+*host+".key", nil))
		}()
	}
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func defaultHandler(w http.ResponseWriter, req *http.Request) {
	if *serveTLS && req.TLS == nil {
		redirect(w, req)
		return
	}
	fs.ServeHTTP(w, req)
}

func redirect(w http.ResponseWriter, req *http.Request) {
	req.URL.Host = *host
	if *serveTLS {
		req.URL.Scheme = "https"
	} else {
		req.URL.Scheme = "http"
	}
	http.Redirect(w, req, req.URL.String(), 302)
}
