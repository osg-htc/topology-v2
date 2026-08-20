package paritytest

// diffTrees compares two generic trees (as produced by decodeXML, or by
// stdlib json.Unmarshal into interface{} for the JSON feed) and reports every
// path where a key/element exists on only one side, or exists on both with a
// differing value. See decode_test.go for the tree shape.
//
// Design notes (deliberate, not incidental):
//   - A key present with an empty value and a key absent entirely are
//     reported differently -- "present on v1 only (value=)" is not the same
//     finding as both sides simply not having the key.
//   - Same-tag repeated children are compared as an order-tolerant multiset
//     matched by full (recursively canonicalized) content, not by index --
//     neither v1 nor v2 guarantees element ordering.
//   - No third-party diff library (go-cmp et al.) or dependency is used here;
//     the repo has none today, and the semantics needed (attribute/text/child
//     separation, multiset children, missing-vs-empty) are specific enough
//     that configuring a generic library would be more code than this.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// diffTrees returns a human-readable diff line per divergence found. An empty
// result means the two trees are equivalent (given the ignore set).
func diffTrees(path string, a, b interface{}, ignore map[string]struct{}) []string {
	if _, skip := ignore[path]; skip {
		return nil
	}

	am, aIsMap := a.(map[string]interface{})
	bm, bIsMap := b.(map[string]interface{})
	if aIsMap && bIsMap {
		return diffMaps(path, am, bm, ignore)
	}
	if aIsMap != bIsMap {
		return []string{fmt.Sprintf("%s: type mismatch (v1=%T, v2=%T)", path, a, b)}
	}

	aSlice, aIsSlice := a.([]interface{})
	bSlice, bIsSlice := b.([]interface{})
	if aIsSlice && bIsSlice {
		return diffMultiset(path, aSlice, bSlice, ignore)
	}
	if aIsSlice != bIsSlice {
		return []string{fmt.Sprintf("%s: type mismatch (v1=%T, v2=%T)", path, a, b)}
	}

	if !reflect.DeepEqual(a, b) {
		return []string{fmt.Sprintf("%s: value differs (v1=%v, v2=%v)", path, a, b)}
	}
	return nil
}

func diffMaps(path string, am, bm map[string]interface{}, ignore map[string]struct{}) []string {
	keys := make(map[string]struct{}, len(am)+len(bm))
	for k := range am {
		keys[k] = struct{}{}
	}
	for k := range bm {
		keys[k] = struct{}{}
	}
	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var diffs []string
	for _, k := range sortedKeys {
		childPath := path + "." + k
		av, aok := am[k]
		bv, bok := bm[k]
		switch {
		case aok && !bok:
			diffs = append(diffs, fmt.Sprintf("%s: present on v1 only (value=%v)", childPath, av))
		case !aok && bok:
			diffs = append(diffs, fmt.Sprintf("%s: present on v2 only (value=%v)", childPath, bv))
		default:
			diffs = append(diffs, diffTrees(childPath, av, bv, ignore)...)
		}
	}
	return diffs
}

// diffMultiset compares two same-tag sibling lists as order-tolerant
// multisets, matched by full recursively-canonicalized content. When exactly
// one element is unmatched on each side (the common case: one element
// changed rather than an element being structurally added/removed), it
// reports a precise field-level diff between that pair instead of an opaque
// whole-element content blob -- a coarse "these don't match" is technically
// correct but not useful for a human deciding what actually changed.
func diffMultiset(path string, a, b []interface{}, ignore map[string]struct{}) []string {
	ac := map[string]int{}
	bc := map[string]int{}
	aByKey := map[string]interface{}{}
	bByKey := map[string]interface{}{}
	for _, v := range a {
		k := canonicalKey(v)
		ac[k]++
		aByKey[k] = v
	}
	for _, v := range b {
		k := canonicalKey(v)
		bc[k]++
		bByKey[k] = v
	}

	keys := make(map[string]struct{}, len(ac)+len(bc))
	for k := range ac {
		keys[k] = struct{}{}
	}
	for k := range bc {
		keys[k] = struct{}{}
	}
	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var onlyInA, onlyInB []string
	for _, k := range sortedKeys {
		if ac[k] != bc[k] {
			if ac[k] > bc[k] {
				onlyInA = append(onlyInA, k)
			} else {
				onlyInB = append(onlyInB, k)
			}
		}
	}

	if len(onlyInA) == 1 && len(onlyInB) == 1 {
		return diffTrees(path+"[matched]", aByKey[onlyInA[0]], bByKey[onlyInB[0]], ignore)
	}

	var diffs []string
	for _, k := range onlyInA {
		diffs = append(diffs, fmt.Sprintf("%s[]: element present in v1 only (count v1=%d, v2=%d): %s",
			path, ac[k], bc[k], truncate(k, 200)))
	}
	for _, k := range onlyInB {
		diffs = append(diffs, fmt.Sprintf("%s[]: element present in v2 only (count v1=%d, v2=%d): %s",
			path, ac[k], bc[k], truncate(k, 200)))
	}
	return diffs
}

// canonicalKey produces a stable string for an element regardless of map key
// order (encoding/json already sorts map[string]interface{} keys) or nested
// slice order (canonicalize sorts every nested slice recursively too, so two
// semantically-equal elements that merely nest their own repeated children in
// a different order still hash identically).
func canonicalKey(v interface{}) string {
	b, _ := json.Marshal(canonicalize(v))
	return string(b)
}

func canonicalize(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, vv := range t {
			out[k] = canonicalize(vv)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, vv := range t {
			out[i] = canonicalize(vv)
		}
		sort.Slice(out, func(i, j int) bool {
			bi, _ := json.Marshal(out[i])
			bj, _ := json.Marshal(out[j])
			return string(bi) < string(bj)
		})
		return out
	default:
		return v
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
