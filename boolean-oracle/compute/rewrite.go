// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This program rewrites "gob" encoded tables into raw binary tables.
// The latter require less memory to decode on App Engine startup.

package main

import (
	"bufio"
	"encoding/binary"
	"encoding/gob"
	"io"
	"log"
	"os"
	"reflect"
	"runtime"
)

type Func uint32

type Record struct {
	F, P, Q Func
}

type Savepoint struct {
	Howto  []Record
	BySize [][]Func
}

func gobUnmarshal(name string, data interface{}) error {
	var err error
	var f io.Reader
	f, err = os.Open(name)
	if err != nil {
		f1, err := os.Open(name + ".aa")
		if err != nil {
			return err
		}
		f2, err := os.Open(name + ".ab")
		if err != nil {
			return err
		}
		f = io.MultiReader(f1, f2)
	}
	println("decode", name)
	err = gob.NewDecoder(f).Decode(data)
	println("done", len(data.(*Savepoint).Howto))
	runtime.GC()
	if err != nil {
		panic(err)
	}
	return nil
}

type intwriter struct {
	*bufio.Writer
	buf [4]byte
}

func (w *intwriter) Write(x uint32) {
	binary.BigEndian.PutUint32(w.buf[:], x)
	w.Writer.Write(w.buf[:])
}

type intreader struct {
	*bufio.Reader
	buf [4]byte
}

func (r *intreader) Read() uint32 {
	n, err := io.ReadFull(r.Reader, r.buf[:])
	if n != 4 {
		log.Fatal(err)
	}
	return binary.BigEndian.Uint32(r.buf[:])
}

func rawMarshal(name string, sp *Savepoint) {
	f, err := os.Create(name)
	if err != nil {
		log.Fatal(err)
	}
	w := &intwriter{Writer: bufio.NewWriter(f)}
	w.Write(uint32(len(sp.Howto)))
	for _, r := range sp.Howto {
		w.Write(uint32(r.F))
		w.Write(uint32(r.P))
		w.Write(uint32(r.Q))
	}
	w.Write(uint32(len(sp.BySize)))
	for _, ff := range sp.BySize {
		w.Write(uint32(len(ff)))
		for _, f := range ff {
			w.Write(uint32(f))
		}
	}
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
}

func rawUnmarshal(name string, sp *Savepoint) {
	f, err := os.Open(name)
	if err != nil {
		log.Fatal(err)
	}
	r := &intreader{Reader: bufio.NewReader(f)}
	sp.Howto = make([]Record, r.Read())
	for i := range sp.Howto {
		sp.Howto[i] = Record{
			Func(r.Read()),
			Func(r.Read()),
			Func(r.Read()),
		}
	}
	sp.BySize = make([][]Func, r.Read())
	for i := range sp.BySize {
		x := make([]Func, r.Read())
		for j := range x {
			x[j] = Func(r.Read())
		}
		sp.BySize[i] = x
	}
}

func main() {
	var sp Savepoint

	if err := gobUnmarshal(os.Args[1], &sp); err != nil {
		log.Fatal(err)
	}
	rawMarshal(os.Args[2], &sp)
	var sp2 Savepoint
	rawUnmarshal(os.Args[2], &sp2)
	if !reflect.DeepEqual(&sp, &sp2) {
		log.Fatal("not equal")
	}
}
