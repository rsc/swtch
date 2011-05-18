// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"time"
)

var redo = flag.Int("redo", -1, "level to redo")
var xor = flag.Bool("xor", false, "allow xor")
var cutoff = flag.Int("cutoff", 30, "last level for explore algorithm")

// A Queue is a queue of Boolean functions that we found.
type Queue []Func

// take takes the current entries from the queue.
func (q *Queue) take() []Func {
	ret := *q
	*q = (*q)[len(*q):]
	return ret
}

var howto []Record
var xorprefix = ""

func main() {
	flag.Parse()

	if *xor {
		xorprefix = "xor."
	}

	// Queue of all functions to consider.
	q := make(Queue, 0, maxFunc[NumVar])
	howto = make([]Record, 0, maxFunc[NumVar])
	
	// Functions in queue indexed by size.
	bySize := make([][]Func, 100)

	// Initialize size bits to all 1s to mean unknown.
	for i := range size {
		size[i] = ^uint64(0)
	}

	// Try to pick up where we left off.
	var targ int
	for targ = 30; targ > 0; targ-- {
		var state Savepoint
		name := fmt.Sprintf("/tmp/%sa056287.%d.%d.gob", xorprefix, NumVar, targ)
		if err := gobUnmarshal(name, &state); err != nil {
			continue
		}
		h := state.Howto
		nh := 0
		for i, b := range state.BySize {
			if i == *redo {
				targ = *redo-1
				break
			}
			for _, h := range h[nh:nh+len(b)] {
				q.visit(h.F, h.P, h.Q, i)
			}
			nh += len(b)
			bySize[i] = q.take()
			log.Println(i, len(bySize[i]), nvisited, cap(bySize[0])-cap(q), len(howto))
		}
		break
	}
	
	if targ == 0 {
		// Build and queue functions of complexity 0.
		// There's only one (all the others are equivalent).
		q.visit(literal(0)^(NumFunc-1), literal(0)^(NumFunc-1), 0, 0)
		bySize[0] = q.take()
		if nvisited != 2*NumVar {
			panic("wrong visit count after literal")
		}
		log.Println(0, len(bySize[0]), nvisited, cap(bySize[0])-cap(q), len(howto))
	}

	for targ++; nvisited < NumFunc; targ++ {
		runtime.GC()
		var t0, t1 int64
		if targ <= *cutoff {
			// Build functions of higher complexity from lower ones.
			resetDid()
			t0 = time.Nanoseconds()
			for i := 0; i+i+1 < targ; i++ {
				fs := bySize[i]
				gs := bySize[targ-1-i]
				for _, f := range fs {
					q.explore(f, gs, targ)
				}
			}
			if (targ-1)%2 == 0 {
				i := targ/2
				gs := bySize[i]
				for j, f := range gs {
					q.explore(f, gs[:j+1], targ)
				}
			}
			t1 = time.Nanoseconds()
		} else {
			// Near end.  Search for ways to create missing functions.
			missing := NumFunc - nvisited
			t0 = time.Nanoseconds()
			total0, total1, total2 := q.searchRange(0, NumFunc/2, bySize, targ)
			t1 = time.Nanoseconds()
			log.Println("search", targ, missing, NumFunc-nvisited, total0, total1, total2)
		}
		bySize[targ] = q.take()
		log.Println(targ, len(bySize[targ]), nvisited, cap(bySize[0])-cap(q), len(howto), float64(t1-t0)/1e9)

		var state Savepoint
		state.Howto = howto
		state.BySize = bySize[0:targ+1]
		name := fmt.Sprintf("/tmp/%sa056287.%d.%d.gob", xorprefix, NumVar, targ)
		err := gobMarshal(name, &state)
		if err != nil {
			log.Printf("gob marshal: %s", err)
		}
	}
}

// visited holds the bitmap of which functions we've visited.
// We can arrange to clear the top bit of any func without
// loss of generality, hence the extra factor of two.
var visited [(NumFunc/2+64-1)/64]uint64
var nvisited uint64  // number of bits set

// size gives the number of variables needed to compute
// the function f.  It packs the 5-bit sizes of 12 functions into
// a single 64-bit word.  (The last 4 bits in each word are wasted.)
var size [(NumFunc/2+11)/12]uint64

// did is a probabilistic data structure for tracking which
// functions f have already been explored.  The exploration
// of f begins by setting did[f%len(f)] = f, so if did[f%len(f)]==f,
// then f has been explored.  Because multiple f hash to the 
// same array index, did[f%len(f)] != f does not guarantee that
// f is unexplored, but the possibility is rare and the duplicated
// effort harmless if inefficient.
//
// The probabilistic check gives O(1) lookup and O(1) insertion times,
// in contrast to the larger times for a precise sorted or unsorted list,
// and it is utterly trivial to implement.
//
var did [100003]Func

