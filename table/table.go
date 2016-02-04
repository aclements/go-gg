// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package table implements ordered, grouped two dimensional relations.
//
// There are two related abstractions: Table and Grouping.
//
// A Table is an ordered relation of rows and columns. Each column is
// a Go slice and hence must be homogeneously typed, but different
// columns may have different types. All columns in a Table have the
// same number of rows.
//
// A Grouping generalizes a Table by grouping the Table's rows into
// zero or more groups. A Table is itself a Grouping with zero or one
// groups. Most operations take a Grouping and operate on each group
// independently, though some operations sub-divide or combine groups.
//
// The structures of both Tables and Groupings are immutable. Adding a
// column to a Table returns a new Table and adding a new Table to a
// Grouping returns a new Grouping.
package table

import (
	"reflect"
	"strconv"

	"github.com/aclements/go-gg/generic"
)

// A Table is an ordered two dimensional relation. It consists of a
// set of named columns, where each column is a sequence of values of
// a consistent type and all columns have the same length.
//
// The zero value of Table is the "empty table": it has no rows and no
// columns. Note that a Table may have one or more columns, but no
// rows; such a Table is *not* considered empty.
//
// A Table is also a trivial Grouping. If a Table is empty, it has no
// groups and hence the zero value of Table is also the "empty group".
// Otherwise, it consists only of the root group, RootGroupID.
//
// A Table's structure is immutable. To construct a Table, start with
// an empty table and add columns to it using Add.
type Table struct {
	cols     map[string]Slice
	colNames []string
	len      int
}

// A Grouping is a set of tables with identical sets of columns, each
// identified by a distinct GroupID.
//
// Visually, a Grouping can be thought of as follows:
//
//	   Col A  Col B  Col C
//	------ group /a ------
//	0   5.4    "x"     90
//	1   -.2    "y"     30
//	------ group /b ------
//	0   9.3    "a"     10
//
// Like a Table, a Grouping's structure is immutable. To construct a
// Grouping, start with a Table (typically the empty Table, which is
// also the empty Grouping) and add tables to it using AddTable.
//
// Despite the fact that GroupIDs form a hierarchy, a Grouping ignores
// this hierarchy and simply operates on a flat map of distinct
// GroupIDs to Tables.
type Grouping interface {
	// Columns returns the names of the columns in this Grouping,
	// or nil if there are no Tables or the group consists solely
	// of empty Tables. All Tables in this Grouping have the same
	// set of columns.
	Columns() []string

	// Groups returns the IDs of the groups in this Grouping.
	Groups() []GroupID

	// Table returns the Table in group gid, or nil if there is no
	// such Table.
	Table(gid GroupID) *Table

	// AddTable reparents Table t into group gid and returns a new
	// Grouping with the addition of Table t bound to group gid.
	// If t is the empty Table, this is a no-op because the empty
	// Table contains no groups. Otherwise, if group gid already
	// exists, it is first removed. Table t must have the same set
	// of columns as any existing Tables in this group or AddTable
	// will panic.
	//
	// TODO The same set or the same sequence of columns? Given
	// that I never use the sequence (except maybe for printing),
	// perhaps it doesn't matter.
	//
	// TODO This doesn't make it easy to combine two Groupings. It
	// could instead take a Grouping and reparent it.
	AddTable(gid GroupID, t *Table) Grouping
}

type groupedTable struct {
	tables   map[GroupID]*Table
	groups   []GroupID
	colNames []string
}

// A Slice is a Go slice value.
//
// This is primarily for documentation. There is no way to statically
// enforce this in Go; however, functions that expect a Slice will
// panic with a *generic.TypeError if passed a non-slice value.
type Slice interface{}

func reflectSlice(s Slice) reflect.Value {
	rv := reflect.ValueOf(s)
	if rv.Kind() != reflect.Slice {
		panic(&generic.TypeError{rv.Type(), nil, "is not a slice"})
	}
	return rv
}

