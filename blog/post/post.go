// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package post

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/appengine"
	"google.golang.org/appengine/blobstore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/memcache"
	userpkg "google.golang.org/appengine/user"
	"rsc.io/swtch/blog/atom"
)

const (
	myHost       = "research.swtch.com"
	bucketName   = "swtch"
	bucketPrefix = "blog"
)

func init() {
	http.HandleFunc("/", serve)
	http.Handle("/feeds/posts/default", http.RedirectHandler("/feed.atom", http.StatusFound))
}

var funcMap = template.FuncMap{
	"now":  time.Now,
	"date": timeFormat,
}

func timeFormat(fmt string, t time.Time) string {
	return t.Format(fmt)
}

type blogTime struct {
	time.Time
}

var timeFormats = []string{
	time.RFC3339,
	"Monday, January 2, 2006",
	"January 2, 2006 15:00 -0700",
}

func (t *blogTime) UnmarshalJSON(data []byte) (err error) {
	str := string(data)
	for _, f := range timeFormats {
		tt, err := time.Parse(`"`+f+`"`, str)
		if err == nil {
			t.Time = tt
			return nil
		}
	}
	return fmt.Errorf("did not recognize time: %s", str)
}

type PostData struct {
	FileModTime time.Time
	FileSize    int64

	Title    string
	Date     blogTime
	Name     string
	OldURL   string
	Summary  string
	Favorite bool

	Reader []string

	PlusAuthor string // Google+ ID of author
	PlusPage   string // Google+ Post ID for comment post
	PlusAPIKey string // Google+ API key
	PlusURL    string
	HostURL    string // host URL
	Comments   bool

	article string
}

func (d *PostData) canRead(user string) bool {
	for _, r := range d.Reader {
		if r == user {
			return true
		}
	}
	return false
}

func (d *PostData) IsDraft() bool {
	return d.Date.IsZero() || d.Date.After(time.Now())
}

// To find PlusPage value:
// https://www.googleapis.com/plus/v1/people/116810148281701144465/activities/public?key=AIzaSyB_JO6hyAJAL659z0Dmu0RUVVvTx02ZPMM
//

const owner = "rsc@swtch.com"
const plusRsc = "116810148281701144465"
const plusKey = "AIzaSyB_JO6hyAJAL659z0Dmu0RUVVvTx02ZPMM"
const feedID = "tag:research.swtch.com,2012:research.swtch.com"

var replacer = strings.NewReplacer(
	"⁰", "<sup>0</sup>",
	"¹", "<sup>1</sup>",
	"²", "<sup>2</sup>",
	"³", "<sup>3</sup>",
	"⁴", "<sup>4</sup>",
	"⁵", "<sup>5</sup>",
	"⁶", "<sup>6</sup>",
	"⁷", "<sup>7</sup>",
	"⁸", "<sup>8</sup>",
	"⁹", "<sup>9</sup>",
	"ⁿ", "<sup>n</sup>",
	"₀", "<sub>0</sub>",
	"₁", "<sub>1</sub>",
	"₂", "<sub>2</sub>",
	"₃", "<sub>3</sub>",
	"₄", "<sub>4</sub>",
	"₅", "<sub>5</sub>",
	"₆", "<sub>6</sub>",
	"₇", "<sub>7</sub>",
	"₈", "<sub>8</sub>",
	"₉", "<sub>9</sub>",
	"``", "&ldquo;",
	"''", "&rdquo;",
)

