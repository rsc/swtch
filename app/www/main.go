// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package www

import (
	"net/http"

	"rsc.io/swtch/servegcs"
)

func init() {
	http.Handle("/", servegcs.Handler("swtch.com", "swtch/www"))
	http.HandleFunc("/plan9port/", servegcs.RedirectHost("9fans.github.io"))
	http.HandleFunc("www.swtch.com/", servegcs.RedirectHost("swtch.com"))
}
