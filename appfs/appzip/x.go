package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"rsc.io/swtch/appfs/proto"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/remote_api"
)

var zw *zip.Writer
var remoteCtx context.Context

func main() {
	const host = "remote-api-server-dot-rsc-swtch-app.appspot.com"

	ctx := context.Background()

	hc, err := google.DefaultClient(ctx,
		"https://www.googleapis.com/auth/appengine.apis",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		log.Fatal(err)
	}

	remoteCtx, err = remote_api.NewRemoteContext(host, hc)
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create("app.zip")
	if err != nil {
		log.Fatal(err)
	}

	zw = zip.NewWriter(f)

	walk("/")

	err = zw.Close()
	if err != nil {
		log.Fatal(err)
	}
}

func walk(dir string) {
	println("DIR", dir)
	if dir == "/qr/flag" || dir == "/qr/upload" || dir == "/qrsave" {
		return
	}
	files, err := ae{}.ReadDir(remoteCtx, dir)
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		name := path.Join(dir, file.Name)
		if file.IsDir {
			walk(name)
			continue
		}
		println("FILE", name)
		data, _, err := ae{}.Read(remoteCtx, name)
		if err != nil {
			log.Fatal(err)
		}
		h := &zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		}
		h.SetModTime(file.ModTime)

		f, err := zw.CreateHeader(h)
		if err != nil {
			log.Fatal(err)
		}
		_, err = f.Write(data)
		if err != nil {
			log.Fatal(err)
		}
	}
}

type request struct {
	w     http.ResponseWriter
	req   *http.Request
	c     context.Context
	name  string
	mname string
	key   *datastore.Key
}

func mangle(c context.Context, name string) (string, string, *datastore.Key) {
	name = path.Clean("/" + name)
	n := strings.Count(name, "/")
	if name == "/" {
		n = 0
	}
	mname := fmt.Sprintf("%d%s", n, name)
	root := datastore.NewKey(c, "RootKey", "v2:", 0, nil)
	key := datastore.NewKey(c, "FileInfo", mname, 0, root)
	return name, mname, key
}

type FileInfo struct {
	Path    string // mangled path
	Name    string
	Qid     int64 // assigned unique id number
	Seq     int64 // modification sequence number in file tree
	ModTime time.Time
	Size    int64
	IsDir   bool
}

type FileData struct {
	Data []byte
}

func stat(c context.Context, name string) (*FileInfo, error) {
	var fi FileInfo
	name, _, key := mangle(c, name)
	err := datastore.Get(c, key, &fi)
	if err != nil {
		return nil, err
	}
	return &fi, nil
}

func (r *request) saveStat(fi *FileInfo) {
	jfi, err := json.Marshal(&fi)
	if err != nil {
		panic(err)
	}
	r.w.Header().Set("X-Appfs-Stat", string(jfi))
}

func (r *request) tx(f func(c context.Context) error) {
	err := f(r.c) // datastore.RunInTransaction(r.c, f, &datastore.TransactionOptions{XG: true})
	if err != nil {
		panic(err)
	}
}

func (r *request) stat() {
	var fi *FileInfo
	r.tx(func(c context.Context) error {
		fi1, err := stat(c, r.name)
		if err != nil {
			return err
		}
		fi = fi1
		return nil
	})

	jfi, err := json.Marshal(&fi)
	if err != nil {
		panic(err)
	}
	r.w.Write(jfi)
}

func read(c context.Context, name string) (fi *FileInfo, data []byte, err error) {
	name, _, _ = mangle(c, name)
	fi1, err := stat(c, name)
	if err != nil {
		return nil, nil, err
	}
	if fi1.IsDir {
		dt, err := readdir(c, name)
		if err != nil {
			return nil, nil, err
		}
		fi = fi1
		data = dt
		return fi, data, nil
	}

	root := datastore.NewKey(c, "RootKey", "v2:", 0, nil)
	dkey := datastore.NewKey(c, "FileData", "", fi1.Qid, root)
	var fd FileData
	if err := datastore.Get(c, dkey, &fd); err != nil {
		return nil, nil, err
	}
	fi = fi1
	data = fd.Data
	return fi, data, nil
}

func (r *request) read() {
	var (
		fi   *FileInfo
		data []byte
	)
	r.tx(func(c context.Context) error {
		var err error
		fi, data, err = read(r.c, r.name)
		return err
	})
	r.saveStat(fi)
	r.w.Write(data)
}

