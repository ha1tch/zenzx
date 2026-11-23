// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

// Package machinestate saves the state of a machine made of Go structs and puts it back.
//
// The snapshot formats of the 48K and 128K models list their fields by hand, and a
// machine as large as the Next would leave a field out the first time one was added. This
// package walks the structs themselves, unexported fields included, so that a field is
// saved because it exists. What is not state is said once, in a Policy: callbacks wired
// at construction, files, the window, the audio device. Everything else is kept.
//
// A save walks the roots in order and writes what it finds. A restore walks the same
// roots of an already constructed machine and writes the saved values over them, so
// that everything wired at construction (callbacks, pointers between the parts) stays
// as it is. Pointers are followed and an object met twice is written once. A pointer
// that is nil in the machine and not in the saved state is given a new object.
//
// What was saved is only good for the structs it was saved from: Fingerprint is a hash
// of their layout, written in the header, and a restore against another layout is
// refused rather than misread.
package machinestate

import (
	"bufio"
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"unsafe"
)

const magic = "ZXMSTATE1\n"

// endMark closes the stream, so that one that was cut, or has more than it should, is known.
const endMark = 0x5A17C0DE

// Policy says what is not state.
type Policy struct {
	// SkipFields names struct fields that are neither saved nor restored, as
	// "pkg.Type.field" (the type as reflect prints it).
	SkipFields map[string]bool
	// SkipTypes are types whose values are neither saved nor restored wherever they
	// occur, as a field or as what a pointer field points to.
	SkipTypes []reflect.Type

	skipTypes map[reflect.Type]bool
}

func (p *Policy) prepare() {
	if p.skipTypes != nil {
		return
	}
	p.skipTypes = map[reflect.Type]bool{}
	for _, t := range p.SkipTypes {
		p.skipTypes[t] = true
	}
}

// skipped reports whether field f of struct type st is left out, and why.
func (p *Policy) skipped(st reflect.Type, f reflect.StructField) (bool, string) {
	p.prepare()
	switch f.Type.Kind() {
	case reflect.Func, reflect.Chan, reflect.Interface, reflect.UnsafePointer:
		return true, "kind " + f.Type.Kind().String()
	}
	if p.SkipFields[st.String()+"."+f.Name] {
		return true, "policy field"
	}
	// A skipped type is skipped as a field, behind a pointer, and in arrays and slices of
	// either, so that [2]*image.RGBA goes as image.RGBA does.
	t := f.Type
	for {
		if p.skipTypes[t] {
			return true, "policy type"
		}
		switch t.Kind() {
		case reflect.Pointer, reflect.Array, reflect.Slice:
			t = t.Elem()
			continue
		}
		return false, ""
	}
}

// Skip is one field that is left out and the reason.
type Skip struct {
	Field  string // "pkg.Type.field"
	Type   string
	Reason string
}

// Skipped lists every field reachable from the roots that is left out, in a fixed order,
// so that a test can pin the list and a new callback or file in the machine is noticed.
func (p *Policy) Skipped(roots ...any) []Skip {
	var out []Skip
	seen := map[reflect.Type]bool{}
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		if seen[t] {
			return
		}
		seen[t] = true
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			walk(t.Elem())
		case reflect.Map:
			walk(t.Key())
			walk(t.Elem())
		case reflect.Struct:
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				if skip, why := p.skipped(t, f); skip {
					out = append(out, Skip{Field: t.String() + "." + f.Name, Type: f.Type.String(), Reason: why})
					continue
				}
				walk(f.Type)
			}
		}
	}
	for _, r := range roots {
		walk(reflect.TypeOf(r))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out
}