func resetDid() {
	for i := range did {
		did[i] = Func(i+1)  // impossible value for slot i
	}
}

// explore tries all the inversions and permutations of f
// and applies them to each of the functions in gs.
func (q *Queue) explore(f Func, gs []Func, size int) {
	f0 := f
Gray:
	for _, i := range grayBit[0:NumInput] {
		m, s := invert[i].mask, invert[i].shift
		f = (f&m)<<s | (f>>s)&m
		mask := Func(int32(f<<(32-NumInput))>>31)>>(32-NumInput)
		fc := f ^ mask
		if did[fc%Func(len(did))] == fc {
			continue Gray
		}

		f1 := f

		// Try all possible permutations, swapping according to
		// permutation sequence.
	Perm:
		for _, j := range permuteBit {
			k, m, s := swap[j].keep, swap[j].mask, swap[j].shift
			f = f&k | (f&m)<<s | (f>>s)&m
			mask := Func(int32(f<<(32-NumInput))>>31)>>(32-NumInput)
			fc := f ^ mask
			off := fc%Func(len(did))
			if did[off] == fc {
				continue Perm
			}
			did[off] = fc
			
			q.explorePair(fc, gs, size)
		}
		
		if f != f1 {
			panic("explore permute did not cycle")
		}
	}

	if f != f0 {
		panic("explore did not cycle")
	}
}

// explorePair tries the four possible combinations of f and g
// for each g in gs, calling visit for each new function created.
func (q *Queue) explorePair(f Func, gs []Func, size int) {
	// Try combination with all g's.
	// Can skip half because they're the negations of the
	// other half.  The ones chosen below are the ones that
	// preserve top-bit-clear.
	for _, g := range gs {
		fg := f & g
		if fg != f && fg != g && visited[fg>>6]&(1<<(fg&63)) == 0 {
			q.visit(fg, f, g, size)
		}

		fg = f | g
		if fg != f && fg != g && visited[fg>>6]&(1<<(fg&63)) == 0 {
			q.visit(fg, f, g, size)
		}

		g1 := g^(NumFunc-1)
		fg = f & g1
		if fg != f && visited[fg>>6]&(1<<(fg&63)) == 0 {
			q.visit(fg, f, g1, size)
		}

		f1 := f^(NumFunc-1)
		fg = f1 & g;
		if fg != g && visited[fg>>6]&(1<<(fg&63)) == 0 {
			q.visit(fg, f1, g, size)
		}
		
		if *xor {
			fg = f ^ g
			if fg != g && fg != f && visited[fg>>6]&(1<<(fg&63)) == 0 {
				q.visit(fg, f, g, size)
			}
		}
	}
}

// visit is a no-op if f has already been visited.
// Otherwise, it computes all the binary functions equivalent
// to f, marks them all visited (to make future checks easier),
// and adds f to q.  It also adds p and r as the ``parents'' of f.
func (q *Queue) visit(f, p, r Func, fsize int) {
	f0, p0, r0 := f, p, r

	// Have we visited f before?  If so we're done.
	if visited[f>>6]&(1<<(f&63)) != 0 {
		return
	}
	
	// Otherwise set up for minimum.
	minf := f
	minp := p
	minr := r

	// Otherwise, try all possible permutations of f's input variables
	// and all possible negations of the input variables to find
	// functions equivalent to f, and set the bits for all of them.
	
	// Cycle through inputs, negating according to Gray code
	// (minimum number of negations).
	for _, i := range grayBit[0:NumInput] {
		m, s := invert[i].mask, invert[i].shift
		f = (f&m)<<s | (f>>s)&m
		p = (p&m)<<s | (p>>s)&m
		r = (r&m)<<s | (r>>s)&m

		mask := Func(int32(f<<(32-NumInput))>>31)>>(32-NumInput)
		fc := f ^ mask

		if fc < minf {
			minf = fc
			minp = p ^ mask
			minr = r ^ mask
		}

		// Might have seen this in an earlier round.
		if visited[fc>>6]&(1<<(fc&63)) != 0 {
			continue
		}

		f1, p1, r1 := f, p, r

		// Try all possible permutations, swapping according to
		// permutation sequence.
		for _, j := range permuteBit {
			k, m, s := swap[j].keep, swap[j].mask, swap[j].shift
			f = f&k | (f&m)<<s | (f>>s)&m
			p = p&k | (p&m)<<s | (p>>s)&m
			r = r&k | (r&m)<<s | (r>>s)&m
			mask := Func(int32(f<<(32-NumInput))>>31)>>(32-NumInput)
			fc := f ^ mask

			if fc < minf {
				minf = fc
				minp = p ^ mask
				minr = r ^ mask
			}

			index := fc>>6
			bit := uint64(1)<<(fc&63)
			if visited[index]&bit != 0 {
				continue
			}

			visited[index] |= bit
			nvisited += 2
			shift := 5 * (fc%12)
			size[fc/12] &= uint64(fsize)<<shift | ^(0x1F<<shift)
		}
		
		if f != f1 || p != p1 || r != r1 {
			panic("visit permute did not cycle")
		}
	}

	if f != f0 || p != p0 || r != r0 {
		panic("visit did not cycle")
	}

	*q = append(*q, minf)
	howto = append(howto, Record{minf, minp, minr})		
}

