package handlers

// Real three-way merge for a proposal's own change against whatever the
// live entity independently became since the proposal's base was captured
// -- the permanent replacement for the old blunt "reject on any staleness"
// guard. Every function here is pure (map[string]interface{} in, a
// reconciled map plus any conflicts out) -- no DB access, fully testable
// without a database.
//
// base/live/proposed must all be decoded via the identical path
// (json.Unmarshal into map[string]interface{}/interface{}) before reaching
// any of these functions -- comparing a value decoded one way against one
// decoded another would produce spurious conflicts from representation
// differences that aren't real changes.

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// conflictField names one field where the proposal's own change and some
// independent change both touched it, to different values, since the
// proposal's base was captured.
type conflictField struct {
	Path                 string
	Base, Live, Proposed interface{}
}

// conflictSummary renders every conflicting field path, sorted and
// deduplicated, for the apply-rejection error message.
func conflictSummary(conflicts []conflictField) string {
	seen := map[string]bool{}
	var paths []string
	for _, c := range conflicts {
		if !seen[c.Path] {
			seen[c.Path] = true
			paths = append(paths, c.Path)
		}
	}
	sort.Strings(paths)
	return strings.Join(paths, ", ")
}

// threeWayMergeScalar reconciles one plain field's three versions:
//   - the proposal never touched it (proposed == base) -> keep live.
//   - nothing else touched it since (live == base) -> apply proposed.
//   - both landed on the same value anyway (live == proposed) -> fine.
//   - otherwise -> both changed it, to different values: a real conflict.
func threeWayMergeScalar(path string, base, live, proposed interface{}) (interface{}, []conflictField) {
	if reflect.DeepEqual(proposed, base) {
		return live, nil
	}
	if reflect.DeepEqual(live, base) {
		return proposed, nil
	}
	if reflect.DeepEqual(live, proposed) {
		return proposed, nil
	}
	return live, []conflictField{{Path: path, Base: base, Live: live, Proposed: proposed}}
}

// threeWayMergeFlat reconciles every key of a flat object (no further
// nesting handled here) via threeWayMergeScalar. path is the prefix for
// this object's own field paths ("" at the true top level).
func threeWayMergeFlat(path string, base, live, proposed map[string]interface{}) (map[string]interface{}, []conflictField) {
	reconciled := map[string]interface{}{}
	var conflicts []conflictField
	for _, key := range unionKeys(base, live, proposed) {
		fieldPath := key
		if path != "" {
			fieldPath = path + "." + key
		}
		v, c := threeWayMergeScalar(fieldPath, base[key], live[key], proposed[key])
		reconciled[key] = v
		conflicts = append(conflicts, c...)
	}
	return reconciled, conflicts
}

// mergeKeyedCollection reconciles a keyed collection (Go representation:
// map[string]interface{}, one entry per key) between base/live/proposed.
// Every combination of presence across the three sides is handled
// explicitly (9 cases; the 8th -- both sides independently deleted the same
// entry -- resolves to "gone, no conflict", easy to miss if handled
// implicitly). When an entry is present on all three sides, recurseFields
// decides whether that entry is atomic (a Tags-style set member -- nothing
// to look inside) or has its own sub-fields to reconcile one level deeper
// (a Service's Description/Details/Extra) via threeWayMergeFlat. That
// nested reconciliation deliberately goes no further than one level --
// Details/Extra are compared as whole values, not recursed into their own
// arbitrary shape.
func mergeKeyedCollection(path string, base, live, proposed map[string]interface{}, recurseFields bool) (map[string]interface{}, []conflictField) {
	reconciled := map[string]interface{}{}
	var conflicts []conflictField
	for _, key := range unionKeys(base, live, proposed) {
		b, bOK := base[key]
		l, lOK := live[key]
		p, pOK := proposed[key]
		entryPath := path + "." + key

		switch {
		case !bOK && pOK && !lOK:
			// We added it; nobody else has an opinion.
			reconciled[key] = p
		case !bOK && !pOK && lOK:
			// Someone else added it; we never mentioned this key at all.
			reconciled[key] = l
		case !bOK && pOK && lOK:
			// Both independently added the same key.
			if reflect.DeepEqual(l, p) {
				reconciled[key] = p
			} else {
				conflicts = append(conflicts, conflictField{Path: entryPath, Live: l, Proposed: p})
				reconciled[key] = l
			}
		case bOK && !pOK && lOK:
			// We deleted it. Did anyone else touch it since base?
			if reflect.DeepEqual(l, b) {
				// No -- deletion applies cleanly (omit from reconciled).
			} else {
				conflicts = append(conflicts, conflictField{Path: entryPath, Base: b, Live: l})
				reconciled[key] = l
			}
		case bOK && pOK && !lOK:
			// Someone else deleted it. Did we actually change it?
			if reflect.DeepEqual(p, b) {
				// No -- we never touched it, their deletion wins silently.
			} else {
				conflicts = append(conflicts, conflictField{Path: entryPath, Base: b, Proposed: p})
				// Omit -- matches "live" (absent) as the conflict default.
			}
		case bOK && !pOK && !lOK:
			// Both independently deleted it: agreement, it's just gone.
		case bOK && pOK && lOK:
			if recurseFields {
				bSub, _ := b.(map[string]interface{})
				lSub, _ := l.(map[string]interface{})
				pSub, _ := p.(map[string]interface{})
				sub, subConflicts := threeWayMergeFlat(entryPath, bSub, lSub, pSub)
				reconciled[key] = sub
				conflicts = append(conflicts, subConflicts...)
			} else {
				v, c := threeWayMergeScalar(entryPath, b, l, p)
				reconciled[key] = v
				conflicts = append(conflicts, c...)
			}
		}
	}
	return reconciled, conflicts
}

