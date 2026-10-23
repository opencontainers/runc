// check-config checks that config.json only has the keys known to
// runtime-spec, spelled exactly as in the spec.
//
// This is needed because Go's encoding/json (v1) matches keys
// case-insensitively, and silently ignores unknown keys, so a wrong key
// in a test goes unnoticed by runc (but not by other runtimes).
package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s config.json\n", os.Args[0])
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
	errs := check(v, reflect.TypeFor[specs.Spec](), "")
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[1], e)
	}
	if len(errs) > 0 {
		os.Exit(1)
	}
}

func check(v any, t reflect.Type, path string) (errs []string) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		fields := jsonFields(t)
		for _, k := range slices.Sorted(maps.Keys(m)) {
			ft, ok := fields[k]
			if !ok {
				errs = append(errs, unknownKey(path, k, fields))
				continue
			}
			errs = append(errs, check(m[k], ft, path+"/"+k)...)
		}
	case reflect.Slice, reflect.Array:
		s, _ := v.([]any)
		for i, e := range s {
			errs = append(errs, check(e, t.Elem(), path+"/"+strconv.Itoa(i))...)
		}
	case reflect.Map:
		m, _ := v.(map[string]any)
		for _, k := range slices.Sorted(maps.Keys(m)) {
			errs = append(errs, check(m[k], t.Elem(), path+"/"+k)...)
		}
	}
	return errs
}

// jsonFields returns a map of JSON keys to field types for a struct type.
func jsonFields(t reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type)
	for f := range t.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		// Fields of an embedded struct with no tag are promoted.
		if f.Anonymous && name == "" && f.Type.Kind() == reflect.Struct {
			maps.Copy(fields, jsonFields(f.Type))
			continue
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		fields[name] = f.Type
	}
	return fields
}

func unknownKey(path, key string, fields map[string]reflect.Type) string {
	for name := range fields {
		if strings.EqualFold(name, key) {
			return fmt.Sprintf("%s/%s: wrong case, should be %q", path, key, name)
		}
	}
	return fmt.Sprintf("%s/%s: unknown key", path, key)
}