func readdir(c context.Context, name string) ([]byte, error) {
	name, _, _ = mangle(c, name)
	var buf bytes.Buffer

	n := strings.Count(name, "/")
	if name == "/" {
		name = ""
		n = 0
	}
	root := datastore.NewKey(c, "RootKey", "v2:", 0, nil)
	first := fmt.Sprintf("%d%s/", n+1, name)
	limit := fmt.Sprintf("%d%s0", n+1, name)
	q := datastore.NewQuery("FileInfo").
		Filter("Path >=", first).
		Filter("Path <", limit).
		Ancestor(root)
	enc := json.NewEncoder(&buf)
	it := q.Run(c)
	var fi FileInfo
	var pfi proto.FileInfo
	for {
		fi = FileInfo{}
		_, err := it.Next(&fi)
		if err != nil {
			if err == datastore.Done {
				break
			}
			fmt.Fprintf(os.Stderr, "DS: %v\n", err)
			return nil, err
		}
		pfi = proto.FileInfo{
			Name:    fi.Name,
			ModTime: fi.ModTime,
			Size:    fi.Size,
			IsDir:   fi.IsDir,
		}
		if err := enc.Encode(&pfi); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func readdirRaw(c context.Context, name string) ([]proto.FileInfo, error) {
	name, _, _ = mangle(c, name)
	n := strings.Count(name, "/")
	if name == "/" {
		name = ""
		n = 0
	}
	root := datastore.NewKey(c, "RootKey", "v2:", 0, nil)
	first := fmt.Sprintf("%d%s/", n+1, name)
	limit := fmt.Sprintf("%d%s0", n+1, name)
	q := datastore.NewQuery("FileInfo").
		Filter("Path >=", first).
		Filter("Path <", limit).
		Ancestor(root).
		Limit(1000)
	it := q.Run(c)
	var fi FileInfo
	var pfi proto.FileInfo
	var out []proto.FileInfo
	for {
		fi = FileInfo{}
		_, err := it.Next(&fi)
		if err != nil {
			if err == datastore.Done {
				break
			}
			fmt.Fprintf(os.Stderr, "DS: %v\n", err)
			return nil, err
		}
		pfi = proto.FileInfo{
			Name:    fi.Name,
			ModTime: fi.ModTime,
			Size:    fi.Size,
			IsDir:   fi.IsDir,
		}
		out = append(out, pfi)
	}
	return out, nil
}

func (r *request) write() {
	data, err := ioutil.ReadAll(r.req.Body)
	if err != nil {
		panic(err)
	}

	var fi *FileInfo
	var seq int64
	r.tx(func(c context.Context) error {
		var err error
		fi, seq, err = write(r.c, r.name, data)
		return err
	})
	updateCacheTime(r.c, seq)
	r.saveStat(fi)
}

func write(c context.Context, name string, data []byte) (*FileInfo, int64, error) {
	name, _, key := mangle(c, name)

	// Check that file exists and is not a directory.
	fi1, err := stat(c, name)
	if err != nil {
		return nil, 0, err
	}
	if fi1.IsDir {
		return nil, 0, fmt.Errorf("cannot write to directory")
	}

	// Fetch and increment root sequence number.
	rfi, err := stat(c, "/")
	if err != nil {
		return nil, 0, err
	}
	rfi.Seq++

	// Write data.
	root := datastore.NewKey(c, "RootKey", "v2:", 0, nil)
	dkey := datastore.NewKey(c, "FileData", "", fi1.Qid, root)
	fd := &FileData{data}
	if _, err := datastore.Put(c, dkey, fd); err != nil {
		return nil, 0, err
	}

	// Update directory entry.
	fi1.Seq = rfi.Seq
	fi1.Size = int64(len(data))
	fi1.ModTime = time.Now()
	if _, err := datastore.Put(c, key, fi1); err != nil {
		return nil, 0, err
	}

	// Update sequence numbers all the way to the root.
	if err := updateSeq(c, name, rfi.Seq, 1); err != nil {
		return nil, 0, err
	}

	return fi1, rfi.Seq, nil
}

func updateSeq(c context.Context, name string, seq int64, skip int) error {
	p := path.Clean(name)
	for i := 0; ; i++ {
		if i >= skip {
			_, _, key := mangle(c, p)
			var fi FileInfo
			if err := datastore.Get(c, key, &fi); err != nil {
				return err
			}
			fi.Seq = seq
			if _, err := datastore.Put(c, key, &fi); err != nil {
				return err
			}
		}
		if p == "/" {
			break
		}
		p, _ = path.Split(p)
		p = path.Clean(p)
	}
	return nil
}

func (r *request) remove() {
	panic("remove not implemented")
}

func (r *request) create() {
	var fi *FileInfo
	var seq int64
	isDir := r.req.FormValue("dir") == "1"
	r.tx(func(c context.Context) error {
		var err error
		fi, seq, err = create(r.c, r.name, isDir, nil)
		return err
	})
	updateCacheTime(r.c, seq)
	r.saveStat(fi)
}

func create(c context.Context, name string, isDir bool, data []byte) (*FileInfo, int64, error) {
	name, mname, key := mangle(c, name)

	// File must not exist.
	fi1, err := stat(c, name)
	if err == nil {
		return nil, 0, fmt.Errorf("file already exists")
	}
	if err != datastore.ErrNoSuchEntity {
		return nil, 0, err
	}

	// Parent must exist and be a directory.
	p, _ := path.Split(name)
	fi2, err := stat(c, p)
	if err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, 0, fmt.Errorf("parent directory %q does not exist", p)
		}
		return nil, 0, err
	}
	if !fi2.IsDir {
		return nil, 0, fmt.Errorf("parent %q is not a directory", p)
	}

	// Fetch and increment root sequence number.
	rfi, err := stat(c, "/")
	if err != nil {
		return nil, 0, err
	}
	rfi.Seq++

	var dataKey int64
	// Create data object.
	if !isDir {
		dataKey = rfi.Seq
		root := datastore.NewKey(c, "RootKey", "v2:", 0, nil)
		dkey := datastore.NewKey(c, "FileData", "", dataKey, root)
		_, err := datastore.Put(c, dkey, &FileData{data})
		if err != nil {
			return nil, 0, err
		}
	}

	// Create new directory entry.
	_, elem := path.Split(name)
	fi1 = &FileInfo{
		Path:    mname,
		Name:    elem,
		Qid:     rfi.Seq,
		Seq:     rfi.Seq,
		ModTime: time.Now(),
		Size:    int64(len(data)),
		IsDir:   isDir,
	}
	if _, err := datastore.Put(c, key, fi1); err != nil {
		return nil, 0, err
	}

	// Update sequence numbers all the way to root,
	// but skip entry we just wrote.
	if err := updateSeq(c, name, rfi.Seq, 1); err != nil {
		return nil, 0, err
	}

	return fi1, rfi.Seq, nil
}

