// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

%{

package web

import (
	"fmt"
	"os"
	"strconv"
)

%}

%union
{
	f func([]int) int
	v func([]int) []int
}

%type	<f>	expr uexpr pexpr condexpr inexpr
%token	<f>	LNUM LVAR LIN
%type	<v>	exprlist
%token	EOF

%left	LOROR
%left	LANDAND
%left LIN
%left	'=' LNE LLE LGE '<' '>'
%left	'+' '-' '|' '^'
%left	'*' '/' '%' '&' LLSH LRSH LANDNOT

%%

prog:
	condexpr EOF
	{ panic($1) }

condexpr:
	inexpr
|	condexpr LOROR condexpr
	{ f, g := $1, $3; $$ = func(val []int) int { return b(f(val) != 0 || g(val) != 0) } }
|	condexpr LANDAND condexpr
	{ f, g := $1, $3; $$ = func(val []int) int { return b(f(val) != 0 && g(val) != 0) } }

inexpr:
	expr
|	expr LIN exprlist
	{ f, l := $1, $3; $$ = func(val []int) int { x := f(val); for _, y := range l(val) { if x == y { return 1 } }; return 0 } }

expr:
	uexpr
|	expr '=' expr
	{ f, g := $1, $3; $$ = func(val []int) int { return b(f(val) ==  g(val)) } }
|	expr LNE expr
	{ f, g := $1, $3; $$ = func(val []int) int { return b(f(val) != g(val)) } }
|	expr '<' expr
	{ f, g := $1, $3; $$ = func(val []int) int { return b(f(val) < g(val)) } }
|	expr LLE expr
	{ f, g := $1, $3; $$ = func(val []int) int { return b(f(val) <= g(val)) } }
|	expr LGE expr
	{ f, g := $1, $3; $$ = func(val []int) int { return b(f(val) >= g(val)) } }
|	expr '>' expr
	{ f, g := $1, $3; $$ = func(val []int) int { return b(f(val) > g(val)) } }
|	expr '+' expr
	{ f, g := $1, $3; $$ = func(val []int) int { return f(val) + g(val) } }
|	expr '-' expr
	{ f, g := $1, $3; $$ = func(val []int) int { return f(val) - g(val) } }
|	expr '|' expr
	{ f, g := $1, $3; $$ = func(val []int) int { return f(val) | g(val) } }
|	expr '^' expr
	{ f, g := $1, $3; $$ = func(val []int) int { return f(val) ^ g(val) } }
|	expr '*' expr
	{ f, g := $1, $3; $$ = func(val []int) int { return f(val) * g(val) } }
|	expr '/' expr
	{ f, g := $1, $3; $$ = func(val []int) int { return f(val) / g(val) } }
|	expr '%' expr
	{ f, g := $1, $3; $$ = func(val []int) int {return f(val) % g(val) } }
|	expr '&' expr
	{ f, g := $1, $3; $$ = func(val []int) int { return f(val) & g(val) } }
|	expr LANDNOT expr
	{ f, g := $1, $3; $$ = func(val []int) int { return f(val) &^ g(val) } }
|	expr LLSH expr
	{ f, g := $1, $3; $$ = func(val []int) int { return f(val) << uint(g(val)) } }
|	expr LRSH expr
	{ f, g := $1, $3; $$ = func(val []int) int { return f(val) >> uint(g(val)) } }

exprlist:
	commas expr
	{ f := $2; $$ = func(val []int) []int { return []int{f(val)} } }
|	exprlist ','
	{ $$ = $1 }
|	exprlist ',' expr
	{ l, f := $1, $3; $$ = func(val []int) []int { return append(l(val), f(val)) } }

commas:
|	commas ','

uexpr:
	pexpr
|	'+' uexpr
	{ f := $2; $$ = f }
|	'-' uexpr
	{ f := $2; $$ = func(val []int) int { return -f(val) } }
|	'!' uexpr
	{ f := $2; $$ = func(val []int) int { return b(f(val) == 0) } }
|	'^' uexpr
	{ f := $2; $$ = func(val []int) int { return ^f(val) } }
|	'~' uexpr
	{ f := $2; $$ = func(val []int) int { return ^f(val) } }

pexpr:
	LNUM
|	LVAR
|	'(' condexpr ')'
	{ $$ = $2 }

%%

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
