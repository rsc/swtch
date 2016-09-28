// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rscio

import (
	"net/http"

	"rsc.io/go-import-redirector/godoc"
)

func init() {
	http.Handle("/", godoc.Redirect("git", "rsc.io/*", "https://github.com/rsc/*"))
}
