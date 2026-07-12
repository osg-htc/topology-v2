// Package xmlapi reproduces the legacy OSG topology web API output — primarily
// the XML summaries (/rgsummary/xml, /rgdowntime/xml) — so existing consumers
// keep working. The Go structs are ordered to match the XSDs in schema/ exactly;
// encoding/xml preserves struct field order on marshal.
package xmlapi

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/bbockelm/topology-v2/internal/crypto"
	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/topology"
)

// GridType strings (from the legacy common.py).
const (
	gridTypeProduction = "OSG Production Resource"
	gridTypeITB        = "OSG Integration Test Bed Resource"
)

// Filters selects which resource groups/resources appear in the summary.
type Filters struct {
	FacilityIDs map[int64]bool
	SiteIDs     map[int64]bool
	RGIDs       map[int64]bool
	SCIDs       map[int64]bool
	ServiceIDs  map[int64]bool
	Active      *bool
	GridTypeProd bool // include production
	GridTypeITB  bool // include ITB
	HasWLCG      bool
}

// empty reports whether a filter set is unset (matches everything).
func idAny(m map[int64]bool) bool { return len(m) == 0 }

// ---- XSD-ordered DTOs ----

type ResourceSummary struct {
	XMLName        struct{} `xml:"ResourceSummary"`
	XsiNS          string   `xml:"xmlns:xsi,attr"`
	SchemaLocation string   `xml:"xsi:schemaLocation,attr"`
	ResourceGroups []RGXML  `xml:"ResourceGroup"`
}

type RGXML struct {
	GridType         string       `xml:"GridType"`
	GroupID          int64        `xml:"GroupID"`
	GroupName        string       `xml:"GroupName"`
	Disable          bool         `xml:"Disable"`
	Facility         FacilityXML  `xml:"Facility"`
	Site             SiteXML      `xml:"Site"`
	SupportCenter    SCXML        `xml:"SupportCenter"`
	GroupDescription string       `xml:"GroupDescription,omitempty"`
	IsCCStar         bool         `xml:"IsCCStar"`
	Production       bool         `xml:"Production"`
	Resources        ResourcesXML `xml:"Resources"`
}

type FacilityXML struct {
	ID            int64  `xml:"ID"`
	InstitutionID string `xml:"InstitutionID"`
	Name          string `xml:"Name"`
	IsCCStar      bool   `xml:"IsCCStar"`
}

type SiteXML struct {
	ID           int64  `xml:"ID"`
	Name         string `xml:"Name"`
	IsCCStar     bool   `xml:"IsCCStar"`
	AddressLine1 string `xml:"AddressLine1,omitempty"`
	AddressLine2 string `xml:"AddressLine2,omitempty"`
	City         string `xml:"City,omitempty"`
	Country      string `xml:"Country,omitempty"`
	Description  string `xml:"Description,omitempty"`
	Latitude     string `xml:"Latitude,omitempty"`
	LongName     string `xml:"LongName,omitempty"`
	Longitude    string `xml:"Longitude,omitempty"`
	State        string `xml:"State,omitempty"`
	Zipcode      string `xml:"Zipcode,omitempty"`
}

type SCXML struct {
	ID   int64  `xml:"ID"`
	Name string `xml:"Name"`
}

type ResourcesXML struct {
	Resources []ResourceXML `xml:"Resource"`
}

type ResourceXML struct {
	ID           int64           `xml:"ID"`
	Name         string          `xml:"Name"`
	Active       bool            `xml:"Active"`
	Disable      bool            `xml:"Disable"`
	Services     *ServicesXML    `xml:"Services,omitempty"`
	Tags         TagsXML         `xml:"Tags"`
	Description  string          `xml:"Description,omitempty"`
	FQDN         string          `xml:"FQDN"`
	FQDNAliases  FQDNAliasesXML  `xml:"FQDNAliases"`
	VOOwnership  VOOwnershipXML  `xml:"VOOwnership"`
	WLCG         WLCGXML         `xml:"WLCGInformation"`
	ContactLists ContactListsXML `xml:"ContactLists"`
	IsCCStar     bool            `xml:"IsCCStar"`
}