// Fingerprint is a hash of the layout of the roots' types, skipped fields left out.
func (p *Policy) Fingerprint(roots ...any) [32]byte {
	h := sha256.New()
	seen := map[reflect.Type]bool{}
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		fmt.Fprintf(h, "%s|%s|", t.String(), t.Kind())
		switch t.Kind() {
		case reflect.Array:
			fmt.Fprintf(h, "%d|", t.Len())
			walk(t.Elem())
		case reflect.Pointer, reflect.Slice:
			walk(t.Elem())
		case reflect.Map:
			walk(t.Key())
			walk(t.Elem())
		case reflect.Struct:
			if seen[t] {
				return
			}
			seen[t] = true
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				if skip, _ := p.skipped(t, f); skip {
					continue
				}
				fmt.Fprintf(h, "%s:", f.Name)
				walk(f.Type)
			}
		}
	}
	for _, r := range roots {
		walk(reflect.TypeOf(r))
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

// open makes v writable and readable whether or not its field is exported.
func open(v reflect.Value) reflect.Value {
	if v.CanSet() {
		return v
	}
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
}

type ptrKey struct {
	addr uintptr
	t    reflect.Type
}

type encoder struct {
	p    *Policy
	w    *bufio.Writer
	ids  map[ptrKey]int
	next int
	buf  [binary.MaxVarintLen64]byte
}

func (e *encoder) uvarint(x uint64) {
	n := binary.PutUvarint(e.buf[:], x)
	e.w.Write(e.buf[:n])
}

func (e *encoder) varint(x int64) {
	n := binary.PutVarint(e.buf[:], x)
	e.w.Write(e.buf[:n])
}

// Encode writes the state reachable from the roots, each a non-nil pointer.
func (p *Policy) Encode(w io.Writer, roots ...any) (err error) {
	p.prepare()
	for _, r := range roots {
		if v := reflect.ValueOf(r); v.Kind() != reflect.Pointer || v.IsNil() {
			return fmt.Errorf("machinestate: a root must be a non-nil pointer, not %T", r)
		}
	}
	fp := p.Fingerprint(roots...)
	if _, err := io.WriteString(w, magic); err != nil {
		return err
	}
	if _, err := w.Write(fp[:]); err != nil {
		return err
	}
	zw, err := flate.NewWriter(w, flate.BestSpeed)
	if err != nil {
		return err
	}
	e := &encoder{p: p, w: bufio.NewWriterSize(zw, 1<<16), ids: map[ptrKey]int{}}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("machinestate: %v", r)
		}
	}()
	for _, r := range roots {
		e.value(reflect.ValueOf(r), "")
	}
	e.uvarint(endMark)
	if err := e.w.Flush(); err != nil {
		return err
	}
	return zw.Close()
}

func (e *encoder) value(v reflect.Value, path string) {
	switch v.Kind() {
	case reflect.Bool:
		if v.Bool() {
			e.w.WriteByte(1)
		} else {
			e.w.WriteByte(0)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		e.varint(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		e.uvarint(v.Uint())
	case reflect.Float32, reflect.Float64:
		e.uvarint(math.Float64bits(v.Float()))
	case reflect.String:
		s := v.String()
		e.uvarint(uint64(len(s)))
		e.w.WriteString(s)
	case reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			e.w.Write(bytesOf(v))
			return
		}
		for i := 0; i < v.Len(); i++ {
			e.value(open(v.Index(i)), path)
		}
	case reflect.Slice:
		if v.IsNil() {
			e.uvarint(0)
			return
		}
		e.uvarint(uint64(v.Len()) + 1)
		if v.Type().Elem().Kind() == reflect.Uint8 {
			e.w.Write(bytesOf(v))
			return
		}
		for i := 0; i < v.Len(); i++ {
			e.value(open(v.Index(i)), path)
		}
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if skip, _ := e.p.skipped(t, f); skip {
				continue
			}
			e.value(open(v.Field(i)), path+"."+f.Name)
		}
	case reflect.Pointer:
		if v.IsNil() {
			e.uvarint(0)
			return
		}
		key := ptrKey{v.Pointer(), v.Type()}
		if id, ok := e.ids[key]; ok {
			e.uvarint(uint64(id) + 2) // met before: 2 + its number
			return
		}
		e.ids[key] = e.next
		e.next++
		e.uvarint(1) // new
		e.value(open(v.Elem()), path)
	case reflect.Map:
		if v.IsNil() {
			e.uvarint(0)
			return
		}
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keyLess(keys[i], keys[j]) })
		e.uvarint(uint64(len(keys)) + 1)
		for _, k := range keys {
			e.value(k, path)
			e.value(v.MapIndex(k), path)
		}
	default:
		panic(fmt.Sprintf("cannot save %s at %s", v.Kind(), path))
	}
}