// Implementation of fs.AppEngine.

type ae struct{}

func tx(c interface{}, f func(c context.Context) error) error {
	return datastore.RunInTransaction(c.(context.Context), f, &datastore.TransactionOptions{XG: true})
}

func (ae) NewContext(req *http.Request) interface{} {
	return appengine.NewContext(req)
}

type cacheKey struct {
	t    int64
	name string
}

func (ae) CacheRead(ctxt interface{}, name, path string) (key interface{}, data []byte, found bool) {
	c := ctxt.(context.Context)
	t, data, _, err := cacheRead(c, "cache", name, path)
	return &cacheKey{t, name}, data, err == nil
}

func (ae) CacheWrite(ctxt, key interface{}, data []byte) {
	return

	c := ctxt.(context.Context)
	k := key.(*cacheKey)
	cacheWrite(c, k.t, "cache", k.name, data)
}

func (ae ae) Read(ctxt interface{}, name string) (data []byte, pfi *proto.FileInfo, err error) {
	c := ctxt.(context.Context)
	name = path.Clean("/" + name)
	_, data, pfi, err = cacheRead(c, "data", name, name)
	if err != nil {
		err = fmt.Errorf("Read %q: %v", name, err)
	}
	return
}

func (ae) Write(ctxt interface{}, path string, data []byte) error {
	var seq int64
	err := tx(ctxt, func(c context.Context) error {
		_, err := stat(c, path)
		if err != nil {
			_, seq, err = create(c, path, false, data)
		} else {
			_, seq, err = write(c, path, data)
		}
		return err
	})
	if seq != 0 {
		updateCacheTime(ctxt.(context.Context), seq)
	}
	if err != nil {
		err = fmt.Errorf("Write %q: %v", path, err)
	}
	return err
}

