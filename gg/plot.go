// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aclements/go-gg/table"
)

// TODO: Split transforms, scalers, and layers into their own packages
// to clean up the name spaces and un-prefix their names?

// Warning is a logger for reporting conditions that don't prevent the
// production of a plot, but may lead to unexpected results.
var Warning = log.New(os.Stderr, "[gg] ", log.Lshortfile)

type Plot struct {
	//facet  facet

	env    *plotEnv
	scales map[string]scalerTree

	scaledData map[scaledDataKey]*scaledData
	scaleSet   map[scaleKey]bool
	marks      []plotMark

	axisLabels map[string][]string
}

func NewPlot(data table.Grouping) *Plot {
	p := &Plot{
		env: &plotEnv{
			data: data,
		},
		scales:     make(map[string]scalerTree),
		scaledData: make(map[scaledDataKey]*scaledData),
		scaleSet:   make(map[scaleKey]bool),
		axisLabels: make(map[string][]string),
	}
	return p
}

type plotEnv struct {
	parent *plotEnv
	data   table.Grouping
}

type scaleKey struct {
	gid   table.GroupID
	aes   string
	scale Scaler
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

func newScalerTree() scalerTree {
	return scalerTree{map[table.GroupID]Scaler{
		table.RootGroupID: &defaultScale{},
	}}
}

func (t scalerTree) bind(gid table.GroupID, s Scaler) {
	// Unbind scales under gid.
	for ogid := range t.scales {
		if gid == table.RootGroupID {
			// Optimize binding the root GID.
			delete(t.scales, ogid)
			continue
		}

		for p := ogid; ; p = p.Parent() {
			if p == gid {
				delete(t.scales, ogid)
				break
			}
			if p == table.RootGroupID {
				break
			}
		}
	}
	t.scales[gid] = s
}

func (t scalerTree) find(gid table.GroupID) Scaler {
	for {
		if s, ok := t.scales[gid]; ok {
			return s
		}
		if gid == table.RootGroupID {
			// This should never happen.
			panic("no scale for group " + gid.String())
		}
		gid = gid.Parent()
	}
}

func (p *Plot) getScales(aes string) scalerTree {
	st, ok := p.scales[aes]
	if !ok {
		st = newScalerTree()
		p.scales[aes] = st
	}
	return st
}

// SetScale binds a scale to the given visual aesthetic. SetScale is
// shorthand for SetScaleAt(aes, s, table.RootGroupID).
//
// SetScale returns p for ease of chaining.
func (p *Plot) SetScale(aes string, s Scaler) *Plot {
	return p.SetScaleAt(aes, s, table.RootGroupID)
}

// SetScaleAt binds a scale to the given visual aesthetic for all data
// in group gid or descendants of gid.
func (p *Plot) SetScaleAt(aes string, s Scaler, gid table.GroupID) *Plot {
	// TODO: Should aes be an enum so you can't mix up aesthetics
	// and column names?
	p.getScales(aes).bind(gid, s)
	return p
}

// GetScale returns the scale for the given visual aesthetic used for
// data in group gid.
func (p *Plot) GetScale(aes string, gid table.GroupID) Scaler {
	return p.getScales(aes).find(gid)
}

type scaledDataKey struct {
	aes  string
	data table.Grouping
	col  string
}

// use binds a column of data to an aesthetic. It expands the domain
// of the aesthetic's scale to include the data in col, and returns
// the scaled data.
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

	// TODO: This is wrong. If the scale tree for aes changes,
	// this may return a stale scaledData bound to the wrong
	// scalers. If I got rid of scale trees, I could just put the
	// scaler in the key. Or I could clean up the cache when the
	// scale tree changes.

	sd := p.scaledData[scaledDataKey{aes, p.Data(), col}]
	if sd == nil {
		// Construct the scaledData.
		sd = &scaledData{
			seqs: make(map[table.GroupID]scaledSeq),
		}

		// Get the scale tree.
		st := p.getScales(aes)

		for _, gid := range p.Data().Tables() {
			table := p.Data().Table(gid)

			// Get the data.
			seq := table.MustColumn(col)

			// Find the scale.
			scaler := st.find(gid)

			// Add the scale to the scale set.
			p.scaleSet[scaleKey{gid, aes, scaler}] = true

			// Train the scale.
			scaler.ExpandDomain(seq)

			// Add it to the scaledData.
			sd.seqs[gid] = scaledSeq{seq, scaler}
		}

		p.scaledData[scaledDataKey{aes, p.Data(), col}] = sd
	}

	// Update axis labels.
	if aes == "x" || aes == "y" {
		p.axisLabels[aes] = append(p.axisLabels[aes], col)
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

func (p *Plot) Add(plotters ...Plotter) *Plot {
	for _, plotter := range plotters {
		plotter.Apply(p)
	}
	return p
}
