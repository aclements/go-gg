// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gg

import (
	"fmt"
	"image/color"
	"math"
	"reflect"

	"github.com/aclements/go-gg/generic"
	"github.com/aclements/go-gg/table"
	"github.com/aclements/go-moremath/scale"
)

// Continuous -> Interpolatable? Definitely.
//
// Continuous -> Discrete? Can always discretize the input either in
// value order or in index order. In this case the transform (linear,
// log, etc) doesn't matter as long as it's order-preserving. OTOH, a
// continuous input scale can be asked to map *any* value of its input
// type, but if I do this I can only map values that were trained.
// That suggests that I have to just bin the range to do this mapping.
//
// Discrete -> Interpolatable? Pick evenly spaced values on [0,1].
//
// Discrete -> Discrete? Definitely. Cycle the range if it's not long
// enough. If the input range is a VarNominal, concatenate the
// sequences and use index ordering.
//
// It's not really "continuous", it's more specifically cardinal.

// XXX
//
// A Scaler can be cardinal, discrete, or identity.
//
// A cardinal Scaler has a VarCardinal input domain. If its output
// range is continuous, it maps an interval over the input to an
// interval of the output (possibly through a transformation such as a
// logarithm). If its output range is discrete, the input is
// discretized in value order and it acts like a discrete scale.
//
// XXX The cardinal -> discrete rule means we need to keep all of the
// input data, rather than just its bounds, just in case the range is
// discrete. Maybe it should just be a bucketing rule?
//
// A discrete Scaler has a VarNominal input domain. If the input is
// VarOrdinal, its order is used; otherwise, index order is imposed.
// If the output range is continuous, a discrete Scaler maps its input
// to the centers of equal sub-intervals of [0, 1] and then applies
// the Ranger. If the output range is discrete, the Scaler maps the
// Nth input level to the N%len(range)th output value.
//
// An identity Scaler ignores its input domain and output range and
// uses an identity function for mapping input to output. This is
// useful for specifying aesthetics directly, such as color or size,
// and is especially useful for constant Vars.
//
// XXX Should identity Scalers map numeric types to float64? Maybe it
// should depend on the range type of the ranger?
//
// XXX Arrange documentation as X -> Y?
type Scaler interface {
	// XXX

	ExpandDomain(table.Slice)

	// Ranger sets this Scaler's output range if r is non-nil and
	// returns the previous range.
	Ranger(r Ranger) Ranger

	// XXX Should RangeType be implied by the aesthetic?
	//
	// XXX Should this be a method of Ranger instead?
	RangeType() reflect.Type

	// XXX
	//
	// x must be of the same type as the values in the domain Var.
	//
	// XXX Or should this take a slice? Or even a Var? That would
	// also eliminate RangeType(), though then Map would need to
	// know how to make the right type of return slice. Unless we
	// pushed slice mapping all the way to Ranger.
	//
	// XXX We could eliminate ExpandDomain if the caller was
	// required to pass everything to this at once and this did
	// the scale training. That would also make it easy to
	// implement the cardinal -> discrete by value order rule.
	// This would probably also make Map much faster.
	Map(x interface{}) interface{}

	// XXX What should this return? moremath returns values in the
	// input space, but that obviously doesn't work for discrete
	// scales if I want the ticks between values. It could return
	// values in the intermediate space or the output space.
	// Intermediate space works for continuous and discrete
	// inputs, but not for discrete ranges (maybe that's okay).
	// Output space is bad because I change the plot location in
	// the course of layout. Currently it returns values in the
	// input space or nil if ticks don't make sense.
	Ticks(n int) (major, minor table.Slice, labels []string)

	CloneScaler() Scaler
}

type ContinuousScaler interface {
	Scaler

	// TODO: There are two variations on min/max. 1) We can force
	// the min/max, even if there's data beyond it. 2) We can say
	// min/max has to be at least something, but data can expand
	// beyond it. In the latter case, maybe min/max doesn't matter
	// and it's just "include this point".

	SetMin(v float64) ContinuousScaler
	SetMax(v float64) ContinuousScaler
	Include(v float64) ContinuousScaler
}

var float64Type = reflect.TypeOf(float64(0))
var colorType = reflect.TypeOf((*color.Color)(nil)).Elem()