func keyLess(a, b reflect.Value) bool {
	switch a.Kind() {
	case reflect.String:
		return a.String() < b.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() < b.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return a.Uint() < b.Uint()
	}
	panic(fmt.Sprintf("map keys of kind %s cannot be ordered", a.Kind()))
}

// bytesOf is the memory of an addressable array or a slice of bytes, without a copy.
func bytesOf(v reflect.Value) []byte {
	if v.Kind() == reflect.Slice {
		if v.Len() == 0 {
			return nil
		}
		return unsafe.Slice((*byte)(v.UnsafePointer()), v.Len())
	}
	if v.Len() == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(v.UnsafeAddr())), v.Len())
}

// ErrLayout is returned by Decode when the state was saved from other structs.
var ErrLayout = errors.New("machinestate: the saved state is of another layout")

// ErrFormat is returned by Decode for something that is not saved state.
var ErrFormat = errors.New("machinestate: not saved state")

type decoder struct {
	p    *Policy
	r    *bufio.Reader
	objs []reflect.Value // pointers, by number
}

// Decode puts saved state over the roots, which must be the same kinds of pointer as
// were saved, in the same order.
func (p *Policy) Decode(r io.Reader, roots ...any) (err error) {
	p.prepare()
	head := make([]byte, len(magic)+32)
	if _, err := io.ReadFull(r, head); err != nil {
		return ErrFormat
	}
	if string(head[:len(magic)]) != magic {
		return ErrFormat
	}
	fp := p.Fingerprint(roots...)
	if !bytes.Equal(head[len(magic):], fp[:]) {
		return ErrLayout
	}
	d := &decoder{p: p, r: bufio.NewReaderSize(flate.NewReader(r), 1<<16)}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("machinestate: %v", r)
		}
	}()
	for _, root := range roots {
		d.root(reflect.ValueOf(root))
	}
	if d.uvarint() != endMark {
		return ErrFormat
	}
	// The stream must end here, and cleanly: a cut one reports it only when read to its end.
	if _, err := d.r.ReadByte(); err != io.EOF {
		return fmt.Errorf("machinestate: the saved state does not end where it should: %v", err)
	}
	return nil
}

// root reads a root, a pointer that is not itself a field and so is never replaced. A root
// met already, as the target of a pointer in an earlier root, must be the same object.
func (d *decoder) root(p reflect.Value) {
	tag := d.uvarint()
	switch {
	case tag == 1:
		d.objs = append(d.objs, p)
		d.value(open(p.Elem()), "")
	case tag >= 2 && int(tag-2) < len(d.objs):
		if d.objs[tag-2].Pointer() != p.Pointer() {
			panic("the saved state has this root as another object than the machine's")
		}
	default:
		panic("the saved state does not begin with the roots given")
	}
}

func (d *decoder) uvarint() uint64 {
	x, err := binary.ReadUvarint(d.r)
	if err != nil {
		panic(fmt.Errorf("the saved state ends early: %w", err))
	}
	return x
}

func (d *decoder) varint() int64 {
	x, err := binary.ReadVarint(d.r)
	if err != nil {
		panic(fmt.Errorf("the saved state ends early: %w", err))
	}
	return x
}

func (d *decoder) fill(b []byte) {
	if _, err := io.ReadFull(d.r, b); err != nil {
		panic(fmt.Errorf("the saved state ends early: %w", err))
	}
}