type ServicesXML struct {
	Services []ServiceXML `xml:"Service"`
}
type ServiceXML struct {
	ID          int64  `xml:"ID"`
	Name        string `xml:"Name"`
	Description string `xml:"Description"`
	Details     string `xml:"Details"`
}
type TagsXML struct {
	Tags []string `xml:"Tag"`
}
type FQDNAliasesXML struct {
	Aliases []string `xml:"FQDNAlias"`
}
type VOOwnershipXML struct {
	Ownership []OwnershipXML `xml:"Ownership"`
	ChartURL  *string        `xml:"ChartURL,omitempty"`
}
type OwnershipXML struct {
	Percent int    `xml:"Percent"`
	VO      string `xml:"VO"`
}

// WLCGXML renders <WLCGInformation>. The inner sequence is optional in the XSD,
// so when a resource has no WLCG data all fields are nil and an empty element is
// emitted. When data is present, every XSD-required field is populated (with
// defaults for any missing) so the sequence validates; the two optional fields
// (APELNormalFactor, HEPScore23Percentage) are emitted only when present.
// Field order matches rgsummary.xsd.
type WLCGXML struct {
	InteropBDII          *bool    `xml:"InteropBDII,omitempty"`
	LDAPURL              *string  `xml:"LDAPURL,omitempty"`
	InteropMonitoring    *bool    `xml:"InteropMonitoring,omitempty"`
	InteropAccounting    *bool    `xml:"InteropAccounting,omitempty"`
	AccountingName       *string  `xml:"AccountingName,omitempty"`
	KSI2KMin             *float64 `xml:"KSI2KMin,omitempty"`
	KSI2KMax             *float64 `xml:"KSI2KMax,omitempty"`
	StorageCapacityMin   *string  `xml:"StorageCapacityMin,omitempty"`
	StorageCapacityMax   *string  `xml:"StorageCapacityMax,omitempty"`
	HEPSPEC              *string  `xml:"HEPSPEC,omitempty"`
	APELNormalFactor     *string  `xml:"APELNormalFactor,omitempty"`
	HEPScore23Percentage *float64 `xml:"HEPScore23Percentage,omitempty"`
	TapeCapacity         *string  `xml:"TapeCapacity,omitempty"`
}

type ContactListsXML struct {
	ContactLists []ContactListXML `xml:"ContactList"`
}
type ContactListXML struct {
	ContactType string       `xml:"ContactType"`
	Contacts    ContactsXML  `xml:"Contacts"`
}
type ContactsXML struct {
	Contacts []ContactXML `xml:"Contact"`
}
type ContactXML struct {
	Name        string `xml:"Name,omitempty"`
	CILogonID   string `xml:"CILogonID,omitempty"`
	Email       string `xml:"Email,omitempty"`
	ContactRank string `xml:"ContactRank,omitempty"`
}