// threeWayMergeSet reconciles an unordered set of strings (Tags/AllowedVOs/
// FQDNAliases) between base/live/proposed. Treating these as sets rather
// than atomic arrays is what lets two independent edits to the same field
// -- e.g. two people each tagging the same resource differently -- compose
// automatically instead of manufacturing a false conflict on every
// concurrent edit to the field, which whole-array equality would. Presence-
// only entries can never actually conflict under mergeKeyedCollection's
// rules (a set member's "content" is its existence, which is always self-
// consistent), which is exactly the desired behavior for a set. Always
// returns a non-nil slice so it marshals to [] rather than null.
func threeWayMergeSet(path string, base, live, proposed []string) ([]string, []conflictField) {
	toSet := func(s []string) map[string]interface{} {
		m := make(map[string]interface{}, len(s))
		for _, v := range s {
			m[v] = true
		}
		return m
	}
	merged, conflicts := mergeKeyedCollection(path, toSet(base), toSet(live), toSet(proposed), false)
	out := make([]string, 0, len(merged))
	for k := range merged {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, conflicts
}

// threeWayMergeResource reconciles a full resourceProposal-shaped document
// (envelope {resource_group, name, resource: {...}}), matching the exact
// depth mergeProposedState already merges at: the envelope itself, the
// resource's own scalar fields, Tags/AllowedVOs/FQDNAliases as sets, and
// Services as a keyed collection recursing one level into each service's
// own fields. ContactLists stays atomic (matches the existing 2-way
// precedent in mergeShallow) -- a deliberate scope decision, not an
// oversight.
func threeWayMergeResource(base, live, proposed map[string]interface{}) (map[string]interface{}, []conflictField) {
	reconciled, conflicts := threeWayMergeFlat("", withoutKeys(base, "resource"), withoutKeys(live, "resource"), withoutKeys(proposed, "resource"))

	baseRes := asMap(base["resource"])
	liveRes := asMap(live["resource"])
	proposedRes := asMap(proposed["resource"])

	setFields := []string{"Tags", "AllowedVOs", "FQDNAliases"}
	scalarBase := withoutKeys(baseRes, append(setFields, "Services")...)
	scalarLive := withoutKeys(liveRes, append(setFields, "Services")...)
	scalarProposed := withoutKeys(proposedRes, append(setFields, "Services")...)
	reconciledRes, resConflicts := threeWayMergeFlat("resource", scalarBase, scalarLive, scalarProposed)
	conflicts = append(conflicts, resConflicts...)

	for _, field := range setFields {
		merged, c := threeWayMergeSet("resource."+field, asStringSlice(baseRes[field]), asStringSlice(liveRes[field]), asStringSlice(proposedRes[field]))
		reconciledRes[field] = merged
		conflicts = append(conflicts, c...)
	}

	svcMerged, svcConflicts := mergeKeyedCollection("resource.Services", asMap(baseRes["Services"]), asMap(liveRes["Services"]), asMap(proposedRes["Services"]), true)
	reconciledRes["Services"] = svcMerged
	conflicts = append(conflicts, svcConflicts...)

	reconciled["resource"] = reconciledRes
	return reconciled, conflicts
}

// --- small map/slice helpers, generic-JSON-shaped ---

func unionKeys(maps ...map[string]interface{}) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range maps {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}

func withoutKeys(m map[string]interface{}, keys ...string) map[string]interface{} {
	if m == nil {
		return nil
	}
	skip := make(map[string]bool, len(keys))
	for _, k := range keys {
		skip[k] = true
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if !skip[k] {
			out[k] = v
		}
	}
	return out
}

func asMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func asStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// errConflict is returned (wrapped with the field list) when a genuine
// field-level conflict is found; the caller surfaces it through the exact
// same error path every other apply failure already uses.
func errConflict(conflicts []conflictField) error {
	return fmt.Errorf(
		"conflicting changes to: %s -- this entity changed independently since this proposal was created; revise this proposal to incorporate the current state",
		conflictSummary(conflicts),
	)
}