// value reads one value over v, which is addressable.
func (d *decoder) value(v reflect.Value, path string) {
	switch v.Kind() {
	case reflect.Bool:
		b, err := d.r.ReadByte()
		if err != nil {
			panic(err)
		}
		v.SetBool(b != 0)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(d.varint())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v.SetUint(d.uvarint())
	case reflect.Float32, reflect.Float64:
		v.SetFloat(math.Float64frombits(d.uvarint()))
	case reflect.String:
		n := d.uvarint()
		b := make([]byte, n)
		d.fill(b)
		v.SetString(string(b))
	case reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			d.fill(bytesOf(v))
			return
		}
		for i := 0; i < v.Len(); i++ {
			d.value(open(v.Index(i)), path)
		}
	case reflect.Slice:
		n := d.uvarint()
		if n == 0 {
			v.Set(reflect.Zero(v.Type()))
			return
		}
		n--
		if v.IsNil() || uint64(v.Len()) != n {
			v.Set(reflect.MakeSlice(v.Type(), int(n), int(n)))
		}
		if v.Type().Elem().Kind() == reflect.Uint8 {
			d.fill(bytesOf(v))
			return
		}
		for i := 0; i < v.Len(); i++ {
			d.value(open(v.Index(i)), path)
		}
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if skip, _ := d.p.skipped(t, f); skip {
				continue
			}
			d.value(open(v.Field(i)), path+"."+f.Name)
		}
	case reflect.Pointer:
		tag := d.uvarint()
		switch tag {
		case 0:
			v.Set(reflect.Zero(v.Type()))
		case 1:
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			cp := reflect.New(v.Type()).Elem()
			cp.Set(v)
			d.objs = append(d.objs, cp)
			d.value(open(v.Elem()), path)
		default:
			id := int(tag - 2)
			if id >= len(d.objs) {
				panic(fmt.Sprintf("a pointer refers to object %d of %d at %s", id, len(d.objs), path))
			}
			v.Set(d.objs[id])
		}
	case reflect.Map:
		n := d.uvarint()
		if n == 0 {
			v.Set(reflect.Zero(v.Type()))
			return
		}
		n--
		m := reflect.MakeMapWithSize(v.Type(), int(n))
		for i := uint64(0); i < n; i++ {
			k := reflect.New(v.Type().Key()).Elem()
			d.value(k, path)
			e := reflect.New(v.Type().Elem()).Elem()
			d.value(e, path)
			m.SetMapIndex(k, e)
		}
		v.Set(m)
	default:
		panic(fmt.Sprintf("cannot restore %s at %s", v.Kind(), path))
	}
}

// Dump writes the state reachable from the roots as one line per value, "path = value",
// byte arrays and slices as their length and a hash. Two machines that dump alike hold the
// same state, and where they differ the first line that does is the field to look at.
func (p *Policy) Dump(w io.Writer, roots ...any) error {
	p.prepare()
	d := &dumper{p: p, w: bufio.NewWriter(w), ids: map[ptrKey]int{}}
	for i, r := range roots {
		d.value(reflect.ValueOf(r), fmt.Sprintf("root%d", i))
	}
	return d.w.Flush()
}

type dumper struct {
	p    *Policy
	w    *bufio.Writer
	ids  map[ptrKey]int
	next int
}

func (d *dumper) line(path, format string, a ...any) {
	fmt.Fprintf(d.w, "%s = ", path)
	fmt.Fprintf(d.w, format, a...)
	d.w.WriteByte('\n')
}

func (d *dumper) value(v reflect.Value, path string) {
	switch v.Kind() {
	case reflect.Bool:
		d.line(path, "%v", v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		d.line(path, "%d", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		d.line(path, "%d", v.Uint())
	case reflect.Float32, reflect.Float64:
		d.line(path, "%v", v.Float())
	case reflect.String:
		d.line(path, "%q", v.String())
	case reflect.Array, reflect.Slice:
		if v.Kind() == reflect.Slice && v.IsNil() {
			d.line(path, "nil")
			return
		}
		if v.Type().Elem().Kind() == reflect.Uint8 {
			sum := sha256.Sum256(bytesOf(open(v)))
			d.line(path, "%d bytes %x", v.Len(), sum[:6])
			return
		}
		for i := 0; i < v.Len(); i++ {
			d.value(open(v.Index(i)), fmt.Sprintf("%s[%d]", path, i))
		}
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if skip, _ := d.p.skipped(t, f); skip {
				continue
			}
			d.value(open(v.Field(i)), path+"."+f.Name)
		}
	case reflect.Pointer:
		if v.IsNil() {
			d.line(path, "nil")
			return
		}
		key := ptrKey{v.Pointer(), v.Type()}
		if id, ok := d.ids[key]; ok {
			d.line(path, "-> #%d", id)
			return
		}
		d.ids[key] = d.next
		d.line(path, "#%d", d.next)
		d.next++
		d.value(open(v.Elem()), path)
	case reflect.Map:
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keyLess(keys[i], keys[j]) })
		for _, k := range keys {
			d.value(v.MapIndex(k), fmt.Sprintf("%s[%v]", path, k))
		}
	}
}