func serve(w http.ResponseWriter, req *http.Request) {
	ctx := appengine.NewContext(req)

	println("SERVE", req.URL.Path)

	defer func() {
		if err := recover(); err != nil {
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "panic: %s\n\n", err)
			buf.Write(debug.Stack())
			log.Criticalf(ctx, "%s", buf.String())

			http.Error(w, buf.String(), 500)
		}
	}()

	p := path.Clean("/" + req.URL.Path)
	/*
		if strings.Contains(req.Host, "appspot.com") {
			http.Redirect(w, req, "http://research.swtch.com" + p, http.StatusFound)
		}
	*/
	if p != req.URL.Path {
		http.Redirect(w, req, p, http.StatusFound)
		return
	}

	if p == "/feed.atom" {
		atomfeed(w, req)
		return
	}

	if strings.HasPrefix(p, "/20") && strings.Contains(p[1:], "/") {
		// Assume this is an old-style URL.
		oldRedirect(ctx, w, req, p)
		return
	}

	u := userpkg.Current(ctx)
	user := ""
	if u != nil {
		user = u.Email
	}
	isOwner := user == owner || len(os.Args) >= 2 && os.Args[1] == "LISTEN_STDIN"
	if p == "" || p == "/" || p == "/draft" {
		if p == "/draft" && user == "?" {
			log.Criticalf(ctx, "/draft loaded by %s", user)
			notfound(ctx, w, req)
			return
		}
		toc(w, req, p == "/draft", isOwner, user)
		return
	}

	draft := false
	if strings.HasPrefix(p, "/draft/") {
		if user == "?" {
			log.Criticalf(ctx, "/draft loaded by %s", user)
			notfound(ctx, w, req)
			return
		}
		draft = true
		p = p[len("/draft"):]
	}

	if strings.Contains(p[1:], "/") {
		notfound(ctx, w, req)
		return
	}

	if strings.Contains(p, ".") {
		// Let Google's front end servers cache static
		// content for a short amount of time.
		httpCache(w, 5*time.Minute)

		w.Header().Set("X-AppEngine-BlobRange", "bytes=0-")

		key, err := blobstore.BlobKeyForFile(ctx, "/gs/"+bucketName+p)
		if err != nil {
			log.Errorf(ctx, "blobstore.BlobKeyForFile: %v", err)
			http.Error(w, "problem loading file", http.StatusInternalServerError)
			return
		}
		blobstore.Send(w, key)
		return
	}

	// Use just 'blog' as the cache path so that if we change
	// templates, all the cached HTML gets invalidated.
	var data []byte
	pp := "bloghtml:" + p
	if draft && !isOwner {
		pp += ",user=" + user
	}
	if key, ok := cacheLoad(ctx, pp, "blog", &data); !ok {
		meta, article, err := loadPost(ctx, p, req)
		if err != nil || meta.IsDraft() != draft || (draft && !isOwner && !meta.canRead(user)) {
			log.Criticalf(ctx, "no %s for %s", p, user)
			notfound(ctx, w, req)
			return
		}
		t := mainTemplate(ctx)
		template.Must(t.New("article").Parse(article))

		var buf bytes.Buffer
		meta.Comments = true
		if err := t.Execute(&buf, meta); err != nil {
			panic(err)
		}
		data = buf.Bytes()
		cacheStore(ctx, key, data)
	}
	w.Write(data)
}

func notfound(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	var buf bytes.Buffer
	var data struct {
		HostURL string
	}
	data.HostURL = hostURL(req)
	t := mainTemplate(ctx)
	if err := t.Lookup("404").Execute(&buf, &data); err != nil {
		panic(err)
	}
	w.WriteHeader(404)
	w.Write(buf.Bytes())
}

func mainTemplate(c context.Context) *template.Template {
	t := template.New("main")
	t.Funcs(funcMap)

	main, _, err := readFile(c, "blog/main.html")
	if err != nil {
		panic(err)
	}
	style, _, _ := readFile(c, "blog/style.html")
	main = append(main, style...)
	_, err = t.Parse(string(main))
	if err != nil {
		panic(err)
	}
	return t
}

func loadPost(c context.Context, name string, req *http.Request) (meta *PostData, article string, err error) {
	meta = &PostData{
		Name:       name,
		Title:      "TITLE HERE",
		PlusAuthor: plusRsc,
		PlusAPIKey: plusKey,
		HostURL:    hostURL(req),
	}

	art, attrs, err := readFile(c, "blog/post/"+name)
	if err != nil {
		return nil, "", err
	}
	if bytes.HasPrefix(art, []byte("{\n")) {
		i := bytes.Index(art, []byte("\n}\n"))
		if i < 0 {
			panic("cannot find end of json metadata")
		}
		hdr, rest := art[:i+3], art[i+3:]
		if err := json.Unmarshal(hdr, meta); err != nil {
			panic(fmt.Sprintf("loading %s: %s", name, err))
		}
		art = rest
	}
	meta.FileModTime = attrs.Updated
	meta.FileSize = attrs.Size

	return meta, replacer.Replace(string(art)), nil
}

type byTime []*PostData

func (x byTime) Len() int           { return len(x) }
func (x byTime) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
func (x byTime) Less(i, j int) bool { return x[i].Date.Time.After(x[j].Date.Time) }

type TocData struct {
	Draft   bool
	HostURL string
	Posts   []*PostData
}

