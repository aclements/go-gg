// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package generic

import (
	"reflect"
	"sort"
)

// Sort sorts v in increasing order. v must implement sort.Interface
// or must be a slice, array, or pointer to array whose element type
// is orderable.
func Sort(v interface{}) {
	sort.Sort(Sorter(v))
}

// Sorter returns a sort.Interface for sorting v. v must implement
// sort.Interface or must be a slice, array, or pointer to array whose
// element type is orderable.
func Sorter(v interface{}) sort.Interface {
	switch v := v.(type) {
	case []int:
		return sort.IntSlice(v)
	case []float64:
		return sort.Float64Slice(v)
	case []string:
		return sort.StringSlice(v)
	case sort.Interface:
		return v
	}

	rv := sequence(v)
	switch rv.Type().Elem().Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return sortIntSlice{rv}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return sortUintSlice{rv}
	case reflect.Float32, reflect.Float64:
		return sortFloatSlice{rv}
	case reflect.String:
		return sortStringSlice{rv}
	}
	panic(&TypeError{rv.Type().Elem(), nil, "is not orderable"})
}

type sortIntSlice struct {
	reflect.Value
}

func (s sortIntSlice) Len() int {
	return s.Len()
}

func (s sortIntSlice) Less(i, j int) bool {
	return s.Index(i).Int() < s.Index(j).Int()
}

func (s sortIntSlice) Swap(i, j int) {
	t := s.Index(j)
	s.Index(j).Set(s.Index(i))
	s.Index(i).Set(t)
}

type sortUintSlice struct {
	reflect.Value
}

func (s sortUintSlice) Len() int {
	return s.Len()
}

func (s sortUintSlice) Less(i, j int) bool {
	return s.Index(i).Uint() < s.Index(j).Uint()
}

func (s sortUintSlice) Swap(i, j int) {
	t := s.Index(j)
	s.Index(j).Set(s.Index(i))
	s.Index(i).Set(t)
}

type sortFloatSlice struct {
	reflect.Value
}

func (s sortFloatSlice) Len() int {
	return s.Len()
}

func (s sortFloatSlice) Less(i, j int) bool {
	return s.Index(i).Float() < s.Index(j).Float()
}

func (s sortFloatSlice) Swap(i, j int) {
	t := s.Index(j)
	s.Index(j).Set(s.Index(i))
	s.Index(i).Set(t)
}

type sortStringSlice struct {
	reflect.Value
}

func (s sortStringSlice) Len() int {
	return s.Len()
}

func (s sortStringSlice) Less(i, j int) bool {
	return s.Index(i).String() < s.Index(j).String()
}

func (s sortStringSlice) Swap(i, j int) {
	t := s.Index(j)
	s.Index(j).Set(s.Index(i))
	s.Index(i).Set(t)
}
