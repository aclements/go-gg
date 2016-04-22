// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ggstat

import (
	"fmt"
	"reflect"

	"github.com/aclements/go-gg/generic"
	"github.com/aclements/go-gg/table"
	"github.com/aclements/go-moremath/stats"
)

// TODO: AggFirst, AggTukey, AggMin/Max/Sum

// Agg constructs an Aggregate transform from a grouping column and a
// set of Aggregators.
//
// TODO: Does this belong in ggstat? The specific aggregator functions
// probably do, but the concept could go in package table.
func Agg(x string, aggs ...Aggregator) Aggregate {
	return Aggregate{x, aggs}
}

// Aggregate computes aggregate functions of a table grouped by
// distinct values of the X column.
//
// The result of Aggregate will have a column named the same as column
// X that consists of the distinct values of the X column, in addition
// to constant columns from the input. The names of any other columns
// are determined by the Aggregators.
type Aggregate struct {
	// X is the name of the column to group values by before
	// computing aggregate functions.
	//
	// TODO: Make this optional?
	X string

	// Aggregators is the set of Aggregator functions to apply to
	// each group of values.
	Aggregators []Aggregator
}

// An Aggregator is a function that aggregates each group of input
// into one row and adds it to output. It may be based on multiple
// columns from input and may add multiple columns to output.
type Aggregator func(input table.Grouping, output *table.Table) *table.Table

func (s Aggregate) F(g table.Grouping) table.Grouping {
	return table.MapTables(func(_ table.GroupID, t *table.Table) *table.Table {
		g := table.GroupBy(t, s.X)

		// Construct X column.
		rows := len(g.Tables())
		xs := reflect.MakeSlice(reflect.TypeOf(t.MustColumn(s.X)), rows, rows)
		for i, gid := range g.Tables() {
			xs.Index(i).Set(reflect.ValueOf(gid.Label()))
		}

		// Start new table.
		nt := new(table.Table).Add(s.X, xs.Interface())

		// Apply Aggregators.
		for _, agg := range s.Aggregators {
			nt = agg(g, nt)
		}

		// Keep constant columns.
		return preserveConsts(nt, t)
	}, g)
}

// AggCount returns an aggregate function that computes the number of
// rows in each group. The resulting column will be named label, or
// "count" if label is "".
func AggCount(label string) Aggregator {
	if label == "" {
		label = "count"
	}

	return func(input table.Grouping, t *table.Table) *table.Table {
		counts := make([]int, 0, len(input.Tables()))
		for _, gid := range input.Tables() {
			counts = append(counts, input.Table(gid).Len())
		}
		return t.Add(label, counts)
	}
}

// AggMean returns an aggregate function that computes the mean of
// each of cols. The resulting columns will be named "mean <col>".
func AggMean(cols ...string) Aggregator {
	ocols := make([]string, len(cols))
	for i, col := range cols {
		ocols[i] = "mean " + col
	}

	return func(input table.Grouping, t *table.Table) *table.Table {
		for coli, col := range cols {
			means := make([]float64, 0, len(input.Tables()))

			var xs []float64
			for _, gid := range input.Tables() {
				generic.ConvertSlice(&xs, input.Table(gid).MustColumn(col))
				means = append(means, stats.Mean(xs))
			}

			t = t.Add(ocols[coli], means)
		}
		return t
	}
}

// AggUnique returns an aggregate function that computes the unique
// value of each of cols. If a given group contains more than one
// value, it panics.
func AggUnique(cols ...string) Aggregator {
	return func(input table.Grouping, t *table.Table) *table.Table {
		if len(cols) == 0 {
			return t
		}
		if len(input.Tables()) == 0 {
			panic(fmt.Sprintf("unknown column: %q", cols[0]))
		}
		i0 := input.Table(input.Tables()[0])

		for _, col := range cols {
			ctype := reflect.TypeOf(i0.MustColumn(col))
			rows := len(input.Tables())
			vs := reflect.MakeSlice(ctype, rows, rows)
			for i, gid := range input.Tables() {
				// Get values in this column.
				xs := reflect.ValueOf(input.Table(gid).MustColumn(col))

				// Check for uniqueness.
				if xs.Len() == 0 {
					panic(fmt.Sprintf("cannot AggUnique empty column %q", col))
				}
				uniquev := xs.Index(0)
				unique := uniquev.Interface()
				for i, len := 1, xs.Len(); i < len; i++ {
					other := xs.Index(i).Interface()
					if unique != other {
						panic(fmt.Sprintf("column %q is not unique; contains at least %v and %v", col, unique, other))
					}
				}

				// Store unique value.
				vs.Index(i).Set(uniquev)
			}

			// Add unique values slice to output table.
			t = t.Add(col, vs.Interface())
		}

		return t
	}
}
