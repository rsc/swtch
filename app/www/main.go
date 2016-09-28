// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package www

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/storage"

	"google.golang.org/appengine"
	"google.golang.org/appengine/blobstore"
	"google.golang.org/appengine/log"
)

const (
	myHost       = "swtch.com"
	bucketName   = "swtch"
	bucketPrefix = "www"
)

func init() {
	http.HandleFunc("/", handler)
	http.HandleFunc("www."+myHost+"/", redirect)
}

var badRobot = `User-agent: *
Disallow: /
`

func handler(w http.ResponseWriter, r *http.Request) {
	// Keep robots away from test instances.
	if r.URL.Host != myHost && r.URL.Path == "/robots.txt" {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(badRobot))
		return
	}

	// Disallow any "dot files" or dot-dot elements.
	if strings.Contains(r.URL.Path, "/.") || !strings.HasPrefix(r.URL.Path, "/") {
		http.Error(w, "invalid URL", http.StatusBadRequest)
		return
	}

	file := bucketPrefix + r.URL.Path

	// Redirect /index.html to directory.
	if strings.HasSuffix(file, "/index.html") {
		localRedirect(w, r, "./")
		return
	}

	// Check that file exists.
	ctx := appengine.NewContext(r)
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Errorf(ctx, "failed to create client: %v", err)
		return
	}
	defer client.Close()
	bucket := client.Bucket(bucketName)

	attrs, err := bucket.Object(file).Attrs(ctx)

	if err == storage.ErrObjectNotExist {
		// Maybe file is a directory containing index.html?
		dir := strings.TrimSuffix(file, "/") + "/"
		if attrs1, err1 := bucket.Object(dir + "index.html").Attrs(ctx); err1 == nil {
			if file != dir {
				localRedirect(w, r, path.Base(file)+"/")
				return
			}
			file += "index.html"
			attrs, err = attrs1, err1
		}
	}

	if err != nil {
		log.Errorf(ctx, "lookup %s/%s: %v", bucketName, file, err)
		if err != storage.ErrObjectNotExist {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Custom 404 body.
		if r, err := bucket.Object(bucketPrefix + "/404.html").NewReader(ctx); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			io.Copy(w, r)
			r.Close()
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Handle ranges, etags so that reloads are fast (send 304s).
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Etag", fmt.Sprintf("%x", attrs.MD5))

	if checkLastModified(w, r, attrs.Updated) {
		return
	}
	rangeReq, done := checkETag(w, r, attrs.Updated)
	if done {
		return
	}

	// Something magical somewhere copies the byte range from the request
	// unless we override it explicitly. Since we are implementing If-Range,
	// we need to override it.
	if rangeReq == "" {
		rangeReq = "bytes=0-"
	}
	w.Header().Set("X-AppEngine-BlobRange", rangeReq)

	key, err := blobstore.BlobKeyForFile(ctx, "/gs/"+bucketName+"/"+file)
	if err != nil {
		log.Errorf(ctx, "blobstore.BlobKeyForFile: %v", err)
		http.Error(w, "problem loading file", http.StatusInternalServerError)
		return
	}
	blobstore.Send(w, key)
}

func redirect(w http.ResponseWriter, req *http.Request) {
	req.URL.Host = myHost
	http.Redirect(w, req, req.URL.String(), 302)
}

// localRedirect gives a Moved Permanently response.
// It does not convert relative paths to absolute paths like Redirect does.
// Copied from net/http.
func localRedirect(w http.ResponseWriter, r *http.Request, newPath string) {
	if q := r.URL.RawQuery; q != "" {
		newPath += "?" + q
	}
	w.Header().Set("Location", newPath)
	w.WriteHeader(http.StatusMovedPermanently)
}

// modtime is the modification time of the resource to be served, or IsZero().
// return value is whether this request is now complete.
// Copied from net/http.
func checkLastModified(w http.ResponseWriter, r *http.Request, modtime time.Time) bool {
	if modtime.IsZero() || modtime.Equal(unixEpochTime) {
		// If the file doesn't have a modtime (IsZero), or the modtime
		// is obviously garbage (Unix time == 0), then ignore modtimes
		// and don't process the If-Modified-Since header.
		return false
	}

	// The Date-Modified header truncates sub-second precision, so
	// use mtime < t+1s instead of mtime <= t to check for unmodified.
	if t, err := time.Parse(http.TimeFormat, r.Header.Get("If-Modified-Since")); err == nil && modtime.Before(t.Add(1*time.Second)) {
		h := w.Header()
		delete(h, "Content-Type")
		delete(h, "Content-Length")
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	w.Header().Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))
	return false
}

// checkETag implements If-None-Match and If-Range checks.
//
// The ETag or modtime must have been previously set in the
// ResponseWriter's headers. The modtime is only compared at second
// granularity and may be the zero value to mean unknown.
//
// The return value is the effective request "Range" header to use and
// whether this request is now considered done.
// Copied from net/http.
func checkETag(w http.ResponseWriter, r *http.Request, modtime time.Time) (rangeReq string, done bool) {
	etag := w.Header().Get("Etag")
	rangeReq = r.Header.Get("Range")

	// Invalidate the range request if the entity doesn't match the one
	// the client was expecting.
	// "If-Range: version" means "ignore the Range: header unless version matches the
	// current file."
	// We only support ETag versions.
	// The caller must have set the ETag on the response already.
	if ir := r.Header.Get("If-Range"); ir != "" && ir != etag {
		// The If-Range value is typically the ETag value, but it may also be
		// the modtime date. See golang.org/issue/8367.
		timeMatches := false
		if !modtime.IsZero() {
			if t, err := http.ParseTime(ir); err == nil && t.Unix() == modtime.Unix() {
				timeMatches = true
			}
		}
		if !timeMatches {
			rangeReq = ""
		}
	}

	if inm := r.Header.Get("If-None-Match"); inm != "" {
		// Must know ETag.
		if etag == "" {
			return rangeReq, false
		}

		// TODO(bradfitz): non-GET/HEAD requests require more work:
		// sending a different status code on matches, and
		// also can't use weak cache validators (those with a "W/
		// prefix).  But most users of ServeContent will be using
		// it on GET or HEAD, so only support those for now.
		if r.Method != "GET" && r.Method != "HEAD" {
			return rangeReq, false
		}

		// TODO(bradfitz): deal with comma-separated or multiple-valued
		// list of If-None-match values. For now just handle the common
		// case of a single item.
		if inm == etag || inm == "*" {
			h := w.Header()
			delete(h, "Content-Type")
			delete(h, "Content-Length")
			w.WriteHeader(http.StatusNotModified)
			return "", true
		}
	}
	return rangeReq, false
}

var unixEpochTime = time.Unix(0, 0)
