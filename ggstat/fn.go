// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ggstat

import (
	"math"
	"reflect"

	"github.com/aclements/go-gg/generic/slice"
	"github.com/aclements/go-gg/table"
	"github.com/aclements/go-moremath/stats"
	"github.com/aclements/go-moremath/vec"
)

// Function samples a continuous univariate function.
//
// Function finds the range over which to sample Fn based on the data
// in the X column and the values of Widen and SplitGroups. It samples
// Fn at N evenly spaced values within the range.
//
// The result of Function binds column X to the X values at which the
// function is sampled and retains constant columns from the input.
// The computed function can add arbitrary columns for its output.
type Function struct {
	// X is the name of the column to use for input domain of this
	// function.
	X string

	// N is the number of points to sample the function at. If N
	// is 0, a reasonable default is used.
	N int

	// Widen sets the range over which Fn is sampled to Widen
	// times the span of the data. If Widen is 0, it is treated as
	// 1.1 (that is, widen the domain by 10%, or 5% on the left
	// and 5% on the right).
	//
	// TODO: Have a way to specify a specific range?
	Widen float64

	// SplitGroups indicates that each group in the table should
	// have separate bounds based on the data in that group alone.
	// The default, false, indicates that the bounds should be
	// based on all of the data in the table combined. This makes
	// it possible to stack functions and easier to compare them
	// across groups.
	SplitGroups bool

	// Fn is the continuous univariate function to sample. Fn will
	// be called with each table in the grouping and the X values
	// at which it should be sampled. Fn must add its output
	// columns to out. The output table will already contain the
	// sample points bound to the X column.
	Fn func(gid table.GroupID, in *table.Table, sampleAt []float64, out *table.Builder)
}

const (
	defaultFunctionSamples = 200
	defaultWiden           = 1.1
)

func (f Function) F(g table.Grouping) table.Grouping {
	// Set defaults.
	if f.N <= 0 {
		f.N = defaultFunctionSamples
	}
	widen := f.Widen
	if widen <= 0 {
		widen = defaultWiden
	}

	var xs []float64
	var gmin, gmax float64
	if !f.SplitGroups {
		// Compute combined bounds.
		gmin, gmax = math.NaN(), math.NaN()
		for _, gid := range g.Tables() {
			t := g.Table(gid)
			slice.Convert(&xs, t.MustColumn(f.X))
			xmin, xmax := stats.Bounds(xs)
			if xmin < gmin || math.IsNaN(gmin) {
				gmin = xmin
			}
			if xmax > gmax || math.IsNaN(gmax) {
				gmax = xmax
			}
		}

		// Widen bounds.
		span := gmax - gmin
		gmin, gmax = gmin-span*(widen-1)/2, gmax+span*(widen-1)/2
	}

	return table.MapTables(g, func(gid table.GroupID, t *table.Table) *table.Table {
		var min, max float64
		if !f.SplitGroups {
			min, max = gmin, gmax
		} else {
			// Compute bounds.
			slice.Convert(&xs, t.MustColumn(f.X))
			min, max = stats.Bounds(xs)

			// Widen bounds.
			span := max - min
			min, max = min-span*(widen-1)/2, max+span*(widen-1)/2
		}

		// Compute sample points. If there's no data, there
		// are no sample points, but we still have to run the
		// function to get the right output columns.
		var ss []float64
		if math.IsNaN(min) {
			ss = []float64{}
		} else {
			ss = vec.Linspace(min, max, f.N)
		}

		var nt table.Builder
		ctype := table.ColType(t, f.X)
		if ctype == float64Type {
			// Bind output X column.
			nt.Add(f.X, ss)
		} else {
			// Convert to the column type.
			vsp := reflect.New(ctype)
			slice.Convert(vsp.Interface(), ss)
			vs := vsp.Elem()
			// This may have produced duplicate values.
			// Eliminate those.
			if vs.Len() > 0 {
				prev, i := vs.Index(0).Interface(), 1
				for j := 1; j < vs.Len(); j++ {
					next := vs.Index(j).Interface()
					if prev == next {
						// Skip duplicate.
						continue
					}

					if i != j {
						vs.Index(i).Set(vs.Index(j))
					}
					i++
					prev = next
				}
				vs.SetLen(i)
			}
			// Bind column-typed values to output X.
			nt.Add(f.X, vs.Interface())
			// And convert back to []float64 so we can
			// apply the function.
			slice.Convert(&ss, vs.Interface())
		}

		// Apply the function to the sample points.
		f.Fn(gid, t, ss, &nt)

		preserveConsts(&nt, t)
		return nt.Done()
	})
}

// preserveConsts copies the constant columns from t into nt.
func preserveConsts(nt *table.Builder, t *table.Table) {
	for _, col := range t.Columns() {
		if nt.Has(col) {
			// Don't overwrite existing columns in nt.
			continue
		}
		if cv, ok := t.Const(col); ok {
			nt.AddConst(col, cv)
		}
	}
}

type colInfo struct {
	data     []float64
	min, max float64
}

// getCol extracts column x from each group, converts it to []float64,
// and finds its bounds.
//
// TODO: Maybe this should be a callback interface to avoid building
// the map and holding on to so much allocation?
func getCol(g table.Grouping, x string, widen float64, splitGroups bool) map[table.GroupID]colInfo {
	if widen <= 0 {
		widen = 1.1
	}

	col := make(map[table.GroupID]colInfo)

	if !splitGroups {
		// Compute combined bounds.
		min, max := math.NaN(), math.NaN()
		for _, gid := range g.Tables() {
			var xs []float64
			t := g.Table(gid)
			slice.Convert(&xs, t.MustColumn(x))
			xmin, xmax := stats.Bounds(xs)
			if xmin < min || math.IsNaN(min) {
				min = xmin
			}
			if xmax > max || math.IsNaN(max) {
				max = xmax
			}
			col[gid] = colInfo{xs, 0, 0}
		}

		// Widen bounds.
		span := max - min
		min, max = min-span*(widen-1)/2, max+span*(widen-1)/2

		for gid, info := range col {
			info.min, info.max = min, max
			col[gid] = info
		}

		return col
	}

	// Find bounds for each group separately.
	for _, gid := range g.Tables() {
		t := g.Table(gid)

		// Compute bounds.
		var xs []float64
		slice.Convert(&xs, t.MustColumn(x))
		min, max := stats.Bounds(xs)

		// Widen bounds.
		span := max - min
		min, max = min-span*(widen-1)/2, max+span*(widen-1)/2

		col[gid] = colInfo{xs, min, max}
	}
	return col
}