// BuildResourceSummary assembles the ResourceSummary from the database.
// includePII controls whether contact emails are exposed (auth-gated).
func BuildResourceSummary(ctx context.Context, q *db.Queries, enc *crypto.Encryptor, f Filters, includePII bool) (*ResourceSummary, error) {
	facs, err := q.ListFacilities(ctx)
	if err != nil {
		return nil, err
	}
	facByName := map[string]db.FacilityRow{}
	for _, r := range facs {
		facByName[r.Name] = r
	}
	sites, err := q.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	siteByName := map[string]db.SiteRow{}
	for _, s := range sites {
		siteByName[s.Name] = s
	}
	rgs, err := q.ListResourceGroups(ctx)
	if err != nil {
		return nil, err
	}
	resources, err := q.ListResources(ctx)
	if err != nil {
		return nil, err
	}
	resByRG := map[string][]db.ResourceRow{}
	for _, r := range resources {
		resByRG[r.RGName] = append(resByRG[r.RGName], r)
	}

	out := &ResourceSummary{
		XsiNS:          "http://www.w3.org/2001/XMLSchema-instance",
		SchemaLocation: "https://topology.opensciencegrid.org/schema/rgsummary.xsd",
	}

	sort.Slice(rgs, func(i, j int) bool {
		return strings.ToLower(rgs[i].Name) < strings.ToLower(rgs[j].Name)
	})

	for _, rg := range rgs {
		if !idAny(f.RGIDs) && !f.RGIDs[rg.GroupID] {
			continue
		}
		production := rg.Production == nil || *rg.Production
		if (production && !f.GridTypeProd && f.gridTypeSet()) ||
			(!production && !f.GridTypeITB && f.gridTypeSet()) {
			continue
		}
		site := siteByName[rg.SiteName]
		fac := facByName[site.FacilityName]
		if !idAny(f.FacilityIDs) && !f.FacilityIDs[fac.TopologyID] {
			continue
		}
		if !idAny(f.SiteIDs) && !f.SiteIDs[site.TopologyID] {
			continue
		}
		scID := int64(0)
		if rg.SupportCenter != "" {
			if id, ok := q.SupportCenterIDByName(ctx, rg.SupportCenter); ok {
				scID = id
			} else {
				scID = topology.GenID(rg.SupportCenter)
			}
		}
		if !idAny(f.SCIDs) && !f.SCIDs[scID] {
			continue
		}

		resXMLs, ccstar, matchedService := buildResources(ctx, q, enc, resByRG[rg.Name], f, includePII)
		// A service filter is set but no resource in this group offers it.
		if !idAny(f.ServiceIDs) && !matchedService {
			continue
		}
		// A resource-level filter (active/service) removed every resource.
		if len(resXMLs) == 0 && (f.Active != nil || !idAny(f.ServiceIDs)) {
			continue
		}

		gt := gridTypeProduction
		if !production {
			gt = gridTypeITB
		}
		rgx := RGXML{
			GridType:         gt,
			GroupID:          rg.GroupID,
			GroupName:        rg.Name,
			Disable:          false,
			Facility:         FacilityXML{ID: fac.TopologyID, InstitutionID: fac.InstitutionID, Name: fac.Name, IsCCStar: ccstar},
			Site:             siteXML(site, ccstar),
			SupportCenter:    SCXML{ID: scID, Name: rg.SupportCenter},
			GroupDescription: rg.GroupDescription,
			IsCCStar:         ccstar,
			Production:       production,
			Resources:        ResourcesXML{Resources: resXMLs},
		}
		out.ResourceGroups = append(out.ResourceGroups, rgx)
	}
	return out, nil
}

func (f Filters) gridTypeSet() bool { return f.GridTypeProd || f.GridTypeITB }

func siteXML(s db.SiteRow, ccstar bool) SiteXML {
	x := SiteXML{
		ID: s.TopologyID, Name: s.Name, IsCCStar: ccstar,
		AddressLine1: s.AddressLine1, AddressLine2: s.AddressLine2, City: s.City,
		Country: s.Country, Description: s.Description, LongName: s.LongName,
		State: s.State, Zipcode: s.Zipcode,
	}
	if s.Latitude != nil {
		x.Latitude = strconv.FormatFloat(*s.Latitude, 'f', -1, 64)
	}
	if s.Longitude != nil {
		x.Longitude = strconv.FormatFloat(*s.Longitude, 'f', -1, 64)
	}
	return x
}

// buildResources renders a resource group's resources, returning whether any
// resource carries the CC* tag (bubbles up) and whether a service filter matched.
func buildResources(ctx context.Context, q *db.Queries, enc *crypto.Encryptor, rows []db.ResourceRow, f Filters, includePII bool) ([]ResourceXML, bool, bool) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	var out []ResourceXML
	ccstar := false
	matchedService := false

	for _, r := range rows {
		active := r.Active != nil && *r.Active
		if f.Active != nil && active != *f.Active {
			continue
		}
		svcs, _ := q.ListResourceServices(ctx, r.ID)
		var svcXMLs []ServiceXML
		for _, s := range svcs {
			id, ok := q.ServiceIDByName(ctx, s.ServiceName)
			if !ok {
				id = topology.GenID(s.ServiceName)
			}
			if !idAny(f.ServiceIDs) && f.ServiceIDs[id] {
				matchedService = true
			}
			svcXMLs = append(svcXMLs, ServiceXML{ID: id, Name: s.ServiceName, Description: s.Description})
		}

		isCC := false
		for _, tag := range r.Tags {
			if tag == "CC*" {
				isCC = true
				ccstar = true
			}
		}

		rx := ResourceXML{
			ID: r.TopologyID, Name: r.Name, Active: active, Disable: !active,
			Description: r.Description, FQDN: r.FQDN,
			Tags:        TagsXML{Tags: r.Tags},
			FQDNAliases: FQDNAliasesXML{Aliases: r.FQDNAliases},
			VOOwnership: voOwnershipXML(r.VOOwnership),
			WLCG:        wlcgXML(r.WLCGInformation),
			ContactLists: buildContactLists(ctx, q, enc, r.ID, includePII),
			IsCCStar:    isCC,
		}
		if len(svcXMLs) > 0 {
			rx.Services = &ServicesXML{Services: svcXMLs}
		}
		out = append(out, rx)
	}
	return out, ccstar, matchedService
}