// searchRange scans the visited bitmap for functions fg such that lo <= fg < hi
// that have not been visited, and then it looks for all possible ways
// to create fg by combining two existing functions f and g to make
// size targ.  On the face of it this would seem a very inefficient way to
// find fg, because the same pairs f, g get considered once for every 
// missing function.  However, we run the search by considering only
// the g that can possibly lead to fg and then deriving all the possible f.
// This cuts the search pairs quite a bit.
func (q *Queue) searchRange(lo, hi Func, bySize [][]Func, targ int) (n0, n1, n2 int64) {
	for fg := lo &^ 63; fg < hi; {
		m := visited[fg>>6]
		if ^m == 0 {	// all visited
			fg += 64
			continue
		}
		for i := Func(0); i < 64; i++ {
			if m & (1<<i) == 0 && lo <= fg && fg < hi {
				n0++
				nn1, nn2 := q.search(fg, bySize, targ)
				n1 += nn1
				n2 += nn2
			}
			fg++
		}
	}
	return
}

func (q *Queue) search(fg Func, bySize [][]Func, targ int) (n1, n2 int64) {
	for size, bs := range bySize[:targ] {
		for _, g := range bs {
			// Which f would give us fg in the search loop?
			// fg loops over all possible forms,
			// g loops over minimal canonical forms.
			// find looks for any possible f (not just canonical).
			// Need to pick the forms we'll look for.

			g1 := g ^ (NumFunc-1)
			fg1 := fg ^ (NumFunc-1)
			
			// Find f such that fg = f | g.
			// Only possible if fg | g == fg.
			// Must have f[i] == 1 where fg[i] == 1 and g[i] == 0.
			// Can have f[i] == 1 anywhere g[i] == 1.
			if fg | g == fg {
				n1++
				f, ok, nn := find(fg&^g, g, targ-size-1)
				n2 += nn 
				if ok {
					q.visit(fg, f, g, targ)
					return
				}
			}
			
			// Find f such that fg = f | ^g.
			if fg | g1 == fg {
				n1++
				f, ok, nn := find(fg&g, g1, targ-size-1)
				n2 += nn
				if ok {
					q.visit(fg, f, g1, targ)
					return
				}
			}
			
			// Find f such that ^fg = f | g (aka fg = ^f & ^g).
			if fg1 | g == fg1 {
				n1++
				f, ok, nn := find(fg1&^g, g, targ-size-1)
				n2 += nn
				if ok {
					q.visit(fg, f^(NumFunc-1), g1, targ)
					return
				}
			}
			
			// Find f such that ^fg = f | ^g (aka fg = ^f & g)
			if fg1 | g1 == fg1 {
				n1++
				f, ok, nn := find(fg1&g, g1, targ-size-1)
				n2 += nn
				if ok {
					q.visit(fg, f^(NumFunc-1), g, targ)
					return
				}
			}
		}
	}
	return
}

// Find looks for an already computed function f = x | m&canSet
// (for an arbitrary mask) with the given target size.
func find(x, canSet Func, targetSize int) (Func, bool, int64) {
	// Try x.  (Canonicalize to x1.)
	x1 := x ^ Func(int32(x<<(32-NumInput))>>31)>>(32-NumInput)
	n := int64(1)
	if visited[x1>>6]&(1<<(x1&63)) != 0 {
		xsize := int(size[x1/12] >> (5*(x1%12))) & 0x1F
		if xsize == targetSize {
			return x, true, n
		}
	}

	// Kick off recursions trying effect of setting various bits.
	for canSet != 0 {
		bit := canSet & -canSet  // bottom 1 bit in canSet
		canSet ^= bit
		f, ok, nn := find(x | bit, canSet, targetSize)
		n += nn
		if ok {
			return f, ok, n
		}
	}

	return 0, false, n
}
