// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"gob"
	"fmt"
	"os"
)

// Change to compute different answer.
// Warning: if you set this to 5 the binary needs over 2G of memory to run.
const NumVar = 4

// Derived constants.
const (
	NumInput = 1 << NumVar
	NumFunc  = 1 << NumInput
)

// A Func represents a single boolean function.
// It specifies only the outputs for each input,
// not a way to compute it.
type Func uint32

func (f Func) String() string {
	return fmt.Sprintf("%#0*x", NumInput/4, uint32(f))
}

// literal returns the function whose value is always
// equal to the literal input #i.
func literal(i int) Func {
	f := Func(0)
	for k := 0; k < NumInput; k++ {
		// k is a bit mask specifying an input.
		// k & (1<<i) is set if i is true in the input.
		// f & (1<<k) should be set if f(k) is true.
		// (How often do you see the same number
		// used as a value and a shift count in the
		// same expression?)
		f |= Func((k>>uint(i))&1) << uint(k)
	}
	return f
}

// A Record records the Boolean function and its parents
// for computing it.
type Record struct {
	F, P, Q Func
}

// maxFunc[n] is the number of NPN-equivalence classes of
// Boolean functions of n or fewer variables.
// It gives the maximum size of the various queues.
// http://oeis.org/A000370
var maxFunc = []int{1, 2, 4, 14, 222, 616126}

// grayBit[i] is the bit to flip to turn the i'th gray code into
// the i+1'th gray code.
// http://oeis.org/A007814
// See also Knuth 7.2.1.1.
var grayBit = [32]uint8{
	0, 1, 0, 2, 0, 1, 0, 3,
	0, 1, 0, 2, 0, 1, 0, 4,
	0, 1, 0, 2, 0, 1, 0, 3,
	0, 1, 0, 2, 0, 1, 0, 5,
}

// Tweak the final entry in the gray bit list
// to cause a cycle, as a sanity check for our
// conversions.
func init() {
	grayBit[NumInput-1]--
}

// To calculate the Func obtained by inverting the i'th input bit of a Func f,
// swap the bits selected by invert[mask with the bits selected by the shifted mask.
// That is, use
//	m, s := invert[i].mask, invert[i].shift
//	f1 := (f&m)<<s | (f>>s)&m
var invert = []struct {
	mask  Func
	shift uint
}{
	{0x55555555, 1},
	{0x33333333, 2},
	{0x0f0f0f0f, 4},
	{0x00ff00ff, 8},
	{0x0000ffff, 16},
}

// permuteBit gives a sequence of adjacent swaps (swap x with x+1)
// that will cycle through all permutations of a given set.
// See also Knuth 7.2.1.2.
var permuteBit = computePermuteBit(NumVar)

// To calculate the Func obtained by swapping i'th and i+1'th input bits of a Func f,
// keep the bits selected by mask keep and then swap the bits selected by mask
// with the bits selected by the shifted mask.
// That is, use:
//	k, m, s := swap[i].keep, swap[i].mask, swap[i].shift
//	f1 := f&k | (f&m)<<s | (f>>s)&m
var swap = []struct {
	keep  Func
	mask  Func
	shift uint
}{
	{0x99999999, 0x22222222, 1},
	{0xc3c3c3c3, 0x0c0c0c0c, 2},
	{0xf00ff00f, 0x00f000f0, 4},
	{0xff0000ff, 0x0000ff00, 8},
}

// Generate permuteBit sequence for n.
// Algorithm is from Knuth 7.2.1.2 Algorithm P (Plain changes).
// 17th century bell ringing algorithm.
func computePermuteBit(n int) []int {
	var out []int

	c := make([]int, n)
	o := make([]int, n)
	for i := range o {
		o[i] = 1
	}
P2:
	j := n
	s := 0
P4:
	q := c[j-1] + o[j-1]
	if q < 0 {
		goto P7
	}
	if q == j {
		goto P6
	}
	x, y := j-c[j-1]+s, j-q+s
	if x < y {
		out = append(out, x-1)
	} else {
		out = append(out, y-1)
	}
	c[j-1] = q
	goto P2
P6:
	if j == 1 {
		// Final swap to return to normal.
		out = append(out, 0)
		if len(out) != fact(n) {
			panic("computePermuteBits: wrong length")
		}
		return out
	}
	s++
P7:
	o[j-1] = -o[j-1]
	j--
	goto P4
}

func fact(i int) int {
	m := 1
	for i > 1 {
		m *= i
		i--
	}
	return m
}

func findMin(f Func) Func {
	minf := f
	f0 := f
	for _, i := range grayBit[0:NumInput] {
		m, s := invert[i].mask, invert[i].shift
		f = (f&m)<<s | (f>>s)&m
		mask := Func(int32(f<<(32-NumInput))>>31) >> (32 - NumInput)
		fc := f ^ mask

		if fc < minf {
			minf = fc
		}

		f1 := f
		for _, j := range permuteBit {
			k, m, s := swap[j].keep, swap[j].mask, swap[j].shift
			f = f&k | (f&m)<<s | (f>>s)&m
			mask := Func(int32(f<<(32-NumInput))>>31) >> (32 - NumInput)
			fc := f ^ mask

			if fc < minf {
				minf = fc
			}

		}

		if f != f1 {
			panic("min permute did not cycle")
		}
	}

	if f != f0 {
		panic("min did not cycle")
	}

	return minf
}

// A Savepoint records information for starting again later.
type Savepoint struct {
	Howto  []Record
	BySize [][]Func
}

func gobMarshal(name string, data interface{}) os.Error {
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	err = gob.NewEncoder(f).Encode(data)
	if err != nil {
		panic(err)
	}
	return nil
}

func gobUnmarshal(name string, data interface{}) os.Error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	err = gob.NewDecoder(f).Decode(data)
	if err != nil {
		panic(err)
	}
	return nil
}
