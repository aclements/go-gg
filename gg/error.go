// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import "reflect"

// XXX I'm not sure how useful Method is here. See generic.TypeError.

type TypeError struct {
	Method string
	Type   reflect.Type
	Extra  string
}

func (e TypeError) Error() string {
	msg := e.Method + " cannot be used with value of type " + e.Type.String()
	if e.Extra != "" {
		msg += "; " + e.Extra
	}
	return msg
}