var canCardinal = map[reflect.Kind]bool{
	reflect.Float32: true,
	reflect.Float64: true,
	reflect.Int:     true,
	reflect.Int8:    true,
	reflect.Int16:   true,
	reflect.Int32:   true,
	reflect.Int64:   true,
	reflect.Uint:    true,
	reflect.Uintptr: true,
	reflect.Uint8:   true,
	reflect.Uint16:  true,
	reflect.Uint32:  true,
	reflect.Uint64:  true,
}

func isCardinal(k reflect.Kind) bool {
	// XXX Move this to generic.IsCardinalR and rename CanOrderR
	// to IsOrderedR. Does complex count? It supports most
	// arithmetic operators. Maybe cardinal is a plot concept and
	// not a generic concept? If sort.Interface influences this,
	// this may need to be a question about a Slice, not a
	// reflect.Kind.
	return canCardinal[k]
}

type defaultScale struct {
	scale Scaler
}

func (s *defaultScale) String() string {
	return fmt.Sprintf("default (%s)", s.scale)
}

func (s *defaultScale) ExpandDomain(v table.Slice) {
	if s.scale == nil {
		var err error
		s.scale, err = DefaultScale(v)
		if err != nil {
			panic(&generic.TypeError{reflect.TypeOf(v), nil, err.Error()})
		}
	}
	s.scale.ExpandDomain(v)
}

func (s *defaultScale) ensure() Scaler {
	if s.scale == nil {
		s.scale = NewLinearScaler()
	}
	return s.scale
}

func (s *defaultScale) Ranger(r Ranger) Ranger {
	// TODO: It would be nice if a package user could grab a scale
	// and set a ranger on it without necessarily defaulting it to
	// a linear scale.
	return s.ensure().Ranger(r)
}

func (s *defaultScale) RangeType() reflect.Type {
	return s.ensure().RangeType()
}

func (s *defaultScale) Map(x interface{}) interface{} {
	return s.ensure().Map(x)
}

func (s *defaultScale) Ticks(n int) (major, minor table.Slice, labels []string) {
	return s.ensure().Ticks(n)
}

func (s *defaultScale) CloneScaler() Scaler {
	if s.scale == nil {
		return &defaultScale{}
	}
	return &defaultScale{s.scale.CloneScaler()}
}

func DefaultScale(seq table.Slice) (Scaler, error) {
	// Handle common case types.
	switch seq.(type) {
	case []float64, []int, []uint:
		return NewLinearScaler(), nil

	case []string:
		// TODO: Ordinal scale
	}

	rt := reflect.TypeOf(seq).Elem()
	rtk := rt.Kind()

	switch {
	case rt.Implements(colorType):
		// For things that are already visual values, use an
		// identity scale.
		return NewIdentityScale(), nil

		// TODO: GroupAuto needs to make similar
		// cardinal/ordinal/nominal decisions. Deduplicate
		// these better.
	case isCardinal(rtk):
		return NewLinearScaler(), nil

	case generic.CanOrderR(rtk):
		return NewOrdinalScale(), nil

	case rt.Comparable():
		// TODO: Nominal scale
		panic("not implemented")
	}

	return nil, fmt.Errorf("no default scale type for %T", seq)
}

func NewIdentityScale() Scaler {
	return &identityScale{}
}

type identityScale struct {
	rangeType reflect.Type
}

func (s *identityScale) ExpandDomain(v table.Slice) {
	s.rangeType = reflect.TypeOf(v).Elem()
}

func (s *identityScale) RangeType() reflect.Type {
	return s.rangeType
}

func (s *identityScale) Ranger(r Ranger) Ranger        { return nil }
func (s *identityScale) Map(x interface{}) interface{} { return x }

func (s *identityScale) Ticks(n int) (major, minor table.Slice, labels []string) {
	return nil, nil, nil
}

func (s *identityScale) CloneScaler() Scaler {
	s2 := *s
	return &s2
}

// NewLinearScaler returns a continuous linear scale. The domain must
// be a VarCardinal.
//
// XXX If I return a Scaler, I can't have methods for setting fixed
// bounds and such. I don't really want to expose the whole type.
// Maybe a sub-interface for continuous Scalers?
func NewLinearScaler() ContinuousScaler {
	// TODO: Control over base.
	return &linearScale{
		s:       scale.Linear{Min: math.NaN(), Max: math.NaN()},
		dataMin: math.NaN(),
		dataMax: math.NaN(),
	}
}

type linearScale struct {
	s scale.Linear
	r Ranger

	domainType       reflect.Type
	dataMin, dataMax float64
}

