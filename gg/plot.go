// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"

	"github.com/aclements/go-gg/generic"
	"github.com/aclements/go-gg/table"
)

// TODO: Split transforms, scalers, and layers into their own packages
// to clean up the name spaces and un-prefix their names?

// Warning is a logger for reporting conditions that don't prevent the
// production of a plot, but may lead to unexpected results.
var Warning = log.New(os.Stderr, "[gg]", log.Lshortfile)

type Plot struct {
	//facet  facet

	env    *plotEnv
	scales map[string]scalerTree

	scaledData map[scaledDataKey]*scaledData
	scaleSet   map[string]map[Scaler]bool
	marks      []plotMark
}

func NewPlot(data table.Grouping) *Plot {
	p := &Plot{
		env: &plotEnv{
			data: data,
		},
		scales:     make(map[string]scalerTree),
		scaledData: make(map[scaledDataKey]*scaledData),
		scaleSet:   make(map[string]map[Scaler]bool),
	}
	return p
}

type plotEnv struct {
	parent *plotEnv
	data   table.Grouping
}

// SetData sets p's current data table. The caller must not modify
// data in this table after this point.
func (p *Plot) SetData(data table.Grouping) *Plot {
	p.env.data = data
	return p
}

// Data returns p's current data table.
func (p *Plot) Data() table.Grouping {
	return p.env.data
}

type scalerTree struct {
	scales map[table.GroupID]Scaler
}

func newScalerTree(s Scaler) scalerTree {
	// TODO: If I already have a faceting, perhaps this needs to
	// re-apply the faceting to s to split up this scale.
	return scalerTree{map[table.GroupID]Scaler{
		table.RootGroupID: s,
	}}
}

func (t scalerTree) find(gid table.GroupID) Scaler {
	for {
		if s, ok := t.scales[gid]; ok {
			return s
		}
		if gid == table.RootGroupID {
			// TODO: This could happen if an operation
			// that creates non-root scales (faceting) and
			// then flattened the data or set new data.
			// Maybe setting new data needs to check that
			// all scales still apply?
			panic("no scale for group " + gid.String())
		}
		gid = gid.Parent()
	}
}

// Scale binds a scale to the given visual aesthetic.
//
// Scale returns p for ease of chaining.
func (p *Plot) Scale(aes string, s Scaler) *Plot {
	// TODO: Should aes be an enum so you can't mix up aesthetics
	// and column names?

	// Bind this scale.
	p.scales[aes] = newScalerTree(s)

	// Add s to aes's scale set.
	ss := p.scaleSet[aes]
	if ss == nil {
		ss = make(map[Scaler]bool)
		p.scaleSet[aes] = ss
	}
	ss[s] = true
	return p
}

type scaledDataKey struct {
	data table.Grouping
	col  string
}

// use binds a column of data to an aesthetic. It creates the
// aesthetic's scale if necessary, expands its domain to include the
// data in col, and returns the scaled data.
//
// col may be "", in which case it simply returns nil.
//
// TODO: Should aes be an enum?
func (p *Plot) use(aes string, col string) *scaledData {
	if col == "" {
		return nil
	}

	if col[0] == '@' {
		// TODO: Document this.
		n, err := strconv.Atoi(col[1:])
		if err == nil {
			cols := p.Data().Columns()
			if n >= len(cols) {
				panic(fmt.Sprintf("column index %d out of range; table has %d columns", n, len(cols)))
			}
			col = cols[n]
		}
	}

	sd := p.scaledData[scaledDataKey{p.Data(), col}]
	if sd == nil {
		// Construct the scaledData.
		sd = &scaledData{
			seqs: make(map[table.GroupID]scaledSeq),
		}

		// Get the scale tree.
		scales := p.scales[aes]

		for _, gid := range p.Data().Tables() {
			table := p.Data().Table(gid)

			// Get the data.
			seq := table.MustColumn(col)

			// Find or create the scale.
			var scaler Scaler
			if scales.scales == nil {
				var err error
				scaler, err = DefaultScale(seq)
				if err != nil {
					panic(&generic.TypeError{reflect.TypeOf(seq), nil, err.Error()})
				}
				p.Scale(aes, scaler)
				scales = p.scales[aes]
			}
			scaler = scales.find(gid)

			// Train the scale.
			scaler.ExpandDomain(seq)

			// Add it to the scaledData.
			sd.seqs[gid] = scaledSeq{seq, scaler}
		}

		p.scaledData[scaledDataKey{p.Data(), col}] = sd
	}

	return sd
}

// Save saves the current data table of p to a stack.
func (p *Plot) Save() *Plot {
	p.env = &plotEnv{
		parent: p.env,
		data:   p.env.data,
	}
	return p
}

// Restore restores the data table of p from the save stack.
func (p *Plot) Restore() *Plot {
	if p.env.parent == nil {
		panic("unbalanced Save/Restore")
	}
	p.env = p.env.parent
	return p
}

type Plotter interface {
	Apply(*Plot)
}

func (p *Plot) Add(plotter Plotter) *Plot {
	plotter.Apply(p)
	return p
}
