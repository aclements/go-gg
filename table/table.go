// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package table implements ordered, grouped two dimensional relations.
//
// Package table provides two related abstractions: Table and Grouped.
//
// A Table is an ordered relation of rows and columns. Each column is
// a Go slice and hence must be homogeneously typed, but different
// columns may have different types. All columns in a Table have the
// same number of rows.
//
// A Grouped generalizes a Table by grouping the Table's rows into
// zero or more groups. A Table is itself a Grouped with only one
// group. Most operations take a Grouped and operate on each group
// independently, though some operations sub-divide or combine groups.
//
// The structures of both Tables and Groupeds are immutable. Adding a
// column to a Table returns a new Table and adding a new Table
// (group) to a Grouped returns a new Grouped.
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
// A Table is also a trivial Grouped consisting of only a root group.
//
// The zero value of Table is an empty table with no rows and no
// columns.
//
// A Table's structure is immutable. To construct a Table, start with
// an empty table and add columns to it using Add.
type Table struct {
	cols     map[string]Slice
	colNames []string
	len      int
}

// TODO: Could I arrange this so that an empty table is also the empty
// group? That would require tweaking what it means to add an empty
// Table to a group and also tweaking what groups an Empty table
// contains.

// A Grouped is a set of tables with identical sets of columns, each
// identified by a distinct GroupID.
//
// Visually, a Grouped can be thought of as follows:
//
//	   Col A  Col B  Col C
//	------- group a ------
//	1   5.4    "x"     90
//	2   -.2    "y"     30
//	------- group b ------
//	1   9.3    "a"     10
//
// Like a Table, a Grouped's structure is immutable. To construct a
// Grouped, start with either an EmptyGrouped or a *Table and add
// tables to it using AddTable.
//
// Despite the fact that GroupIDs form a hierarchy, a Grouped ignores
// this hierarchy and simply operates on a flat map of distinct
// GroupIDs to Tables.
type Grouped interface {
	// Columns returns the names of the columns in this Grouped,
	// or nil if there are no Tables or the group consists solely
	// of empty Tables. All Tables in this Grouped have the same
	// set of columns.
	Columns() []string

	// Groups returns the IDs of the groups in this Grouped.
	Groups() []GroupID

	// Table returns the Table in group gid, or nil if there is no
	// such Table.
	Table(gid GroupID) *Table

	// AddTable returns a new Grouped with Table t bound to group
	// gid. If this group already exists, it is first removed.
	// Table t must have the same set of columns as any existing
	// Tables in this group or AddTable will panic.
	//
	// TODO The same set or the same sequence of columns? Given
	// that I never use the sequence (except maybe for printing),
	// perhaps it doesn't matter.
	//
	// TODO This doesn't make it easy to combine two Groupeds. It
	// could instead take a Grouped and reparent it.
	AddTable(gid GroupID, t *Table) Grouped
}

type groupedTable struct {
	tables   map[GroupID]*Table
	groups   []GroupID
	colNames []string
}

// EmptyGrouped is a Grouped with no tables.
var EmptyGrouped = emptyGrouped()

func emptyGrouped() *groupedTable {
	// Hide this initializer from the documentation.
	return &groupedTable{
		tables:   map[GroupID]*Table{},
		groups:   []GroupID{},
		colNames: nil,
	}
}

// A Slice is a Go slice value.
//
// This is primarily for documentation. There is no way to statically
// enforce this in Go; however, operations that expect a Slice will
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

// Groups returns only RootGroupID, since a Table is a Grouped
// consisting of a single group.
func (t *Table) Groups() []GroupID {
	return []GroupID{RootGroupID}
}

// Table returns t if gid is RootGroupID; otherwise it returns nil.
func (t *Table) Table(gid GroupID) *Table {
	if gid == RootGroupID {
		return t
	}
	return nil
}

// AddTable returns a Grouped with at most two groups: t in
// RootGroupID and t2 in group gid. If gid is RootGroupID, this
// returns a Grouped consisting only of t2.
//
// This exists so Table satisfies Grouped. Typically Grouped tables
// should be built starting from EmptyGrouped.
func (t *Table) AddTable(gid GroupID, t2 *Table) Grouped {
	if gid == RootGroupID {
		return EmptyGrouped.AddTable(gid, t2)
	}
	return EmptyGrouped.AddTable(RootGroupID, t).AddTable(gid, t2)
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

func (g *groupedTable) AddTable(gid GroupID, t *Table) Grouped {
	// TODO: Currently adding N tables is O(N^2).

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