func (s *linearScale) String() string {
	return fmt.Sprintf("linear [%g,%g] => %s", s.s.Min, s.s.Max, s.r)
}

func (s *linearScale) ExpandDomain(v table.Slice) {
	if s.domainType == nil {
		s.domainType = reflect.TypeOf(v).Elem()
	}

	var data []float64
	generic.ConvertSlice(&data, v)
	min, max := s.dataMin, s.dataMax
	for _, v := range data {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		if v < min || math.IsNaN(min) {
			min = v
		}
		if v > max || math.IsNaN(max) {
			max = v
		}
	}
	s.dataMin, s.dataMax = min, max
}

func (s *linearScale) SetMin(v float64) ContinuousScaler {
	s.s.Min = v
	return s
}

func (s *linearScale) SetMax(v float64) ContinuousScaler {
	s.s.Max = v
	return s
}

func (s *linearScale) Include(v float64) ContinuousScaler {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return s
	}
	if math.IsNaN(s.dataMin) {
		s.dataMin, s.dataMax = v, v
	} else {
		s.dataMin = math.Min(s.dataMin, v)
		s.dataMax = math.Max(s.dataMax, v)
	}
	return s
}

func (s *linearScale) get() scale.Linear {
	ls := s.s
	if ls.Min > ls.Max {
		ls.Min, ls.Max = ls.Max, ls.Min
	}
	if math.IsNaN(ls.Min) {
		ls.Min = s.dataMin
	}
	if math.IsNaN(ls.Max) {
		ls.Max = s.dataMax
	}
	if math.IsNaN(ls.Min) {
		// Only possible if both dataMin and dataMax are NaN.
		ls.Min, ls.Max = -1, 1
	}
	return ls
}

func (s *linearScale) Ranger(r Ranger) Ranger {
	old := s.r
	if r != nil {
		s.r = r
	}
	return old
}

func (s *linearScale) RangeType() reflect.Type {
	return s.r.RangeType()
}

func (s *linearScale) Map(x interface{}) interface{} {
	ls := s.get()
	f64 := reflect.TypeOf(float64(0))
	v := reflect.ValueOf(x).Convert(f64).Float()
	scaled := ls.Map(v)
	switch r := s.r.(type) {
	case ContinuousRanger:
		return r.Map(scaled)

	case DiscreteRanger:
		_, levels := r.Levels()
		// Bin the scaled value into 'levels' bins.
		level := int(scaled * float64(levels))
		if level < 0 {
			level = 0
		} else if level >= levels {
			level = levels - 1
		}
		return r.Map(level, levels)

	default:
		panic("Ranger must be a ContinuousRanger or DiscreteRanger")
	}
}

func (s *linearScale) Ticks(n int) (major, minor table.Slice, labels []string) {
	type Stringer interface {
		String() string
	}

	ls := s.get()
	majorx, minorx := ls.Ticks(n)

	// Compute labels.
	//
	// TODO: Custom label formats.
	labels = make([]string, len(majorx))
	if s.domainType != nil {
		z := reflect.Zero(s.domainType).Interface()
		if _, ok := z.(Stringer); ok {
			// Convert the ticks back into the domain type
			// and use its String method.
			//
			// TODO: If the domain type is integral, don't
			// let the tick level go below 0.
			for i, x := range majorx {
				v := reflect.ValueOf(x).Convert(s.domainType)
				labels[i] = v.Interface().(Stringer).String()
			}
			goto done
		}
	}
	// Otherwise, just format them as floats.
	for i, x := range majorx {
		labels[i] = fmt.Sprintf("%g", x)
	}

done:
	return majorx, minorx, labels
}

func (s *linearScale) CloneScaler() Scaler {
	s2 := *s
	return &s2
}

func NewOrdinalScale() Scaler {
	return &ordinalScale{}
}

type ordinalScale struct {
	allData []generic.Slice
	r       Ranger
	ordered table.Slice
	index   map[interface{}]int
}

func (s *ordinalScale) ExpandDomain(v table.Slice) {
	// TODO: Type-check?
	s.allData = append(s.allData, generic.Slice(v))
	s.ordered, s.index = nil, nil
}

func (s *ordinalScale) Ranger(r Ranger) Ranger {
	old := s.r
	if r != nil {
		s.r = r
	}
	return old
}

func (s *ordinalScale) RangeType() reflect.Type {
	return s.r.RangeType()
}