func (ae) Remove(ctxt interface{}, path string) error {
	return fmt.Errorf("remove not implemented")
}

func (ae) Mkdir(ctxt interface{}, path string) error {
	var seq int64
	err := tx(ctxt, func(c context.Context) error {
		var err error
		_, seq, err = create(c, path, true, nil)
		return err
	})
	if seq != 0 {
		updateCacheTime(ctxt.(context.Context), seq)
	}
	if err != nil {
		err = fmt.Errorf("Mkdir %q: %v", path, err)
	}
	return err
}

func (ae) Criticalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

type readDirCacheEntry struct {
	Dir   []proto.FileInfo
	Error string
}

func (ae) ReadDir(ctxt context.Context, name string) (dir []proto.FileInfo, err error) {
	name = path.Clean("/" + name)
	dir, err = readdirRaw(ctxt, name)
	if err != nil {
		err = fmt.Errorf("ReadDir %q: %v", name, err)
	}
	return
}

// Caching of file system data.
//
// The cache stores entries under keys of the form time,space,name,
// where time is the time at which the entry is valid for, space is a name
// space identifier, and name is an arbitrary name.
//
// A key of the form t,mtime,path maps to an integer value giving the
// modification time of the named path at root time t.
// The special key 0,mtime,/ is an integer giving the current time at the root.
//
// A key of the form t,data,path maps to the content of path at time t.
//
// Thus, a read from path should first obtain the root time,
// then obtain the modification time for the path at that root time
// then obtain the data for that path.
//	t1 = get(0,mtime,/)
//	t2 = get(t1,mtime,path)
//	data = get(t2,data,path)
//
// The API allows clients to cache their own data too, with expiry tied to
// the modification time of a particular path (file or directory).  To look
// up one of those, we use:
//	t1 = get(0,mtime,/)
//	t2 = get(t1,mtime,path)
//	data = get(t2,clientdata,name)
//
// To store data in the cache, the t1, t2 should be determined before reading
// from datastore.  Then the data should be saved under t2.  This ensures
// that if a datastore update happens after the read but before the cache write,
// we'll be writing to an entry that will no longer be used (t2).

const rootMemcacheKey = "0,mtime,/"

func updateCacheTime(c context.Context, seq int64) {
	const key = rootMemcacheKey
	bseq := []byte(strconv.FormatInt(seq, 10))
	for tries := 0; tries < 10; tries++ {
		item, err := memcache.Get(c, key)
		if err != nil {
			err = memcache.Add(c, &memcache.Item{Key: key, Value: bseq})
			if err == nil {
				return
			}
		}
		v, err := strconv.ParseInt(string(item.Value), 10, 64)
		if err != nil {
			ae{}.Criticalf("memcache.Get %q = %q (%v)", key, item.Value, err)
			return
		}
		if v >= seq {
			return
		}
		item.Value = bseq
		err = memcache.CompareAndSwap(c, item)
		if err == nil {
			return
		}
	}
	ae{}.Criticalf("repeatedly failed to update root key")
}

type statCacheEntry struct {
	FileInfo *proto.FileInfo
	Error    string
}

func cacheRead(c context.Context, kind, name, path string) (mtime int64, data []byte, pfi *proto.FileInfo, err error) {
	var t int64

	// Need stat, or maybe stat+data.
	var fi *FileInfo
	fi, data, err = read(c, name)
	if err == nil && fi.Seq != t {
		t = fi.Seq
	}

	// Cache stat, including error.
	st := statCacheEntry{}
	if fi != nil {
		st.FileInfo = &proto.FileInfo{
			Name:    fi.Name,
			ModTime: fi.ModTime,
			Size:    fi.Size,
			IsDir:   fi.IsDir,
		}
	}
	if err != nil {
		st.Error = err.Error()
		// If this is a deadline exceeded, do not cache.
		if strings.Contains(st.Error, "Canceled") || strings.Contains(st.Error, "Deadline") {
			return t, data, st.FileInfo, err
		}
	}

	// Done!
	return t, data, st.FileInfo, err
}

func cacheWrite(c context.Context, t int64, kind, name string, data []byte) error {
	return nil
	mkey := fmt.Sprintf("%d,%s,%s", t, kind, name)
	err := memcache.Set(c, &memcache.Item{Key: mkey, Value: data})
	if err != nil {
		ae{}.Criticalf("cacheWrite memcache.Set %q: %v", mkey, err)
	}
	return err
}
