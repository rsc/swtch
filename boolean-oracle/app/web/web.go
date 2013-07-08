// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package web

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Info struct {
	Record
	Size int
}

var info []Info
var xorInfo []Info
var fatalErr error

var fs = http.FileServer(http.Dir("static"))

func init() {
	http.HandleFunc("/", main)
	http.HandleFunc("/about", aboutHandler)
	http.HandleFunc("/result", resultHandler)
	http.HandleFunc("/debug", debug)
}

func load() {
	info = loadInfo("a056287.5.28.raw")
	xorInfo = loadInfo("xor.a056287.5.12.raw")
}

var once sync.Once

func debug(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(blog.Bytes())
}

type byF []Info

func (v byF) Len() int           { return len(v) }
func (v byF) Swap(i, j int)      { v[i], v[j] = v[j], v[i] }
func (v byF) Less(i, j int) bool { return v[i].F < v[j].F }

func loadInfo(name string) []Info {
	var state Savepoint
	rawUnmarshal(name, &state)
	info := make([]Info, len(state.Howto))
	for i := range info {
		info[i].Record = state.Howto[i]
	}
	n := 0
	for size, ff := range state.BySize {
		for _, f := range ff {
			if info[n].F != f {
				panic("inconsistent state")
			}
			info[n].Size = size
			n++
		}
	}
	sort.Sort(byF(info))
	return info
}

func dumpInfo(info []Info) {
	w := bufio.NewWriter(os.Stdout)
	for _, inf := range info {
		var op string
		switch {
		case inf.F == inf.P:
			fmt.Fprintf(w, "%v = literal (size %d)\n", inf.F, inf.Size)
			continue
		case inf.F == inf.P&inf.Q:
			op = "&"
		case inf.F == inf.P|inf.Q:
			op = "|"
		case inf.F == inf.P^inf.Q:
			op = "^"
		case inf.F == inf.P^inf.Q^(NumFunc-1):
			op = "^^"
		default:
			fmt.Fprintf(os.Stderr, "unknown op: %v = %v ? %v\n", inf.F, inf.P, inf.Q)
			op = "?"
		}
		fmt.Fprintf(w, "%v = %v %s %v (size %d)\n", inf.F, inf.P, op, inf.Q, inf.Size)
	}
	w.Flush()
}

var V = literal(0)
var W = literal(1)
var X = literal(2)
var Y = literal(3)
var Z = literal(4)

type MainData struct {
	Query  string
	Result template.HTML
	Mem    uint64
}

type ResultData struct {
	Error   error
	Query   string
	Canon   Func
	Func    Func
	Tree    *Tree
	XorTree *Tree
}

func run(w io.Writer, file string, data interface{}) {
	t, err := template.New("").Delims("«", "»").ParseFiles(file)
	if err != nil {
		panic(fmt.Errorf("Parsing %s: %s\n", file, err))
		return
	}
	if err := t.Lookup(file).Execute(w, data); err != nil {
		fmt.Fprintf(w, "\n\nError executing template %s: %s", file, err)
	}
}

func aboutHandler(w http.ResponseWriter, req *http.Request) {
	run(w, "about.html", nil)
}

func main(w http.ResponseWriter, req *http.Request) {
	if h := req.Host; strings.HasPrefix(h, "boolean-oracle.appspot.com") {
		h = req.Host[:len(h)-len("appspot.com")] + "swtch.com"
		http.Redirect(w, req, "http://"+h+req.RequestURI, 302)
	}
	if req.URL.Path != "/" {
		fs.ServeHTTP(w, req)
		return
	}
	defer func() {
		if err := recover(); err != nil {
			stk := make([]byte, 5000)
			n := runtime.Stack(stk, false)
			stk = stk[:n]
			fmt.Fprintf(w, "Failure: %v\n%s\n", err, stk)
		}
	}()

	q := req.FormValue("q")
	data := &MainData{Query: q, Mem: 0}
	if q != "" {
		data.Result = template.HTML(result(q))
	}
	run(w, "main.html", data)
}

func resultHandler(w http.ResponseWriter, req *http.Request) {
	q := req.FormValue("q")
	if q != "" {
		w.Write(result(q))
	}
}

func result(q string) []byte {
	var b bytes.Buffer
	run(&b, "result.html", resultData(q))
	return b.Bytes()
}

func resultData(q string) (res ResultData) {
	once.Do(load)
	res.Query = q
	var fb Func
	if n, err := strconv.ParseUint(q, 0, 64); err == nil {
		fb = Func(n)
	} else {
		f, err := parse(q)
		if err != nil {
			res.Error = fmt.Errorf("Error parsing query: %s", err)
			return
		}

		fp := func(val []int) int {
			defer func() {
				recover()
			}()
			return f(val)
		}

		v := make([]int, NumVar)
		for j := 0; j < NumInput; j++ {
			for k := 0; k < NumVar; k++ {
				v[k] = (j >> uint(k)) & 1
			}
			if fp(v) != 0 {
				fb |= 1 << uint(j)
			}
		}
	}
	res.Func = fb
	res.Canon = findMin(fb)
	res.Tree = findTree(fb, info)
	res.XorTree = findTree(fb, xorInfo)
	return
}

