
//line eqn.y:5

package web

import (
	"fmt"
	"os"
	"strconv"
)


//line eqn.y:16
type	yySymType	struct
{
	yys	int;
	f func([]int) int
	v func([]int) []int
}
const	LNUM	= 57346
const	LVAR	= 57347
const	LIN	= 57348
const	EOF	= 57349
const	LOROR	= 57350
const	LANDAND	= 57351
const	LNE	= 57352
const	LLE	= 57353
const	LGE	= 57354
const	LLSH	= 57355
const	LRSH	= 57356
const	LANDNOT	= 57357
var	yyToknames	 =[]string {
	"LNUM",
	"LVAR",
	"LIN",
	"EOF",
	"LOROR",
	"LANDAND",
	" =",
	"LNE",
	"LLE",
	"LGE",
	" <",
	" >",
	" +",
	" -",
	" |",
	" ^",
	" *",
	" /",
	" %",
	" &",
	"LLSH",
	"LRSH",
	"LANDNOT",
}
var	yyStatenames	 =[]string {
}
																																																											const	yyEofCode	= 1
const	yyErrCode	= 2
const	yyMaxDepth	= 200

//line eqn.y:119


type ExprLex string

func (sp *ExprLex) Lex(yylval *yySymType) int {
	s := *sp
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	*sp = s
	if len(s) == 0 {
		return EOF
	}
	switch s[0] {
	case '+', '-', '*', '/', '%', '^', '(', ')', ',':
	Single:
		*sp = s[1:]
		return int(s[0])
	case '!':
		if len(s) > 1 && s[1] == '=' {
			*sp = s[2:]
			return LNE
		}
		goto Single
	case '>':
		if len(s) > 1 && s[1] == '=' {
			*sp = s[2:]
			return LGE
		}
		if len(s) > 1 && s[1] == '>' {
			*sp = s[2:]
			return LRSH
		}
		goto Single
	case '<':
		if len(s) > 1 && s[1] == '=' {
			*sp = s[2:]
			return LLE
		}
		if len(s) > 1 && s[1] == '<' {
			*sp = s[2:]
			return LLSH
		}
		goto Single
	case '&':
		if len(s) > 1 && s[1] == '&' {
			*sp = s[2:]
			return LANDAND
		}
		if len(s) > 1 && s[1] == '^' {
			*sp = s[2:]
			return LANDNOT
		}
		goto Single
	case '|':
		if len(s) > 1 && s[1] == '|' {
			*sp = s[2:]
			return LOROR
		}
		goto Single
	case '=':
		if len(s) > 1 && s[1] == '=' {
			*sp = s[2:]
			return '='
		}
		goto Single
	case 'v', 'V', 'w', 'W', 'x', 'X', 'y', 'Y', 'z', 'Z':
		c := int(s[0])
		if 'a' <= c && c <= 'z' {
			c += 'A' - 'a'
		}
		yylval.f = func(val []int) int { return val[c-'V'] }
		*sp = s[1:]
		return LVAR
	case 'i':
		if len(s) >= 2 && s[1] == 'n' {
			*sp = s[2:]
			return LIN
		}
		goto Default
	default:
	Default:
		i := 0
		for i < len(s) && '0' <= s[i] && s[i] <= '9' {
			i++
		}
		if i > 0 {
			n, err := strconv.Atoi(string(s[:i]))
			if err != nil {
				sp.Error(err.String())
			}
			yylval.f = func(val []int) int { return n }
			*sp = s[i:]
			return LNUM
		}
		sp.Error("unexpected character")
	}
	panic("not reached")
}

func (sp *ExprLex) Error(s string) {
	panic(s + " near " + strconv.Quote(string(*sp)))
}

func b(v bool) int {
	if v {
		return 1
	}
	return 0
}

func parse(s string) (f func([]int)int, err os.Error) {
	defer func() {
		switch v := recover().(type) {
		case func([]int)int:
			f, err = v, nil
		case string:
			f, err = nil, os.ErrorString(v)
		case os.Error:
			f, err = nil, v
		default:
			panic(v)
		}
	}()
	
	yyParse((*ExprLex)(&s))
	panic("internal error")
}

