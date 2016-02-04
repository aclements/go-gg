// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package table

import (
	"reflect"
	"testing"
)

var xgid = RootGroupID.Extend("xgid")

func TestEmptyTable(t *testing.T) {
	var tab Table
	tab.Add("x", []int{})
	tab.Add("x", []int{1, 2, 3})
	if v := tab.Len(); v != 0 {
		t.Fatalf("Table{}.Len() should be 0; got %v", v)
	}
	if v := tab.Columns(); v != nil {
		t.Fatalf("Table{}.Columns() should be nil; got %v", v)
	}
	if v := tab.Column("x"); v != nil {
		t.Fatalf("Table{}.Column(\"x\") should be nil; got %v", v)
	}
	if v, w := tab.Groups(), []GroupID{}; !reflect.DeepEqual(v, w) {
		t.Fatalf("Table{}.Groups should be %v; got %v", w, v)
	}
	if v := tab.Table(RootGroupID); v != nil {
		t.Fatalf("Table{}.Table(RootGroupID) should be nil; got %v", v)
	}
	if v := tab.Table(xgid); v != nil {
		t.Fatalf("Table{}.Table(xgid) should be nil; got %v", v)
	}
	// TODO: AddTable.
}