func (t *Tree) HTML() template.HTML {
	s := t.String()
	s = strings.Replace(s, "(", `«`, -1)
	s = strings.Replace(s, ")", `»`, -1)
	s = replaceBracket(s, "(", ")")
	s = replaceBracket(s, "<font size=+1>(</font>", "<font size=+1>)</font>")
	s = replaceBracket(s, "<font size=+2>(</font>", "<font size=+2>)</font>")
	s = replaceBracket(s, "<font size=+3>(</font>", "<font size=+3>)</font>")
	s = replaceBracket(s, "<font size=+4>(</font>", "<font size=+4>)</font>")
	s = replaceBracket(s, "<font size=+5>(</font>", "<font size=+5>)</font>")
	s = overline(s)
	return template.HTML(s)
}

func esc(s string) string {
	s = strings.Replace(s, "&", "&amp;", -1)
	s = strings.Replace(s, "<", "&lt;", -1)
	s = strings.Replace(s, ">", "&gt;", -1)
	s = strings.Replace(s, "'", "&apos;", -1)
	s = strings.Replace(s, "\"", "&quot;", -1)
	return s
}

func replaceBracket(s, l, r string) string {
	var b bytes.Buffer
	wrote := 0
	left := -1
	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(s[i:], `«`) {
			left = i
		}
		if strings.HasPrefix(s[i:], `»`) {
			if left >= 0 {
				b.WriteString(s[wrote:left])
				b.WriteString(l)
				b.WriteString(s[left+len(`«`) : i])
				b.WriteString(r)
				wrote = i + len(`»`)
			}
			left = -1
		}
	}
	b.WriteString(s[wrote:])
	return b.String()
}

func overline(s string) string {
	var b bytes.Buffer
	wrote := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '!' {
			b.WriteString(s[wrote:i])
			b.WriteString("¬")
			b.WriteString(s[i+1 : i+2])
			wrote = i + 2
		}
	}
	b.WriteString(s[wrote:])
	return b.String()
}

type Tree struct {
	Op   string
	F    Func
	L, R *Tree
}

func (t *Tree) String() string {
	if t.Op == "Lit" {
		switch t.F {
		case V:
			return "v"
		case V ^ (NumFunc - 1):
			return "!v"
		case W:
			return "w"
		case W ^ (NumFunc - 1):
			return "!w"
		case X:
			return "x"
		case X ^ (NumFunc - 1):
			return "!x"
		case Y:
			return "y"
		case Y ^ (NumFunc - 1):
			return "!y"
		case Z:
			return "z"
		case Z ^ (NumFunc - 1):
			return "!z"
		}
		return t.F.String()
	}
	l := t.L.String()
	if t.L.Op != "Lit" && t.L.Op != t.Op {
		l = "(" + l + ")"
	}
	r := t.R.String()
	if t.R.Op != "Lit" && t.R.Op != t.Op {
		r = "(" + r + ")"
	}
	op := t.Op
	op = " " + op + " "
	return l + op + r
}

func (t *Tree) complexity() int {
	if t.Op == "Lit" {
		return 0
	}
	return t.L.complexity() + 1 + t.R.complexity()
}

func (t *Tree) Complexity() string {
	n := t.complexity()
	if n == 1 {
		return "1 operator"
	}
	return fmt.Sprintf("%d operators", n)
}

func findTree(f Func, info []Info) *Tree {
	m := findMin(f)
	inf := info[sort.Search(len(info), func(i int) bool { return info[i].F >= m })]

	f0 := f
	p := Func(0)
	q := Func(0)
	op := ""
	for _, i := range grayBit[0:NumInput] {
		m, s := invert[i].mask, invert[i].shift
		f = (f&m)<<s | (f>>s)&m
		p = (p&m)<<s | (p>>s)&m
		q = (q&m)<<s | (q>>s)&m

		f1 := f
		for _, j := range permuteBit {
			k, m, s := swap[j].keep, swap[j].mask, swap[j].shift
			f = f&k | (f&m)<<s | (f>>s)&m
			p = p&k | (p&m)<<s | (p>>s)&m
			q = q&k | (q&m)<<s | (q>>s)&m

			mask := Func(int32(f<<(32-NumInput))>>31) >> (32 - NumInput)
			fc := f ^ mask

			if fc == inf.F {
				// record p and q now - they match f.
				// as we cycle back to the original f
				// we'll keep them in sync, so that at the
				// end we'll have the right ones for the
				// original.
				p = inf.P ^ mask
				q = inf.Q ^ mask
			}
		}

		if f != f1 {
			panic("findTree permute did not cycle")
		}
	}

	if f != f0 {
		panic("findTree min did not cycle")
	}

	switch {
	case f == p:
		return &Tree{"Lit", f, nil, nil}
	case f == p|q:
		op = "|"
	case f == p&q:
		op = "&"
	case f == p&(q^(NumFunc-1)):
		op = "&"
		q ^= NumFunc - 1
	case f == (p^(NumFunc-1))&q:
		op = "&"
		p ^= NumFunc - 1
	case f == p^q:
		op = "^"
	case f == p^q^(NumFunc-1):
		op = "^"
		q ^= NumFunc - 1
	default:
		panic("cannot determine op")
	}

	if op == "^" && (p>>NumInput)&1 == 0 {
		p ^= NumFunc - 1
		q ^= NumFunc - 1
	}

	return &Tree{op, f, findTree(p, info), findTree(q, info)}
}
