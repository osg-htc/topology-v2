// Package topology models the on-disk OSG topology YAML data and reads/writes
// the directory tree, so the app can round-trip the existing GitHub repo. The
// structs mirror the YAML shape exactly; an inline `Extra` map captures any keys
// not explicitly modeled, guaranteeing a lossless round-trip.
package topology

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"math/big"
	"strings"
)

// GenID reproduces the legacy id generator used when a YAML entity omits its ID:
//
//	1 + (int(md5(name).hexdigest(), 16) % (2**31 - 1))
//
// It must match the Python implementation exactly or auto-derived ids drift.
func GenID(name string) int64 {
	sum := md5.Sum([]byte(name))
	n := new(big.Int).SetBytes(sum[:]) // == int(hexdigest, 16)
	mod := big.NewInt((1 << 31) - 1)
	r := new(big.Int).Mod(n, mod)
	return 1 + r.Int64()
}

// ContactIDFromEmail is the legacy contact id: SHA1 of the lowercased, trimmed
// email address.
func ContactIDFromEmail(email string) string {
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])
}

// Facility is FACILITY.yaml. The facility name is the directory name. ID is a
// pointer so an absent id (relying on the gen_id fallback) round-trips as absent.
type Facility struct {
	ID            *int64                 `yaml:"ID,omitempty"`
	InstitutionID string                 `yaml:"InstitutionID,omitempty"`
	Extra         map[string]interface{} `yaml:",inline"`

	Name string `yaml:"-"` // from the directory, not the file
}

// Site is SITE.yaml. The site name is the directory name.
type Site struct {
	ID           *int64                 `yaml:"ID,omitempty"`
	LongName     string                 `yaml:"LongName,omitempty"`
	Description  string                 `yaml:"Description,omitempty"`
	AddressLine1 string                 `yaml:"AddressLine1,omitempty"`
	AddressLine2 string                 `yaml:"AddressLine2,omitempty"`
	City         string                 `yaml:"City,omitempty"`
	State        string                 `yaml:"State,omitempty"`
	Country      string                 `yaml:"Country,omitempty"`
	Zipcode      string                 `yaml:"Zipcode,omitempty"`
	Latitude     *float64               `yaml:"Latitude,omitempty"`
	Longitude    *float64               `yaml:"Longitude,omitempty"`
	Extra        map[string]interface{} `yaml:",inline"`

	Name string `yaml:"-"`
}

// ResourceGroup is a <RG>.yaml file. The RG name is the filename minus ".yaml".
type ResourceGroup struct {
	Production       *bool                  `yaml:"Production,omitempty"`
	GroupID          *int64                 `yaml:"GroupID,omitempty"`
	SupportCenter    string                 `yaml:"SupportCenter,omitempty"`
	GroupDescription string                 `yaml:"GroupDescription,omitempty"`
	Resources        map[string]*Resource   `yaml:"Resources"`
	Extra            map[string]interface{} `yaml:",inline"`

	Name string `yaml:"-"`
}

// Resource is the primary entity, nested under a ResourceGroup's Resources map.
type Resource struct {
	ID              *int64                         `yaml:"ID,omitempty"`
	Active          *bool                          `yaml:"Active,omitempty"`
	Description     string                         `yaml:"Description,omitempty"`
	FQDN            string                         `yaml:"FQDN"`
	FQDNAliases     []string                       `yaml:"FQDNAliases,omitempty"`
	DN              string                         `yaml:"DN,omitempty"`
	ContactLists    map[string]map[string]Contact  `yaml:"ContactLists,omitempty"`
	Services        map[string]*Service            `yaml:"Services,omitempty"`
	Tags            []string                       `yaml:"Tags,omitempty"`
	AllowedVOs      []string                       `yaml:"AllowedVOs,omitempty"`
	// VOOwnership/WLCGInformation are normally maps, but a few malformed source
	// files carry scalars there; interface{} tolerates whatever PyYAML accepted.
	VOOwnership     interface{}                    `yaml:"VOOwnership,omitempty"`
	WLCGInformation interface{}                    `yaml:"WLCGInformation,omitempty"`
	Extra           map[string]interface{}         `yaml:",inline"`
}

// Contact is one entry in a ContactLists rank (Primary/Secondary/Tertiary).
type Contact struct {
	Name string `yaml:"Name,omitempty"`
	ID   string `yaml:"ID,omitempty"`
}

// Service is one entry in a resource's Services map. Details is normally a map
// but tolerates malformed scalar values found in a few source files.
type Service struct {
	Description string                 `yaml:"Description,omitempty"`
	Details     interface{}            `yaml:"Details,omitempty"`
	Extra       map[string]interface{} `yaml:",inline"`
}

// Downtime is one entry in a <RG>_downtime.yaml list.
type Downtime struct {
	ID           int64                  `yaml:"ID"`
	Class        string                 `yaml:"Class"`
	Description  string                 `yaml:"Description,omitempty"`
	Severity     string                 `yaml:"Severity,omitempty"`
	StartTime    string                 `yaml:"StartTime"`
	EndTime      string                 `yaml:"EndTime"`
	CreatedTime  string                 `yaml:"CreatedTime,omitempty"`
	ResourceName string                 `yaml:"ResourceName"`
	Services     []string               `yaml:"Services,omitempty"`
	Extra        map[string]interface{} `yaml:",inline"`
}

// Topology is the whole tree loaded from disk.
type Topology struct {
	Facilities     map[string]*Facility        // by facility name
	Sites          map[string]*Site            // by site name
	ResourceGroups map[string]*ResourceGroup   // by RG name
	Downtimes      map[string][]*Downtime      // by RG name
	// Parentage needed to reconstruct the directory tree.
	SiteFacility map[string]string // site name -> facility name
	RGSite       map[string]string // RG name -> site name
}