//line yacctab:1
var	yyExca = []int {
-1, 1,
	1, -1,
	-2, 0,
}
const	yyNprod	= 39
const	yyPrivate	= 57344
var	yyTokenNames []string
var	yyStates []string
const	yyLast	= 118
var	yyAct	= []int {

   4,  19,  20,  22,  23,  21,  24,  25,  26,  27,
  28,  29,  30,  31,  32,  34,  35,  33,  16,  17,
  46,  47,  48,  49,  50,  51,  52,  53,  54,  55,
  56,  57,  58,  59,  60,  61,  62,  17,  64,  45,
   1,  63,  44,  18,   3,   6,  65,  19,  20,  22,
  23,  21,  24,  25,  26,  27,  28,  29,  30,  31,
  32,  34,  35,  33,   5,  67,  12,  13,   0,   2,
  12,  13,  36,  37,  38,  39,  40,   0,   7,   8,
   0,  10,   7,   8,  41,  10,  42,  43,   0,  66,
   9,  11,  14,   0,   9,  11,  14,  25,  26,  27,
  28,  29,  30,  31,  32,  34,  35,  33,  29,  30,
  31,  32,  34,  35,  33,  15,  16,  17,
};
var	yyPact	= []int {

  66,-1000, 108,-1000,  37,-1000,-1000,  66,  66,  66,
  66,  66,-1000,-1000,  66,-1000,  66,  66,-1000,  66,
  66,  66,  66,  66,  66,  66,  66,  66,  66,  66,
  66,  66,  66,  66,  66,  66,-1000,-1000,-1000,-1000,
-1000,  10,  28,-1000,  11,  62,  81,  81,  81,  81,
  81,  81,  88,  88,  88,  88,-1000,-1000,-1000,-1000,
-1000,-1000,-1000,-1000,  66,  -9,-1000,  -9,
};
var	yyPgo	= []int {

   0,   0,  64,  45,  69,  44,  42,  40,  39,
};
var	yyR1	= []int {

   0,   7,   4,   4,   4,   5,   5,   1,   1,   1,
   1,   1,   1,   1,   1,   1,   1,   1,   1,   1,
   1,   1,   1,   1,   1,   6,   6,   6,   8,   8,
   2,   2,   2,   2,   2,   2,   3,   3,   3,
};
var	yyR2	= []int {

   0,   2,   1,   3,   3,   1,   3,   1,   3,   3,
   3,   3,   3,   3,   3,   3,   3,   3,   3,   3,
   3,   3,   3,   3,   3,   2,   2,   3,   0,   2,
   1,   2,   2,   2,   2,   2,   1,   1,   3,
};
var	yyChk	= []int {

-1000,  -7,  -4,  -5,  -1,  -2,  -3,  16,  17,  28,
  19,  29,   4,   5,  30,   7,   8,   9,   6,  10,
  11,  14,  12,  13,  15,  16,  17,  18,  19,  20,
  21,  22,  23,  26,  24,  25,  -2,  -2,  -2,  -2,
  -2,  -4,  -4,  -4,  -6,  -8,  -1,  -1,  -1,  -1,
  -1,  -1,  -1,  -1,  -1,  -1,  -1,  -1,  -1,  -1,
  -1,  -1,  -1,  31,  27,  -1,  27,  -1,
};
var	yyDef	= []int {

   0,  -2,   0,   2,   5,   7,  30,   0,   0,   0,
   0,   0,  36,  37,   0,   1,   0,   0,  28,   0,
   0,   0,   0,   0,   0,   0,   0,   0,   0,   0,
   0,   0,   0,   0,   0,   0,  31,  32,  33,  34,
  35,   0,   3,   4,   6,   0,   8,   9,  10,  11,
  12,  13,  14,  15,  16,  17,  18,  19,  20,  21,
  22,  23,  24,  38,  26,  25,  29,  27,
};
var	yyTok1	= []int {

   1,   3,   3,   3,   3,   3,   3,   3,   3,   3,
   3,   3,   3,   3,   3,   3,   3,   3,   3,   3,
   3,   3,   3,   3,   3,   3,   3,   3,   3,   3,
   3,   3,   3,  28,   3,   3,   3,  22,  23,   3,
  30,  31,  20,  16,  27,  17,   3,  21,   3,   3,
   3,   3,   3,   3,   3,   3,   3,   3,   3,   3,
  14,  10,  15,   3,   3,   3,   3,   3,   3,   3,
   3,   3,   3,   3,   3,   3,   3,   3,   3,   3,
   3,   3,   3,   3,   3,   3,   3,   3,   3,   3,
   3,   3,   3,   3,  19,   3,   3,   3,   3,   3,
   3,   3,   3,   3,   3,   3,   3,   3,   3,   3,
   3,   3,   3,   3,   3,   3,   3,   3,   3,   3,
   3,   3,   3,   3,  18,   3,  29,
};
var	yyTok2	= []int {

   2,   3,   4,   5,   6,   7,   8,   9,  11,  12,
  13,  24,  25,  26,
};
var	yyTok3	= []int {
   0,
 };

//line yaccpar:1

/*	parser for yacc output	*/

var yyDebug = 0

