package githistory

// PathIdentity tracks, across an entire commit-ordered walk, which single
// underlying entity a given file path refers to through any number of
// renames. A name-keyed entity (facility/site/resource_group/project) has no
// immutable id in this system -- ListProposalsByEntity finds a proposal by
// exact current name -- so a renamed entity's pre-rename history would be
// invisible from its post-rename history page unless every historical row
// for it is normalized to use its one final name. That normalization is
// this type's whole job: assign each entity a stable opaque id the moment
// it's first seen, thread that id through every rename, and record the
// last name it ever had (its name at HEAD if it still exists there, or its
// name just before deletion otherwise).
//
// Usage is two-pass by construction: walk the full history once calling
// Touch for every changed path (this populates the final-name-per-id map
// completely), then walk the buffered events a second time calling
// FinalName(id) -- by then every id's last name is already known, including
// ones assigned by a later commit than the one currently being resolved.
type PathIdentity struct {
	nextID    int
	current   map[string]int // path -> identity id, as of "now" in the walk
	finalName map[int]string // identity id -> most recent name assigned to it
}

// NewPathIdentity returns an empty registry.
func NewPathIdentity() *PathIdentity {
	return &PathIdentity{current: map[string]int{}, finalName: map[int]string{}}
}

// Touch records one changed path's effect on the registry and returns the
// stable identity id that path refers to at this point in the walk. name is
// the entity's name at this path right now (its new name for an add/modify/
// rename, or its old name for a delete).
func (r *PathIdentity) Touch(status byte, oldPath, newPath, name string) int {
	switch status {
	case 'A':
		id := r.newID()
		r.current[newPath] = id
		r.finalName[id] = name
		return id
	case 'R', 'C':
		id, ok := r.current[oldPath]
		if !ok {
			// Renamed from a path this walk never saw an Add for (e.g. the
			// walk started mid-history) -- treat it as a fresh identity.
			id = r.newID()
		} else {
			delete(r.current, oldPath)
		}
		r.current[newPath] = id
		r.finalName[id] = name
		return id
	case 'M':
		id, ok := r.current[newPath]
		if !ok {
			id = r.newID()
			r.current[newPath] = id
		}
		r.finalName[id] = name
		return id
	case 'D':
		id, ok := r.current[oldPath]
		if !ok {
			id = r.newID()
		} else {
			delete(r.current, oldPath)
		}
		return id
	default:
		return -1
	}
}

func (r *PathIdentity) newID() int {
	id := r.nextID
	r.nextID++
	return id
}

// FinalName returns the last name recorded for id -- its name at HEAD if it
// still exists there, or its name just before deletion otherwise. Only
// meaningful after a full first pass over the whole history has completed.
func (r *PathIdentity) FinalName(id int) string {
	return r.finalName[id]
}