func voOwnershipXML(v []byte) VOOwnershipXML {
	m := parseJSONMap(v)
	x := VOOwnershipXML{}
	if len(m) == 0 {
		return x
	}
	vos := make([]string, 0, len(m))
	for vo := range m {
		vos = append(vos, vo)
	}
	sort.Strings(vos)
	for _, vo := range vos {
		x.Ownership = append(x.Ownership, OwnershipXML{Percent: toInt(m[vo]), VO: vo})
	}
	empty := ""
	x.ChartURL = &empty
	return x
}

func buildContactLists(ctx context.Context, q *db.Queries, enc *crypto.Encryptor, resourceID string, includePII bool) ContactListsXML {
	contacts, _ := q.ListResourceContacts(ctx, resourceID)
	byType := map[string][]ContactXML{}
	order := []string{}
	for _, c := range contacts {
		if _, ok := byType[c.ContactType]; !ok {
			order = append(order, c.ContactType)
		}
		cx := ContactXML{Name: c.ContactName, ContactRank: c.Rank}
		if strings.HasPrefix(c.ContactID, "OSG") {
			cx.CILogonID = c.ContactID
		}
		// Email is auth-gated: only for authorized (authenticated) requests, and
		// only when the contact resolves to an account with a decryptable email.
		if includePII && enc != nil && c.ContactID != "" {
			if ct, wrapped, ok := q.EncryptedEmailByContactID(ctx, c.ContactID); ok {
				if email, err := enc.DecryptPII(&crypto.EncryptedField{Ciphertext: ct, WrappedDEK: wrapped}); err == nil {
					cx.Email = email
				}
			}
		}
		byType[c.ContactType] = append(byType[c.ContactType], cx)
	}
	out := ContactListsXML{}
	for _, ct := range order {
		out.ContactLists = append(out.ContactLists, ContactListXML{
			ContactType: ct, Contacts: ContactsXML{Contacts: byType[ct]},
		})
	}
	return out
}

// wlcgXML builds the WLCGInformation element from the stored map. An empty map
// yields an empty element; otherwise every XSD-required field is populated.
func wlcgXML(v []byte) WLCGXML {
	m := parseJSONMap(v)
	if len(m) == 0 {
		return WLCGXML{}
	}
	b := func(key string) *bool { v := getBool(m, key); return &v }
	s := func(key string) *string { v := getStr(m, key); return &v }
	f := func(key string) *float64 { v := getFloat(m, key); return &v }

	x := WLCGXML{
		InteropBDII:        b("InteropBDII"),
		LDAPURL:            s("LDAPURL"),
		InteropMonitoring:  b("InteropMonitoring"),
		InteropAccounting:  b("InteropAccounting"),
		AccountingName:     s("AccountingName"),
		KSI2KMin:           f("KSI2KMin"),
		KSI2KMax:           f("KSI2KMax"),
		StorageCapacityMin: s("StorageCapacityMin"),
		StorageCapacityMax: s("StorageCapacityMax"),
		HEPSPEC:            s("HEPSPEC"),
		TapeCapacity:       s("TapeCapacity"),
	}
	// Optional fields: only when present in the source.
	if _, ok := m["APELNormalFactor"]; ok {
		v := getStr(m, "APELNormalFactor")
		x.APELNormalFactor = &v
	}
	if _, ok := m["HEPScore23Percentage"]; ok {
		v := getFloat(m, "HEPScore23Percentage")
		x.HEPScore23Percentage = &v
	}
	return x
}