type yyLexer interface {
	Lex(lval *yySymType) int
	Error(s string)
}

const yyFlag = -1000

func yyTokname(c int) string {
	if c > 0 && c <= len(yyToknames) {
		if yyToknames[c-1] != "" {
			return yyToknames[c-1]
		}
	}
	return fmt.Sprintf("tok-%v", c)
}

func yyStatname(s int) string {
	if s >= 0 && s < len(yyStatenames) {
		if yyStatenames[s] != "" {
			return yyStatenames[s]
		}
	}
	return fmt.Sprintf("state-%v", s)
}

func yylex1(lex yyLexer, lval *yySymType) int {
	c := 0
	char := lex.Lex(lval)
	if char <= 0 {
		c = yyTok1[0]
		goto out
	}
	if char < len(yyTok1) {
		c = yyTok1[char]
		goto out
	}
	if char >= yyPrivate {
		if char < yyPrivate+len(yyTok2) {
			c = yyTok2[char-yyPrivate]
			goto out
		}
	}
	for i := 0; i < len(yyTok3); i += 2 {
		c = yyTok3[i+0]
		if c == char {
			c = yyTok3[i+1]
			goto out
		}
	}

out:
	if c == 0 {
		c = yyTok2[1] /* unknown char */
	}
	if yyDebug >= 3 {
		fmt.Printf("lex %U %s\n", uint(char), yyTokname(c))
	}
	return c
}

func yyParse(yylex yyLexer) int {
	var yyn int
	var yylval yySymType
	var yyVAL yySymType
	yyS := make([]yySymType, yyMaxDepth)

	Nerrs := 0   /* number of errors */
	Errflag := 0 /* error recovery flag */
	yystate := 0
	yychar := -1
	yyp := -1
	goto yystack

ret0:
	return 0

ret1:
	return 1

yystack:
	/* put a state and value onto the stack */
	if yyDebug >= 4 {
		fmt.Printf("char %v in %v\n", yyTokname(yychar), yyStatname(yystate))
	}

	yyp++
	if yyp >= len(yyS) {
		nyys := make([]yySymType, len(yyS)*2)
		copy(nyys, yyS)
		yyS = nyys
	}
	yyS[yyp] = yyVAL
	yyS[yyp].yys = yystate

yynewstate:
	yyn = yyPact[yystate]
	if yyn <= yyFlag {
		goto yydefault /* simple state */
	}
	if yychar < 0 {
		yychar = yylex1(yylex, &yylval)
	}
	yyn += yychar
	if yyn < 0 || yyn >= yyLast {
		goto yydefault
	}
	yyn = yyAct[yyn]
	if yyChk[yyn] == yychar { /* valid shift */
		yychar = -1
		yyVAL = yylval
		yystate = yyn
		if Errflag > 0 {
			Errflag--
		}
		goto yystack
	}

yydefault:
	/* default state action */
	yyn = yyDef[yystate]
	if yyn == -2 {
		if yychar < 0 {
			yychar = yylex1(yylex, &yylval)
		}

		/* look through exception table */
		xi := 0
		for {
			if yyExca[xi+0] == -1 && yyExca[xi+1] == yystate {
				break
			}
			xi += 2
		}
		for xi += 2; ; xi += 2 {
			yyn = yyExca[xi+0]
			if yyn < 0 || yyn == yychar {
				break
			}
		}
		yyn = yyExca[xi+1]
		if yyn < 0 {
			goto ret0
		}
	}
	if yyn == 0 {
		/* error ... attempt to resume parsing */
		switch Errflag {
		case 0: /* brand new error */
			yylex.Error("syntax error")
			Nerrs++
			if yyDebug >= 1 {
				fmt.Printf("%s", yyStatname(yystate))
				fmt.Printf("saw %s\n", yyTokname(yychar))
			}
			fallthrough

		case 1, 2: /* incompletely recovered error ... try again */
			Errflag = 3

			/* find a state where "error" is a legal shift action */
			for yyp >= 0 {
				yyn = yyPact[yyS[yyp].yys] + yyErrCode
				if yyn >= 0 && yyn < yyLast {
					yystate = yyAct[yyn] /* simulate a shift of "error" */
					if yyChk[yystate] == yyErrCode {
						goto yystack
					}
				}

				/* the current p has no shift onn "error", pop stack */
				if yyDebug >= 2 {
					fmt.Printf("error recovery pops state %d, uncovers %d\n",
						yyS[yyp].yys, yyS[yyp-1].yys)
				}
				yyp--
			}
			/* there is no state on the stack with an error shift ... abort */
			goto ret1

		case 3: /* no shift yet; clobber input char */
			if yyDebug >= 2 {
				fmt.Printf("error recovery discards %s\n", yyTokname(yychar))
			}
			if yychar == yyEofCode {
				goto ret1
			}
			yychar = -1
			goto yynewstate /* try again in the same state */
		}
	}

	/* reduction by production yyn */
	if yyDebug >= 2 {
		fmt.Printf("reduce %v in:\n\t%v\n", yyn, yyStatname(yystate))
	}

	yynt := yyn
	yypt := yyp
	_ = yypt		// guard against "declared and not used"

	yyp -= yyR2[yyn]
	yyVAL = yyS[yyp+1]

	/* consult goto table to find next state */
	yyn = yyR1[yyn]
	yyg := yyPgo[yyn]
	yyj := yyg + yyS[yyp].yys + 1

	if yyj >= yyLast {
		yystate = yyAct[yyg]
	} else {
		yystate = yyAct[yyj]
		if yyChk[yystate] != -yyn {
			yystate = yyAct[yyg]
		}
	}
	// dummy call; replaced with literal code
	switch yynt {

case 1:
//line eqn.y:38
{ panic(yyS[yypt-1].f) }
case 2:
	yyVAL.f = yyS[yypt-0].f;
case 3:
//line eqn.y:43
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return b(f(val) != 0 || g(val) != 0) } }
case 4:
//line eqn.y:45
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return b(f(val) != 0 && g(val) != 0) } }
case 5:
	yyVAL.f = yyS[yypt-0].f;
