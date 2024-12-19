// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package servegcs implements serving a file tree from Google Cloud Storage.
// Files are served with headers allowing caching by Google infrastructure for up to 5 minutes.
//
//	func init() {
//		http.Handle("/", servegcs.Handler("swtch.com", "swtch/www"))
//		http.HandleFunc("www.swtch.com/", servegcs.RedirectHost("swtch.com"))
//	}
package servegcs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"unsafe"

	"cloud.google.com/go/storage"
)

var badRobot = `User-agent: *
Disallow: /
`

func Handler(host, bucket string) http.HandlerFunc {
	i := strings.Index(bucket, "/")
	if i < 0 {
		panic("bucket must have slash")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		handler(host, bucket[:i], bucket[i+1:], w, r)
	}
}

func handler(host, bucketName, bucketPrefix string, w http.ResponseWriter, r *http.Request) {
	// Keep robots away from test instances.
	requestHost := r.URL.Host
	if requestHost == "" {
		requestHost = r.Host
	}
	if requestHost != host && r.URL.Path == "/robots.txt" {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(badRobot))
		return
	}

	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("only GET or HEAD"))
		return
	}

	// Disallow any "dot files" or dot-dot elements, except ".well-known",
	// which is needed for various automated systems.
	replaced := strings.Replace(r.URL.Path, "/.well-known/", "/dot-well-known-is-ok/", -1)
	if strings.Contains(replaced, "/.") || !strings.HasPrefix(replaced, "/") {
		http.Error(w, "invalid URL", http.StatusBadRequest)
		return
	}

	file := bucketPrefix + r.URL.Path

	// Redirect /index.html to directory.
	if strings.HasSuffix(file, "/index.html") {
		localRedirect(w, r, "./")
		return
	}

	ctx := r.Context()
	client, err := storage.NewClient(ctx)
	if err != nil {
		logErrorf(r, "failed to create client: %v", err)
		return
	}
	defer client.Close()

	// Check that file exists.
	attrs, err := lookupAttrs(ctx, client, bucketName, file)

	if err == storage.ErrObjectNotExist {
		// Maybe file is a directory containing index.html?
		dir := strings.TrimSuffix(file, "/") + "/"
		if attrs1, err1 := lookupAttrs(ctx, client, bucketName, dir+"index.html"); err1 == nil {
			if file != dir {
				localRedirect(w, r, path.Base(file)+"/")
				return
			}
			file += "index.html"
			attrs, err = attrs1, err1
		}
	}

	if err != nil {
		logErrorf(r, "lookup %s/%s: %v", bucketName, file, err)
		if err != storage.ErrObjectNotExist {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Custom 404 body.
		if attrs, err := lookupAttrs(ctx, client, bucketName, bucketPrefix+"/404.html"); err == nil {
			if body, err := lookupContent(ctx, client, attrs, bucketName, bucketPrefix+"/404.html"); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				w.Write(body)
				return
			}
		}

		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if status := attrs.Metadata["metadata.httpstatus"]; status != "" {
		if n, err := strconv.Atoi(status); err == nil {
			w.WriteHeader(n)
			w.Write([]byte("I am a status " + status + " page."))
			return
		}
	}

	// Allow caching of found results for 5 minutes.
	// May cut load on our server, and we don't expect our Google Cloud files to change often.
	// Override with standard GCS Cache-Control attribute.
	cacheControl := "public, max-age=300"
	if attrs.CacheControl != "" {
		cacheControl = attrs.CacheControl
	}

	// Request from GCS using stolen authenticated http.Client
	// from inside storage.Client and proxy result back.
	authClient := (*http.Client)(unsafe.Pointer(reflect.ValueOf(client).Elem().FieldByName("hc").Pointer()))

	newURL, err := url.Parse("https://storage.googleapis.com/" + bucketName + "/" + file)
	if err != nil {
		logErrorf(r, "parsing GCS URL: %v", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}

	newReq, err := http.NewRequestWithContext(ctx, "GET", newURL.String(), nil)
	if err != nil {
		logErrorf(r, "NewRequestWithContext: %v", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}

	for _, hdr := range headersToGCS {
		if vals, ok := r.Header[hdr]; ok {
			newReq.Header[hdr] = vals
		}
	}

	resp, err := authClient.Do(newReq)
	if err != nil {
		logErrorf(r, "authClient.Do: %v", err)
		http.Error(w, "failed", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	for _, hdr := range headersFromGCS {
		if vals, ok := resp.Header[hdr]; ok {
			if _, ok := w.Header()[hdr]; !ok {
				w.Header()[hdr] = vals
			}
		}
	}
	w.Header()["Cache-Control"] = []string{cacheControl}

	// Cloud Run limits the size of any one response to 32 MB.
	// But there is an exception for chunked responses.
	// So if the response would be too large, do not set Content-Length,
	// which will force it to be chunked.
	if resp.ContentLength < 30e6 || r.Method == "HEAD" {
		// w.Header().Set("Content-Length", fmt.Sprint(resp.ContentLength))
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

var headersToGCS = []string{
	"Accept",
	"Range",
	"If-Range",
	"If-Modified-Since",
	"If-Unmodified-Since",
	"If-Match",
	"If-None-Match",
}

var headersFromGCS = []string{
	"Accept-Ranges",
	"Content-Range",
	"Content-Type",
	"Expires",
	"Etag",
	"Last-Modified",
}

func lookupAttrs(ctx context.Context, client *storage.Client, bucketName, file string) (*storage.ObjectAttrs, error) {
	return client.Bucket(bucketName).Object(file).Attrs(ctx)
}

func lookupContent(ctx context.Context, client *storage.Client, attrs *storage.ObjectAttrs, bucketName, file string) ([]byte, error) {
	r, err := client.Bucket(bucketName).Object(file).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ioutil.ReadAll(r)
}

func logAny(r *http.Request, severity, format string, args ...interface{}) {
	var trace string
	f := strings.Split(r.Header.Get("X-Cloud-Trace-Context"), "/")
	if len(f) > 0 && f[0] != "" {
		trace = fmt.Sprintf("projects/%s/traces/%s", os.Getenv("GOOGLE_CLOUD_PROJECT"), f[0])
	}

	out, err := json.Marshal(struct {
		Message  string `json:"message"`
		Severity string `json:"severity,omitempty"`
		Trace    string `json:"logging.googleapis.com/trace,omitempty"`
	}{
		fmt.Sprintf(format, args...),
		severity,
		trace,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "json.Marshal: %v\n", err)
	}
	out = append(out, '\n')
	os.Stdout.Write(out)
}

func logErrorf(r *http.Request, format string, args ...interface{}) {
	logAny(r, "ERROR", format, args...)
}
func logInfof(r *http.Request, format string, args ...interface{}) {
	logAny(r, "INFO", format, args...)
}
func logCriticalf(r *http.Request, format string, args ...interface{}) {
	logAny(r, "CRITICAL", format, args...)
}

// localRedirect gives a Moved Permanently response.
// It does not convert relative paths to absolute paths like Redirect does.
func localRedirect(w http.ResponseWriter, r *http.Request, newPath string) {
	if q := r.URL.RawQuery; q != "" {
		newPath += "?" + q
	}
	w.Header().Set("Location", newPath)
	w.WriteHeader(http.StatusMovedPermanently)
}

func RedirectHost(host string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		req.URL.Host = host
		http.Redirect(w, req, req.URL.String(), 302)
	}
}
