// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

// A Binding combines a set of data values (such as numeric values or
// factors) with a set of scales for mapping them to visual properties
// (such as position or color). A Binding consists of one or more
// groups, where each group has a sequence of data values and a scale.
// Typically, all groups in a Binding will have the same scale, but
// this isn't necessary.
type Binding map[*GroupID]*BindingGroup

// TODO: Maybe call this VarScalar? BindingGroup could be a group of
// Bindings, which it isn't.

// TODO: This representation doesn't force that all groups in a
// binding have the same Var type. If we had a first-class
// representation of a collection of bindings, each binding could just
// be a single Var with an extra "column" giving the groups. Or we
// could require each group to be at a contiguous range of indexes.

// A BindingGroup represents a single group within a Binding. It is a
// sequence of data values with a consistent scale.
type BindingGroup struct {
	Var    Var
	Scaler Scaler
}

// TODO: Should GroupID be the pointer, since you never want to
// operate on the struct itself?

type GroupID struct {
	Parent *GroupID
	Label  string
}

var RootGroupID = (*GroupID)(nil)

func (g *GroupID) String() string {
	if g == nil {
		return "/"
	}
	buflen := -1
	for p := g; p != nil; p = p.Parent {
		buflen += len(p.Label) + 1
	}
	buf := make([]byte, buflen)
	bufpos := len(buf)
	for p := g; p != nil; p = p.Parent {
		if p != g {
			bufpos--
			buf[bufpos] = '/'
		}
		bufpos -= len(p.Label)
		copy(buf[bufpos:], p.Label)
	}
	return string(buf)
}

func (g *GroupID) Extend(label string) *GroupID {
	return &GroupID{g, label}
}