case 6:
//line eqn.y:50
{ f, l := yyS[yypt-2].f, yyS[yypt-0].v; yyVAL.f = func(val []int) int { x := f(val); for _, y := range l(val) { if x == y { return 1 } }; return 0 } }
case 7:
	yyVAL.f = yyS[yypt-0].f;
case 8:
//line eqn.y:55
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return b(f(val) ==  g(val)) } }
case 9:
//line eqn.y:57
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return b(f(val) != g(val)) } }
case 10:
//line eqn.y:59
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return b(f(val) < g(val)) } }
case 11:
//line eqn.y:61
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return b(f(val) <= g(val)) } }
case 12:
//line eqn.y:63
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return b(f(val) >= g(val)) } }
case 13:
//line eqn.y:65
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return b(f(val) > g(val)) } }
case 14:
//line eqn.y:67
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return f(val) + g(val) } }
case 15:
//line eqn.y:69
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return f(val) - g(val) } }
case 16:
//line eqn.y:71
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return f(val) | g(val) } }
case 17:
//line eqn.y:73
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return f(val) ^ g(val) } }
case 18:
//line eqn.y:75
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return f(val) * g(val) } }
case 19:
//line eqn.y:77
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return f(val) / g(val) } }
case 20:
//line eqn.y:79
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int {return f(val) % g(val) } }
case 21:
//line eqn.y:81
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return f(val) & g(val) } }
case 22:
//line eqn.y:83
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return f(val) &^ g(val) } }
case 23:
//line eqn.y:85
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return f(val) << uint(g(val)) } }
case 24:
//line eqn.y:87
{ f, g := yyS[yypt-2].f, yyS[yypt-0].f; yyVAL.f = func(val []int) int { return f(val) >> uint(g(val)) } }
case 25:
//line eqn.y:91
{ f := yyS[yypt-0].f; yyVAL.v = func(val []int) []int { return []int{f(val)} } }
case 26:
//line eqn.y:93
{ yyVAL.v = yyS[yypt-1].v }
case 27:
//line eqn.y:95
{ l, f := yyS[yypt-2].v, yyS[yypt-0].f; yyVAL.v = func(val []int) []int { return append(l(val), f(val)) } }
case 30:
	yyVAL.f = yyS[yypt-0].f;
case 31:
//line eqn.y:103
{ f := yyS[yypt-0].f; yyVAL.f = f }
case 32:
//line eqn.y:105
{ f := yyS[yypt-0].f; yyVAL.f = func(val []int) int { return -f(val) } }
case 33:
//line eqn.y:107
{ f := yyS[yypt-0].f; yyVAL.f = func(val []int) int { return b(f(val) == 0) } }
case 34:
//line eqn.y:109
{ f := yyS[yypt-0].f; yyVAL.f = func(val []int) int { return ^f(val) } }
case 35:
//line eqn.y:111
{ f := yyS[yypt-0].f; yyVAL.f = func(val []int) int { return ^f(val) } }
case 36:
	yyVAL.f = yyS[yypt-0].f;
case 37:
	yyVAL.f = yyS[yypt-0].f;
case 38:
//line eqn.y:117
{ yyVAL.f = yyS[yypt-1].f }
	}
	goto yystack /* stack new state and value */
}