// Add returns a new Table with a new column bound to data. If Table t
// already has a column with the same name, it is first removed. data
// must have the same length as any existing columns or Add will
// panic.
func (t *Table) Add(name string, data Slice) *Table {
	// TODO: Currently adding N columns is O(N^2). If we built the
	// column index only when it was asked for, the usual case of
	// adding a bunch of columns and then using the final table
	// would be O(N).

	rv := reflectSlice(data)
	dataLen := rv.Len()

	// Create the new table, removing any existing column with the
	// same name.
	nt := &Table{make(map[string]Slice), []string{}, t.len}
	for name2, col2 := range t.cols {
		if name != name2 {
			nt.cols[name2] = col2
			nt.colNames = append(nt.colNames, name2)
		}
	}
	if len(nt.cols) == 0 {
		nt.cols[name] = data
		nt.colNames = append(nt.colNames, name)
		nt.len = dataLen
	} else if nt.len != dataLen {
		panic("cannot add column " + name + " with " + strconv.Itoa(dataLen) + " elements to table with " + strconv.Itoa(nt.len) + " rows")
	} else {
		nt.cols[name] = data
		nt.colNames = append(nt.colNames, name)
	}

	return nt
}

// Len returns the number of rows in Table t.
func (t *Table) Len() int {
	return t.len
}

// Columns returns the names of the columns in Table t, or nil if this
// Table is empty.
func (t *Table) Columns() []string {
	return t.colNames
}

// Column returns the slice of data in column name of Table t, or nil
// if there is no such column.
func (t *Table) Column(name string) Slice {
	return t.cols[name]
}

// MustColumn is like Column, but panics if there is no such column.
func (t *Table) MustColumn(name string) Slice {
	if c := t.Column(name); c != nil {
		return c
	}
	panic("unknown column: " + name)
}

// Groups returns the groups in this Table. If t is empty, there are
// no groups. Otherwise, there is only RootGroupID.
func (t *Table) Groups() []GroupID {
	if t.cols == nil {
		return []GroupID{}
	}
	return []GroupID{RootGroupID}
}

// Table returns t if gid is RootGroupID and t is not empty; otherwise
// it returns nil.
func (t *Table) Table(gid GroupID) *Table {
	if gid == RootGroupID && t.cols != nil {
		return t
	}
	return nil
}

// AddTable returns a Grouping with up to two groups: first, t, if
// non-empty, is bound to RootGroupID; then t2, if non-empty, is bound
// to group gid.
//
// Typically this is used to build up a Grouping by starting with an
// empty Table and adding Tables to it.
func (t *Table) AddTable(gid GroupID, t2 *Table) Grouping {
	if t2.cols == nil {
		return t
	} else if gid == RootGroupID || t.cols == nil {
		return t2
	}

	g := &groupedTable{
		tables:   map[GroupID]*Table{},
		groups:   []GroupID{},
		colNames: nil,
	}
	return g.AddTable(RootGroupID, t).AddTable(gid, t2)
}

func (g *groupedTable) Columns() []string {
	return g.colNames
}

func (g *groupedTable) Groups() []GroupID {
	return g.groups
}

func (g *groupedTable) Table(gid GroupID) *Table {
	return g.tables[gid]
}

func (g *groupedTable) AddTable(gid GroupID, t *Table) Grouping {
	// TODO: Currently adding N tables is O(N^2).

	if t.cols == nil {
		// Adding an empty table has no effect.
		return g
	}

	// Create the new grouped table, removing any existing table
	// with the same GID.
	ng := &groupedTable{map[GroupID]*Table{}, []GroupID{}, g.colNames}
	for gid2, t2 := range g.tables {
		if gid != gid2 {
			ng.tables[gid2] = t2
			ng.groups = append(ng.groups, gid2)
		}
	}
	if len(ng.groups) == 0 {
		ng.tables[gid] = t
		ng.groups = append(ng.groups, gid)
		ng.colNames = t.Columns()
		return ng
	}

	// Check that t's column structure matches.
	for _, col := range ng.colNames {
		if _, ok := t.cols[col]; !ok {
			panic("table missing column: " + col)
		}
	}
	if len(t.cols) != len(ng.colNames) {
		// t has a column the group doesn't.
		colSet := map[string]bool{}
		for _, col := range ng.colNames {
			colSet[col] = true
		}
		for col := range t.cols {
			if !colSet[col] {
				panic("table has extra column: " + col)
			}
		}
	}

	ng.tables[gid] = t
	ng.groups = append(ng.groups, gid)
	return ng
}
