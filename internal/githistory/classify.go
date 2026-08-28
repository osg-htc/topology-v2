package githistory

import (
	"strings"

	"github.com/bbockelm/topology-v2/internal/models"
)

// classified is one entity-file path's parsed identity: what kind of entity
// it is, its name (from the path, matching v1's "identity is the path"
// convention), and enough of its parent path to build the same JSON shape
// the live proposal handlers already use (a site names its facility, a
// resource group names its site).
type classified struct {
	Kind     string
	Name     string
	Site     string // resource_group, site: the site name
	Facility string // site only: the facility name
}

// classifyPath maps one changed path (already known to be under topology/
// or projects/) to the entity it represents, or ok=false for a path that
// isn't one of the four entity-file shapes this importer understands --
// topology/services.yaml, topology/support-centers.yaml, a *_downtime.yaml
// file (downtime history is out of scope for this pass), or anything else
// unexpected.
func classifyPath(path string) (c classified, ok bool) {
	parts := strings.Split(path, "/")
	switch {
	case len(parts) == 3 && parts[0] == "topology" && parts[2] == "FACILITY.yaml":
		return classified{Kind: models.KindFacility, Name: parts[1]}, true
	case len(parts) == 4 && parts[0] == "topology" && parts[3] == "SITE.yaml":
		return classified{Kind: models.KindSite, Name: parts[2], Facility: parts[1]}, true
	case len(parts) == 4 && parts[0] == "topology" && strings.HasSuffix(parts[3], ".yaml") &&
		!strings.HasSuffix(parts[3], "_downtime.yaml"):
		return classified{Kind: models.KindResourceGroup, Name: strings.TrimSuffix(parts[3], ".yaml"), Site: parts[2]}, true
	case len(parts) == 2 && parts[0] == "projects" && strings.HasSuffix(parts[1], ".yaml") && !strings.HasPrefix(parts[1], "_"):
		return classified{Kind: models.KindProject, Name: strings.TrimSuffix(parts[1], ".yaml")}, true
	default:
		return classified{}, false
	}
}
