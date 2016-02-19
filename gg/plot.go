// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"log"
	"os"
	"reflect"

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

	env *plotEnv

	scaledData map[scaledDataKey]*scaledData
	scaleSet   map[string]map[Scaler]bool
	marks      []plotMark
}

func NewPlot(data table.Grouping) *Plot {
	p := &Plot{
		env: &plotEnv{
			data:     data,
			bindings: make(map[string]*binding),
			scales:   make(map[string]scalerTree),
		},
		scaledData: make(map[scaledDataKey]*scaledData),
		scaleSet:   make(map[string]map[Scaler]bool),
	}
	return p
}

type plotEnv struct {
	parent   *plotEnv
	data     table.Grouping
	bindings map[string]*binding
	scales   map[string]scalerTree
}

// A binding combines a set of data values (such as numeric values or
// factors) with a set of scales for mapping them to visual properties
// (such as position or color).
type binding struct {
	isConstant bool
	col        string
	constant   interface{}
}

// SetData sets p's current data table. The caller must not modify
// data in this table after this point.
func (p *Plot) SetData(data table.Grouping) *Plot {
	// TODO: Check that all bindings are valid in the new table?
	p.env.data = data
	return p
}

// Data returns p's current data table.
func (p *Plot) Data() table.Grouping {
	return p.env.data
}

// TODO: How to bind constants?

// Bind binds the data in column col to property prop, so layers can
// look up the data bound to properties they use.
//
// By convention, the scale used for a property has the same name as
// that property, but is there is a "_" in the property name, it and
// everything after it are stripped to get the scale name.
//
// Bind returns p for ease of method chaining.
//
// TODO: Now that the binding doesn't track its own scale, this seems
// extra silly.
func (p *Plot) Bind(prop, col string) *Plot {
	data := p.env.data

	// Check col.
	for _, col2 := range data.Columns() {
		if col == col2 {
			goto haveCol
		}
	}
	panic("unknown column: " + col)
haveCol:

	p.env.bindings[prop] = &binding{col: col}
	return p
}

func (p *Plot) unbindAll() *Plot {
	for _, name := range p.bindings() {
		p.env.bindings[name] = nil
	}
	return p
}

// TODO: Make these public. Might want to just return the name, data,
// and scale rather than the internal struct. OTOH, that would make
// bindings() difficult.

func (p *Plot) getBinding(name string) *binding {
	for e := p.env; e != nil; e = e.parent {
		if b, ok := e.bindings[name]; ok {
			return b
		}
	}
	return nil
}

func (p *Plot) mustGetBinding(name string) *binding {
	if b := p.getBinding(name); b != nil {
		return b
	}
	panic("unknown binding: " + name)
}

func (p *Plot) bindings() []string {
	bs := make([]string, 0, len(p.env.bindings))
	have := make(map[string]bool)
	for e := p.env; e != nil; e = e.parent {
		for name, b := range e.bindings {
			if !have[name] {
				have[name] = true
				if b != nil {
					bs = append(bs, name)
				}
			}
		}
	}
	return bs
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

	// TODO: Should scales actually be scoped or global to the
	// plot, given that you can override them anyway? For example,
	// if I create a default scale, I probably want to bind it in
	// the root environment. Also, given that scales are bound to
	// aesthetics, they are independent of what columns/bindings
	// exist. If I take scales out of the scoping and eliminate
	// bindings, then scopes are just about the Plot data (and
	// maybe scale transforms), which probably makes sense since
	// things like stats are data transforms.

	// Bind this scale.
	p.env.scales[aes] = newScalerTree(s)

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
	b    *binding
}

// use marks a binding as in-use by a mark, finds the aesthetic's
// scale (creating it if necessary), adds the binding's data to the
// domain of the scale, and returns the scaled data.
//
// b may be nil, in which case it simply returns nil. This is
// convenient in combination with getBinding for an optional binding.
//
// TODO: Should aes be an enum?
func (p *Plot) use(aes string, b *binding) *scaledData {
	if b == nil {
		return nil
	}

	sd := p.scaledData[scaledDataKey{p.Data(), b}]
	if sd == nil {
		// Construct the scaledData.
		sd = &scaledData{
			seqs: make(map[table.GroupID]scaledSeq),
		}

		// Get the scale tree.
		var scales scalerTree
		for e := p.env; e != nil; e = e.parent {
			if st, ok := e.scales[aes]; ok {
				scales = st
				break
			}
		}

		for _, gid := range p.Data().Tables() {
			var seq table.Slice
			table := p.Data().Table(gid)
			if b.isConstant {
				// Create the sequence.
				len := table.Len()
				sv := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(b.constant)), len, len)
				cv := reflect.ValueOf(b.constant)
				for i := 0; i < len; i++ {
					sv.Index(i).Set(cv)
				}
				seq = sv.Interface()
			} else {
				// Get the data.
				seq = table.MustColumn(b.col)
			}

			// Find or create the scale.
			var scaler Scaler
			if scales.scales == nil {
				var err error
				scaler, err = DefaultScale(seq)
				if err != nil {
					panic(&generic.TypeError{reflect.TypeOf(seq), nil, err.Error()})
				}
				// TODO: If scales were global, I
				// could just call Scale here.
				scales = newScalerTree(scaler)
				rootEnv := p.env
				for rootEnv.parent != nil {
					rootEnv = rootEnv.parent
				}
				rootEnv.scales[aes] = scales
				ss := p.scaleSet[aes]
				if ss == nil {
					ss = make(map[Scaler]bool)
					p.scaleSet[aes] = ss
				}
				ss[scaler] = true
			} else {
				scaler = scales.find(gid)
			}

			// Train the scale.
			scaler.ExpandDomain(seq)

			// Add it to the scaledData.
			sd.seqs[gid] = scaledSeq{seq, scaler}
		}

		p.scaledData[scaledDataKey{p.Data(), b}] = sd
	}

	return sd
}

// Save saves the current data table, bindings, and scales of p to a
// stack. It creates a new scope for declaring bindings and scales:
// all of the existing bindings continue to be available, but new
// bindings can be created in the scope and existing bindings can be
// shadowed by new bindings of the same name.
func (p *Plot) Save() *Plot {
	p.env = &plotEnv{
		parent:   p.env,
		data:     p.env.data,
		bindings: make(map[string]*binding),
		scales:   make(map[string]scalerTree),
	}
	return p
}

// Restore restores the data table, bindings, and scales of p from the
// save stack. That is, it exits the current bindings scope.
func (p *Plot) Restore() *Plot {
	if p.env.parent == nil {
		panic("unbalanced Save/Restore")
	}
	p.env = p.env.parent
	// TODO: Clear unwound bindings from p.used.
	return p
}

// TODO: Maybe Plotter should be an interface so types can satisfy it?
// This might be a nice way to add Bindings to a plot.

type Plotter func(p *Plot)

func (p *Plot) Add(o Plotter) *Plot {
	o(p)
	return p
}
