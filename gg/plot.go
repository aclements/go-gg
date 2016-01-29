// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// Warning is a logger for reporting conditions that don't prevent the
// production of a plot, but may lead to unexpected results.
var Warning = log.New(os.Stderr, "[gg]", log.Lshortfile)

type Plot struct {
	//facet  facet

	env *plotEnv

	aesMap map[string]map[*binding]bool
	marks  []marker
}

func NewPlot() *Plot {
	p := &Plot{
		aesMap: make(map[string]map[*binding]bool),
	}
	return p.Save()
}

type plotEnv struct {
	parent   *plotEnv
	bindings map[string]*binding
}

// A binding represents the mapping of data values (such as numeric
// values or factors) to visual properties (such as position or
// color) using a particular scale.
type binding struct {
	name  string
	data  Var
	scale Scaler

	// used indicates that this binding is used by a mark and that
	// scale has been trained with data.
	used bool
}

// Bind is shorthand for p.BindWithScale(prop, data, nil). It binds
// "data" to visual property "prop" using a default scale.
func (p *Plot) Bind(prop string, data interface{}) *Plot {
	return p.BindWithScale(prop, data, nil)
}

// BindWithScale binds "data" to visual property "prop", using the
// specified scale to map between data values and visual values.
//
// data may be a Var or anything that can be accepted by AutoVar. The
// caller must not modify data after binding it.
//
// XXX Because of things like grouping properties, "prop" isn't
// necessarily a visual property. It's marks that interpret bindings
// as visual properties.
//
// scale may be nil, in which case this will pick a scale. If prop is
// of the form "𝑎_𝑏" and there is an existing binding named "𝑎", this
// binding will inherit 𝑎's scale. This is used to make properties
// like "y" and "y_2" map to the same scale. Otherwise, BindWithScale
// will chose a default scale based on the type of data.
//
// BindWithScale returns p for ease of method chaining.
func (p *Plot) BindWithScale(prop string, data interface{}, scale Scaler) *Plot {
	if scale == nil {
		if i := strings.Index(prop, "_"); i >= 0 {
			// Find the scale of the base binding, if any.
			scaleName := prop[:i]
			if m := p.getBinding(scaleName); m != nil {
				scale = m.scale
			}
		}

		if scale == nil {
			// Choose the default scale for this data.
			var err error
			scale, err = DefaultScale(data)
			if err != nil {
				panic(err)
			}
		}
	}

	p.env.bindings[prop] = &binding{name: prop, data: AutoVar(data), scale: scale}
	return p
}

// TODO: Make these public. Might want to just return the name, data,
// and scale rather than the internal struct. OTOH, that would make
// bindings() difficult.

func (p *Plot) getBinding(name string) *binding {
	for e := p.env; e != nil; e = e.parent {
		if m, ok := e.bindings[name]; ok {
			return m
		}
	}
	return nil
}

func (p *Plot) mustGetBinding(name string) *binding {
	if b := p.getBinding(name); b != nil {
		return b
	}
	panic(fmt.Sprintf("unknown binding %q", name))
}

func (p *Plot) bindings() []*binding {
	bs := make([]*binding, 0, len(p.env.bindings))
	have := make(map[string]bool)
	for e := p.env; e != nil; e = e.parent {
		for name, b := range e.bindings {
			if !have[name] {
				have[name] = true
				bs = append(bs, b)
			}
		}
	}
	return bs
}

// use marks a binding as in-use by a mark and attaches it to a
// rendering aesthetic. This adds the binding's data to the domain of
// the binding's scale.
//
// TODO: Should aes be an enum?
func (p *Plot) use(aes string, b *binding) *Plot {
	// Train the scale.
	if !b.used {
		b.scale.ExpandDomain(b.data)
		b.used = true
	}

	// Add it to the aesthetic mappings.
	am := p.aesMap[aes]
	if am == nil {
		am = make(map[*binding]bool)
		p.aesMap[aes] = am
	}
	am[b] = true

	return p
}

// Save saves the current bindings of p to a stack. This effectively
// creates a new scope for declaring bindings: all of the existing
// bindings continue to be available, but new bindings can be created
// in the scope and existing bindings can be shadowed by new bindings
// of the same name.
func (p *Plot) Save() *Plot {
	p.env = &plotEnv{parent: p.env, bindings: make(map[string]*binding)}
	return p
}

// Restore restores the bindings of p from the save stack. The
// effectively exits the current bindings scope.
func (p *Plot) Restore() *Plot {
	if p.env.parent == nil {
		panic("unbalanced Save/Restore")
	}
	p.env = p.env.parent
	return p
}

type Plotter func(p *Plot)

func (p *Plot) Add(o Plotter) *Plot {
	o(p)
	return p
}
