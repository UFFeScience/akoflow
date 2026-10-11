// Package montagetools contains scientific experiment tooling, not runtime
// execution. JSON contracts and allowlists are kept separate from raw API data.
package montagetools

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Object = map[string]any

func obj(v any) Object {
	if v == nil {
		return Object{}
	}
	m, ok := v.(map[string]any)
	check(ok, "object required")
	return m
}
func arr(v any) []any {
	if v == nil {
		return []any{}
	}
	a, ok := v.([]any)
	check(ok, "array required")
	return a
}
func get(m Object, k string) any { v, ok := m[k]; check(ok, "missing field "+k); return v }
func str(v any) string           { s, ok := v.(string); check(ok, "string required"); return s }
func num(v any) float64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, e := x.Float64()
		check(e == nil, "invalid number")
		return f
	case string:
		f, e := strconv.ParseFloat(x, 64)
		check(e == nil, "invalid number")
		return f
	}
	panic("number required")
}
func check(ok bool, message string) {
	if !ok {
		panic(message)
	}
}
func eq(a, b any) bool { return reflect.DeepEqual(normal(a), normal(b)) }
func normal(v any) any {
	p, e := json.Marshal(v)
	check(e == nil, "invalid JSON")
	var out any
	check(json.Unmarshal(p, &out) == nil, "invalid JSON")
	return out
}
func clone(m Object) Object { return obj(normal(m)) }
func sortedKeys(m Object) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func list(values ...string) []any {
	a := []any{}
	for _, v := range values {
		a = append(a, v)
	}
	return a
}
func pick(m Object, keys ...string) Object {
	o := Object{}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			o[k] = v
		}
	}
	return o
}
func rows(v any) []Object {
	out := []Object{}
	for _, r := range arr(v) {
		out = append(out, obj(r))
	}
	return out
}
func sortRows(rs []Object, key string) {
	sort.SliceStable(rs, func(i, j int) bool { return str(rs[i][key]) < str(rs[j][key]) })
}
func index(v any, key string) Object {
	out := Object{}
	for _, r := range rows(v) {
		out[str(get(r, key))] = r
	}
	return out
}

type group struct {
	key  []string
	rows []Object
}

func groups(v []Object, keys ...string) []group {
	out := []group{}
	positions := map[string]int{}
	for _, r := range v {
		parts := []string{}
		for _, k := range keys {
			parts = append(parts, str(get(r, k)))
		}
		key := strings.Join(parts, "\x00")
		i, ok := positions[key]
		if !ok {
			i = len(out)
			positions[key] = i
			out = append(out, group{key: parts})
		}
		out[i].rows = append(out[i].rows, r)
	}
	return out
}
func vals(rs []Object, key string) []any {
	out := []any{}
	for _, r := range rs {
		out = append(out, r[key])
	}
	return out
}
func sum(rs []Object, key string) float64 {
	v := 0.0
	for _, r := range rs {
		v += num(r[key])
	}
	return v
}
func digest(p []byte) string { h := sha256.Sum256(p); return hex.EncodeToString(h[:]) }
func encode(v any) []byte {
	p, e := json.MarshalIndent(v, "", "  ")
	check(e == nil, "encode JSON: "+fmt.Sprint(e))
	return append(p, '\n')
}
func read(path string) (Object, []byte) {
	p, e := os.ReadFile(path)
	check(e == nil, "read input: "+fmt.Sprint(e))
	d := json.NewDecoder(bytes.NewReader(p))
	d.UseNumber()
	var m Object
	check(d.Decode(&m) == nil, "invalid input JSON")
	return m, p
}
func write(path string, v any) {
	check(os.WriteFile(path, encode(v), 0600) == nil, "write output failed")
}
func resourceClass(id string) string {
	switch {
	case strings.Contains(id, "fog-e2-micro"):
		return "e2-micro"
	case strings.Contains(id, "e2-medium"):
		return "e2-medium"
	case strings.Contains(id, "bora") || strings.Contains(id, "diablo"):
		return "hpc"
	}
	panic("Unclassified resource " + id)
}

var familyPattern = regexp.MustCompile(`-([a-z]+)id\d+$`)

func family(id string) string {
	m := familyPattern.FindStringSubmatch(id)
	check(len(m) == 2, "Unclassified activity "+id)
	return m[1]
}
func summary(values []any) any {
	vs := []float64{}
	for _, v := range values {
		if v == nil {
			continue
		}
		f := num(v)
		check(!math.IsNaN(f) && !math.IsInf(f, 0) && f >= 0, "Invalid non-finite or negative measurement")
		vs = append(vs, f)
	}
	if len(vs) == 0 {
		return nil
	}
	sort.Float64s(vs)
	n := len(vs)
	median := vs[n/2]
	if n%2 == 0 {
		median = (vs[n/2-1] + median) / 2
	}
	return Object{"samples": n, "median": median, "minimum": vs[0], "maximum": vs[n-1]}
}
func pooledRate(rs []Object, key string) any {
	bytes, seconds := 0.0, 0.0
	n := 0
	for _, r := range rs {
		if num(r[key]) > 0 && num(r["seconds"]) > 0 {
			bytes += num(r[key])
			seconds += num(r["seconds"])
			n++
		}
	}
	if n == 0 {
		return nil
	}
	return Object{"samples": n, "bytes": bytes, "serviceSeconds": seconds, "bytesPerSecond": bytes / seconds, "interpretation": "byte-weighted per-command service coefficient; summed service is not concurrent wall time"}
}
func attempt(fn func() Object) (result Object, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%v", p)
		}
	}()
	return fn(), nil
}