func (s *ordinalScale) makeIndex() {
	if s.index != nil {
		return
	}

	// Compute ordered data index and cache.
	s.ordered = generic.NubAppend(s.allData...)
	generic.Sort(s.ordered)
	ov := reflect.ValueOf(s.ordered)
	s.index = make(map[interface{}]int, ov.Len())
	for i, len := 0, ov.Len(); i < len; i++ {
		s.index[ov.Index(i).Interface()] = i
	}
}

func (s *ordinalScale) Map(x interface{}) interface{} {
	s.makeIndex()
	i := s.index[x]
	switch r := s.r.(type) {
	case ContinuousRanger:
		// Map i to the "middle" of the ith equal j-way
		// subdivision of [0, 1].
		j := len(s.index)
		x := (float64(i) + 0.5) / float64(j)
		return r.Map(x)

	case DiscreteRanger:
		minLevels, maxLevels := r.Levels()
		if len(s.index) <= minLevels {
			return r.Map(i, minLevels)
		} else if len(s.index) <= maxLevels {
			return r.Map(i, len(s.index))
		} else {
			// TODO: Binning would also be a reasonable
			// policy.
			return r.Map(i%maxLevels, maxLevels)
		}

	default:
		panic("Ranger must be a ContinuousRanger or DiscreteRanger")
	}
}

func (s *ordinalScale) Ticks(n int) (major, minor table.Slice, labels []string) {
	// TODO: Return *no* ticks and only labels. Can't current
	// express this.

	s.makeIndex()
	labels = make([]string, len(s.index))
	ov := reflect.ValueOf(s.ordered)
	for i, len := 0, ov.Len(); i < len; i++ {
		labels[i] = fmt.Sprintf("%v", ov.Index(i).Interface())
	}
	return s.ordered, nil, labels
}

func (s *ordinalScale) CloneScaler() Scaler {
	ns := &ordinalScale{
		allData: make([]generic.Slice, len(s.allData)),
		r:       s.r,
	}
	for i, v := range s.allData {
		ns.allData[i] = v
	}
	return s
}

// XXX
//
// A Ranger must be either a ContinuousRanger or a DiscreteRanger.
type Ranger interface {
	RangeType() reflect.Type
}

type ContinuousRanger interface {
	Ranger
	Map(x float64) (y interface{})
	Unmap(y interface{}) (x float64)
}

type DiscreteRanger interface {
	Ranger
	Levels() (min, max int)
	Map(i, j int) interface{}
}

func NewFloatRanger(lo, hi float64) ContinuousRanger {
	return &floatRanger{lo, hi - lo}
}

type floatRanger struct {
	lo, w float64
}

func (r *floatRanger) String() string {
	return fmt.Sprintf("[%g,%g]", r.lo, r.lo+r.w)
}

func (r *floatRanger) RangeType() reflect.Type {
	return float64Type
}

func (r *floatRanger) Map(x float64) interface{} {
	return x*r.w + r.lo
}

func (r *floatRanger) Unmap(y interface{}) float64 {
	return (y.(float64) - r.lo) / r.w
}

func NewColorRanger(palette []color.Color) DiscreteRanger {
	// TODO: Support continuous palettes.
	//
	// TODO: Support discrete palettes that vary depending on the
	// number of levels.
	return &colorRanger{palette}
}

type colorRanger struct {
	palette []color.Color
}

func (r *colorRanger) RangeType() reflect.Type {
	return colorType
}

func (r *colorRanger) Levels() (min, max int) {
	return len(r.palette), len(r.palette)
}

func (r *colorRanger) Map(i, j int) interface{} {
	if i < 0 {
		i = 0
	} else if i >= len(r.palette) {
		i = len(r.palette) - 1
	}
	return r.palette[i]
}

// mapMany applies scaler.Map to all of the values in seq and returns
// a slice of the results.
//
// TODO: Maybe this should just be how Scaler.Map works.
func mapMany(scaler Scaler, seq table.Slice) table.Slice {
	sv := reflect.ValueOf(seq)
	rt := reflect.SliceOf(scaler.RangeType())
	res := reflect.MakeSlice(rt, sv.Len(), sv.Len())
	for i, len := 0, sv.Len(); i < len; i++ {
		val := scaler.Map(sv.Index(i).Interface())
		res.Index(i).Set(reflect.ValueOf(val))
	}
	return res.Interface()
}
