// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package servegcs implements serving a file tree from Google Cloud Storage.
// Recently read data and metadata is cached in memcache, and files are served
// with headers allowing caching by Google infrastructure for up to 5 minutes.
//
//	func init() {
//		http.Handle("/", servegcs.Handler("swtch.com", "swtch/www"))
//		http.HandleFunc("www.swtch.com/", servegcs.RedirectHost("swtch.com"))
//	}
//
package servegcs

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/blobstore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/memcache"
)

const (
	directServeCutoff = 64 * 1024
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
	if r.URL.Host != host && r.URL.Path == "/robots.txt" {
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

	ctx := appengine.NewContext(r)
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Errorf(ctx, "failed to create client: %v", err)
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
		log.Errorf(ctx, "lookup %s/%s: %v", bucketName, file, err)
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

	// Allow caching of found results for 5 minutes.
	// May cut load on our server, and we don't expect our Google Cloud files to change often.
	// Override with standard GCS Cache-Control attribute.
	if attrs.CacheControl != "" {
		w.Header().Set("Cache-Control", attrs.CacheControl)
	} else {
		w.Header().Set("Cache-Control", "public, max-age=300")
	}

	// Take content type from GCS, but add default UTF-8.
	if typ := attrs.ContentType; typ != "" {
		if typ == "text/plain" || typ == "text/html" {
			typ += "; charset=utf-8"
		}
		w.Header().Set("Content-Type", typ)
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

	if attrs.Size < directServeCutoff {
		data, err := lookupContent(ctx, client, attrs, bucketName, file)
		if err == nil {
			w.Write(data)
			return
		}
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

func RedirectHost(host string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		req.URL.Host = host
		http.Redirect(w, req, req.URL.String(), 302)
	}
}

func lookupAttrs(ctx context.Context, client *storage.Client, bucketName, file string) (*storage.ObjectAttrs, error) {
	const (
		cachePrefix  = "www-attrs:"
		cacheVersion = "v2"
	)
	key := cachePrefix + bucketName + "/" + file
	item, err := memcache.Get(ctx, key)
	if err != nil && err != memcache.ErrCacheMiss {
		log.Errorf(ctx, "memcache.Get attrs: %v", err)
	}
	if err == nil {
		log.Infof(ctx, "memcache.Get attrs hit: %v", key)
	}
	forceLookup := false
Again:
	if err != nil || forceLookup {
		log.Infof(ctx, "memcache.Get attrs refresh: %v", key)
		// Get attrs and seed cache.
		attrs, err := client.Bucket(bucketName).Object(file).Attrs(ctx)
		if err != nil && err != storage.ErrObjectNotExist {
			log.Errorf(ctx, "reading object attrs: %v", err)
			return nil, err
		}
		item = new(memcache.Item)
		item.Key = key
		if err == storage.ErrObjectNotExist {
			item.Value = []byte("?")
		} else {
			item.Value = []byte(fmt.Sprintf("%s\n%q\n%q\n%x\n%d\n%s",
				cacheVersion,
				attrs.CacheControl,
				attrs.ContentType,
				attrs.MD5,
				attrs.Size,
				attrs.Updated.UTC().Format(time.RFC3339)))
		}
		// TODO: Limit Expiration based on attrs.CacheControl
		item.Expiration = 5 * time.Minute
		if err := memcache.Set(ctx, item); err != nil {
			log.Errorf(ctx, "caching object attrs: %v", err)
			// Don't return: keep going with filled-in item.
		}
	}

	// Now have item, either from cache or just synthesized.
	// Parse to recreate attrs.
	// We do this even on the cache miss path to ensure that the
	// information returned from a cache miss is not more detailed
	// than the information returned from a cache hit (and that cache hit
	// parsing works at all).
	f := strings.Split(string(item.Value), "\n")
	if len(f) == 1 && f[0] == "?" {
		return nil, storage.ErrObjectNotExist
	}
	attrs := new(storage.ObjectAttrs)
	if len(f) != 6 || f[0] != cacheVersion {
		goto BadCache
	}
	attrs.CacheControl, err = strconv.Unquote(f[1])
	if err != nil {
		goto BadCache
	}
	attrs.ContentType, err = strconv.Unquote(f[2])
	if err != nil {
		goto BadCache
	}
	attrs.MD5, err = hex.DecodeString(f[3])
	if err != nil {
		goto BadCache
	}
	attrs.Size, err = strconv.ParseInt(f[4], 10, 64)
	if err != nil {
		goto BadCache
	}
	attrs.Updated, err = time.Parse(time.RFC3339, f[5])
	if err != nil {
		goto BadCache
	}
	return attrs, err

BadCache:
	log.Errorf(ctx, "memcache.Get attrs: unexpected cache value: %d fields, %q", len(f), f[0])
	if !forceLookup {
		forceLookup = true
		goto Again
	}
	return nil, fmt.Errorf("bad cache value")
}

func lookupContent(ctx context.Context, client *storage.Client, attrs *storage.ObjectAttrs, bucketName, file string) ([]byte, error) {
	const (
		cachePrefix  = "www-content:"
		cacheVersion = "v1:"
	)
	key := cachePrefix + bucketName + "/" + file + ":" + fmt.Sprintf("%x", attrs.MD5)
	item, err := memcache.Get(ctx, key)
	if err != nil && err != memcache.ErrCacheMiss {
		log.Errorf(ctx, "memcache.Get content: %v", err)
	}
	if err == nil {
		log.Infof(ctx, "memcache.Get content hit: %v", key)
		if bytes.HasPrefix(item.Value, []byte(cacheVersion)) {
			return item.Value[len(cacheVersion):], nil
		}
		val := item.Value
		if len(val) > 16 {
			val = val[:16]
		}
		log.Errorf(ctx, "memcache.Get bad cache entry: %x", val)
	}

	log.Infof(ctx, "memcache.Get content refresh: %v", key)
	r, err := client.Bucket(bucketName).Object(file).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	item = &memcache.Item{
		Key:   key,
		Value: append([]byte(cacheVersion), data...),
	}
	if err := memcache.Set(ctx, item); err != nil {
		log.Errorf(ctx, "caching object content: %v", err)
	}

	return data, nil
}
