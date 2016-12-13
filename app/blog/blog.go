// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package blog

import (
	"net/http"

	"rsc.io/swtch/servegcs"
)

func init() {
	http.Handle("/", servegcs.Handler("research.swtch.com", "swtch/www-blog"))
	http.Handle("/feeds/posts/default", http.RedirectHandler("/feed.atom", http.StatusFound))
}