func toc(w http.ResponseWriter, req *http.Request, draft bool, isOwner bool, user string) {
	ctx := appengine.NewContext(req)

	var data []byte
	keystr := fmt.Sprintf("blog:toc:%v", draft)
	if req.FormValue("readdir") != "" {
		keystr += ",readdir=" + req.FormValue("readdir")
	}
	if draft {
		keystr += ",user=" + user
	}

	if key, ok := cacheLoad(ctx, keystr, "blog", &data); !ok {
		dir, err := readDir(ctx, "blog/post")
		if err != nil {
			panic(err)
		}

		if req.FormValue("readdir") == "1" {
			fmt.Fprintf(w, "%d dir entries\n", len(dir))
			return
		}

		postCache := map[string]*PostData{}
		if data, _, err := readFile(ctx, "blogcache"); err == nil {
			if err := json.Unmarshal(data, &postCache); err != nil {
				log.Criticalf(ctx, "unmarshal blogcache: %v", err)
			}
		}

		ch := make(chan *PostData, len(dir))
		const par = 1
		var limit = make(chan bool, par)
		for i := 0; i < par; i++ {
			limit <- true
		}
		for _, d := range dir {
			if meta := postCache[d.Name]; meta != nil && meta.FileModTime.Equal(d.Updated) && meta.FileSize == d.Size {
				ch <- meta
				continue
			}

			<-limit
			go func(d *storage.ObjectAttrs) {
				defer func() { limit <- true }()
				meta, _, err := loadPost(ctx, d.Name, req)
				if err != nil {
					// Should not happen: we just listed the directory.
					log.Criticalf(ctx, "loadPost %s: %v", d.Name, err)
					return
				}
				ch <- meta
			}(d)
		}
		for i := 0; i < par; i++ {
			<-limit
		}
		close(ch)
		postCache = map[string]*PostData{}
		var all []*PostData
		for meta := range ch {
			postCache[meta.Name] = meta
			if meta.IsDraft() == draft && (!draft || isOwner || meta.canRead(user)) {
				all = append(all, meta)
			}
		}
		sort.Sort(byTime(all))

		if data, err := json.Marshal(postCache); err != nil {
			log.Criticalf(ctx, "marshal blogcache: %v", err)
		} else if err := writeFile(ctx, "blogcache", data); err != nil {
			log.Criticalf(ctx, "write blogcache: %v", err)
		}

		var buf bytes.Buffer
		t := mainTemplate(ctx)
		if err := t.Lookup("toc").Execute(&buf, &TocData{draft, hostURL(req), all}); err != nil {
			panic(err)
		}
		data = buf.Bytes()
		cacheStore(ctx, key, data)
	}
	w.Write(data)
}

func oldRedirect(ctx context.Context, w http.ResponseWriter, req *http.Request, p string) {
	m := map[string]string{}
	if key, ok := cacheLoad(ctx, "blog:oldRedirectMap", "blog/post", &m); !ok {
		dir, err := readDir(ctx, "blog/post")
		if err != nil {
			panic(err)
		}

		for _, d := range dir {
			meta, _, err := loadPost(ctx, d.Name, req)
			if err != nil {
				// Should not happen: we just listed the directory.
				panic(err)
			}
			m[meta.OldURL] = "/" + d.Name
		}

		cacheStore(ctx, key, m)
	}

	if url, ok := m[p]; ok {
		http.Redirect(w, req, url, http.StatusFound)
		return
	}

	notfound(ctx, w, req)
}

func hostURL(req *http.Request) string {
	if strings.HasPrefix(req.Host, "localhost") {
		return "http://localhost:8080"
	}
	return "http://research.swtch.com"
}

func atomfeed(w http.ResponseWriter, req *http.Request) {
	ctx := appengine.NewContext(req)

	log.Criticalf(ctx, "Header: %v", req.Header)

	var data []byte
	if key, ok := cacheLoad(ctx, "blog:atomfeed", "blog/post", &data); !ok {
		dir, err := readDir(ctx, "blog/post")
		if err != nil {
			panic(err)
		}

		var all []*PostData
		for _, d := range dir {
			meta, article, err := loadPost(ctx, d.Name, req)
			if err != nil {
				// Should not happen: we just loaded the directory.
				panic(err)
			}
			if meta.IsDraft() {
				continue
			}
			meta.article = article
			all = append(all, meta)
		}
		sort.Sort(byTime(all))

		show := all
		if len(show) > 10 {
			show = show[:10]
			for _, meta := range all[10:] {
				if meta.Favorite {
					show = append(show, meta)
				}
			}
		}

		feed := &atom.Feed{
			Title:   "research!rsc",
			ID:      feedID,
			Updated: atom.Time(show[0].Date.Time),
			Author: &atom.Person{
				Name:  "Russ Cox",
				URI:   "https://plus.google.com/" + plusRsc,
				Email: "rsc@swtch.com",
			},
			Link: []atom.Link{
				{Rel: "self", Href: hostURL(req) + "/feed.atom"},
			},
		}

		for _, meta := range show {
			t := template.New("main")
			t.Funcs(funcMap)
			main, _, err := readFile(ctx, "blog/atom.html")
			if err != nil {
				panic(err)
			}
			_, err = t.Parse(string(main))
			if err != nil {
				panic(err)
			}
			template.Must(t.New("article").Parse(meta.article))
			var buf bytes.Buffer
			if err := t.Execute(&buf, meta); err != nil {
				panic(err)
			}

			e := &atom.Entry{
				Title: meta.Title,
				ID:    feed.ID + "/" + meta.Name,
				Link: []atom.Link{
					{Rel: "alternate", Href: meta.HostURL + "/" + meta.Name},
				},
				Published: atom.Time(meta.Date.Time),
				Updated:   atom.Time(meta.Date.Time),
				Summary: &atom.Text{
					Type: "text",
					Body: meta.Summary,
				},
				Content: &atom.Text{
					Type: "html",
					Body: buf.String(),
				},
			}

			feed.Entry = append(feed.Entry, e)
		}

		data, err = xml.Marshal(&feed)
		if err != nil {
			panic(err)
		}

		cacheStore(ctx, key, data)
	}

	// Feed readers like to hammer us; let Google cache the
	// response to reduce the traffic we have to serve.
	httpCache(w, 15*time.Minute)

	w.Header().Set("Content-Type", "application/atom+xml")
	w.Write(data)
}

func httpCache(w http.ResponseWriter, dt time.Duration) {
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(dt.Seconds())))
}

func readDir(ctx context.Context, name string) ([]*storage.ObjectAttrs, error) {
	hc, err := google.DefaultClient(ctx,
		"https://www.googleapis.com/auth/appengine.apis",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		return nil, err
	}
	client, err := storage.NewClient(ctx, option.WithHTTPClient(hc))
	if err != nil {
		log.Errorf(ctx, "failed to create client: %v", err)
		return nil, err
	}

	it := client.Bucket(bucketName).Objects(ctx, &storage.Query{
		Delimiter: "/",
		Prefix:    name + "/",
	})
	var list []*storage.ObjectAttrs
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Errorf(ctx, "reading directory %v: %v", name, err)
			return nil, err
		}
		attrs.Name = strings.TrimPrefix(attrs.Name, name+"/")
		list = append(list, attrs)
	}
	return list, nil
}

func readFile(ctx context.Context, file string) (data []byte, attrs *storage.ObjectAttrs, err error) {
	hc, err := google.DefaultClient(ctx,
		"https://www.googleapis.com/auth/appengine.apis",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		return nil, nil, err
	}
	file = path.Clean(file)
	client, err := storage.NewClient(ctx, option.WithHTTPClient(hc))
	if err != nil {
		log.Errorf(ctx, "failed to create client: %v", err)
		return nil, nil, err
	}

	attrs, err = client.Bucket(bucketName).Object(file).Attrs(ctx)
	if err != nil && err != storage.ErrObjectNotExist {
		log.Errorf(ctx, "reading object attrs for %v: %v", file, err)
	}
	if err != nil {
		log.Errorf(ctx, "reading object attrs for %v: %v", file, err)
		return nil, nil, err
	}

	r, err := client.Bucket(bucketName).Object(file).NewReader(ctx)
	if err != nil {
		log.Errorf(ctx, "reading %v: %v", file, err)
		return nil, nil, err
	}
	defer r.Close()
	data, err = ioutil.ReadAll(r)
	if err != nil {
		log.Errorf(ctx, "reading %v: %v", file, err)
		return nil, nil, err
	}

	return data, attrs, nil
}

func writeFile(ctx context.Context, file string, data []byte) error {
	file = path.Clean(file)
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Errorf(ctx, "failed to create client: %v", err)
		return err
	}

	w := client.Bucket(bucketName).Object(file).NewWriter(ctx)
	if _, err := w.Write(data); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return nil
}

func cacheLoad(ctx context.Context, key, file string, value interface{}) (xkey string, ok bool) {
	file = path.Clean(file)
	item, err := memcache.Get(ctx, key)
	if err != nil && err != memcache.ErrCacheMiss {
		log.Errorf(ctx, "memcache.Get content: %v", err)
	}
	if err != nil {
		return key, false
	}
	if err := gob.NewDecoder(bytes.NewBuffer(item.Value)).Decode(value); err != nil {
		log.Criticalf(ctx, "gob Decode: %v", err)
		return key, false
	}
	return key, true
}

func cacheStore(ctx context.Context, key string, value interface{}) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(value); err != nil {
		log.Criticalf(ctx, "gob Encode: %v", err)
		return
	}
	item := &memcache.Item{
		Key:   key,
		Value: buf.Bytes(),
	}
	if err := memcache.Set(ctx, item); err != nil {
		log.Errorf(ctx, "caching object content: %v", err)
	}
}
