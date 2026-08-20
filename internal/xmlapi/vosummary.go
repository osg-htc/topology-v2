package xmlapi

import (
	"context"
	"encoding/json"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/bbockelm/topology-v2/internal/db"
)

// VOSummary is the /vosummary/xml root. Field order matches vosummary.xsd.
type VOSummary struct {
	XMLName        struct{} `xml:"VOSummary"`
	XsiNS          string   `xml:"xmlns:xsi,attr"`
	SchemaLocation string   `xml:"xsi:schemaLocation,attr"`
	VOs            []VOXML  `xml:"VO"`
}

type VOXML struct {
	ID                    *int64              `xml:"ID,omitempty"`
	Name                  string              `xml:"Name"`
	LongName              string              `xml:"LongName"`
	CertificateOnly       bool                `xml:"CertificateOnly"`
	PrimaryURL            string              `xml:"PrimaryURL"`
	MembershipServicesURL string              `xml:"MembershipServicesURL,omitempty"`
	PurposeURL            string              `xml:"PurposeURL"`
	SupportURL            string              `xml:"SupportURL"`
	AppDescription        string              `xml:"AppDescription"`
	Community             string              `xml:"Community"`
	FieldsOfScience       *FieldsOfScienceXML `xml:"FieldsOfScience,omitempty"`
	ParentVO              ParentVOXML         `xml:"ParentVO"`
	ReportingGroups       ReportingGroupsXML  `xml:"ReportingGroups"`
	Active                bool                `xml:"Active"`
	Disable               bool                `xml:"Disable"`
	ContactTypes          ContactTypesXML     `xml:"ContactTypes"`
	OASIS                 *OASISXML           `xml:"OASIS,omitempty"`
	Credentials           *CredentialsXML     `xml:"Credentials,omitempty"`
}

type FieldsOfScienceXML struct {
	PrimaryFields   *FieldsXML `xml:"PrimaryFields,omitempty"`
	SecondaryFields *FieldsXML `xml:"SecondaryFields,omitempty"`
}
type FieldsXML struct {
	Field []string `xml:"Field"`
}
type ParentVOXML struct {
	ID   string `xml:"ID,omitempty"`
	Name string `xml:"Name,omitempty"`
}
type ReportingGroupsXML struct {
	Groups []ReportingGroupXML `xml:"ReportingGroup"`
}
type ReportingGroupXML struct {
	Name     string        `xml:"Name"`
	FQANs    FQANsXML      `xml:"FQANs"`
	Contacts VOContactsXML `xml:"Contacts"`
}
type FQANsXML struct {
	FQANs []FQANXML `xml:"FQAN"`
}
type FQANXML struct {
	GroupName string `xml:"GroupName"`
	Role      string `xml:"Role"`
}
type ContactTypesXML struct {
	Types []ContactTypeXML `xml:"ContactType"`
}
type ContactTypeXML struct {
	Type     string        `xml:"Type"`
	Contacts VOContactsXML `xml:"Contacts"`
}
type VOContactsXML struct {
	Contacts []VOContactXML `xml:"Contact"`
}
type VOContactXML struct {
	Name       string `xml:"Name,omitempty"`
	CILogonID  string `xml:"CILogonID,omitempty"`
	Email      string `xml:"Email,omitempty"`
	Phone      string `xml:"Phone,omitempty"`
	SMSAddress string `xml:"SMSAddress,omitempty"`
	DN         string `xml:"DN,omitempty"`
}
type OASISXML struct {
	UseOASIS      bool        `xml:"UseOASIS"`
	Managers      ManagersXML `xml:"Managers"`
	OASISRepoURLs URLsXML     `xml:"OASISRepoURLs"`
}
type ManagersXML struct {
	Managers []ManagerXML `xml:"Manager"`
}
type ManagerXML struct {
	Name string `xml:"Name"`
	// No omitempty: v1 always emits this element, defaulting to empty when
	// no matching contact resolves to a cilogon_id (webapp/vos_data.py's
	// _expand_oasis_managers initializes it to None, not absent).
	CILogonID string `xml:"CILogonID"`
	DNs       DNsXML `xml:"DNs"`
}
type DNsXML struct {
	DN []string `xml:"DN"`
}
type URLsXML struct {
	URL []string `xml:"URL"`
}
type CredentialsXML struct {
	TokenIssuers *TokenIssuersXML `xml:"TokenIssuers,omitempty"`
}
type TokenIssuersXML struct {
	Issuers []TokenIssuerXML `xml:"TokenIssuer"`
}
type TokenIssuerXML struct {
	URL             string `xml:"URL"`
	DefaultUnixUser string `xml:"DefaultUnixUser"`
	Description     string `xml:"Description"`
	Subject         string `xml:"Subject"`
}

// BuildVOSummary assembles /vosummary/xml from the stored VO documents.
func BuildVOSummary(ctx context.Context, q *db.Queries, includePII bool) (*VOSummary, error) {
	vos, err := q.ListVOs(ctx)
	if err != nil {
		return nil, err
	}
	rgRows, err := q.ListReportingGroups(ctx)
	if err != nil {
		return nil, err
	}
	rgByName := make(map[string]db.ReportingGroupRow, len(rgRows))
	for _, r := range rgRows {
		rgByName[r.Name] = r
	}
	out := &VOSummary{
		XsiNS:          "http://www.w3.org/2001/XMLSchema-instance",
		SchemaLocation: "https://topology.opensciencegrid.org/schema/vosummary.xsd",
	}
	for _, v := range vos {
		var m map[string]interface{}
		if err := yaml.Unmarshal(v.Raw, &m); err != nil {
			continue
		}
		id := v.VOID
		vx := VOXML{
			ID:                    &id,
			Name:                  v.Name,
			LongName:              mstr(m, "LongName"),
			CertificateOnly:       mbool(m, "CertificateOnly"),
			PrimaryURL:            mstr(m, "PrimaryURL"),
			MembershipServicesURL: mstr(m, "MembershipServicesURL"),
			PurposeURL:            mstr(m, "PurposeURL"),
			SupportURL:            mstr(m, "SupportURL"),
			AppDescription:        mstr(m, "AppDescription"),
			Community:             mstr(m, "Community"),
			FieldsOfScience:       fieldsOfScienceXML(m["FieldsOfScience"]),
			ParentVO:              ParentVOXML{Name: mstr(m, "ParentVO")},
			ReportingGroups:       reportingGroupsXML(stringList(m["ReportingGroups"]), rgByName),
			Active:                !v.Disable,
			Disable:               v.Disable,
			ContactTypes:          voContactTypesXML(m["Contacts"], includePII),
			OASIS:                 oasisXML(m["OASIS"]),
			Credentials:           credentialsXML(m["Credentials"]),
		}
		out.VOs = append(out.VOs, vx)
	}
	return out, nil
}

func fieldsOfScienceXML(v interface{}) *FieldsOfScienceXML {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	x := &FieldsOfScienceXML{}
	if pf := stringList(m["PrimaryFields"]); len(pf) > 0 {
		x.PrimaryFields = &FieldsXML{Field: pf}
	}
	if sf := stringList(m["SecondaryFields"]); len(sf) > 0 {
		x.SecondaryFields = &FieldsXML{Field: sf}
	}
	if x.PrimaryFields == nil && x.SecondaryFields == nil {
		return nil
	}
	return x
}

func voContactTypesXML(v interface{}, includePII bool) ContactTypesXML {
	m, ok := v.(map[string]interface{})
	out := ContactTypesXML{}
	if !ok {
		return out
	}
	types := make([]string, 0, len(m))
	for t := range m {
		types = append(types, t)
	}
	sort.Strings(types)
	for _, t := range types {
		ct := ContactTypeXML{Type: t}
		for _, item := range asSlice(m[t]) {
			cm, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			c := VOContactXML{Name: getStr(cm, "Name")}
			// Matches the same includePII gate used for resource contacts
			// (internal/handlers/topology_api.go's includePII) -- contact
			// ids are only exposed to a contact_reader session, not
			// anonymous/unprivileged clients.
			if id := getStr(cm, "ID"); includePII && len(id) >= 3 && id[:3] == "OSG" {
				c.CILogonID = id
			}
			ct.Contacts.Contacts = append(ct.Contacts.Contacts, c)
		}
		out.Types = append(out.Types, ct)
	}
	return out
}

// reportingGroupJSONContact/reportingGroupJSONFQAN mirror the JSON shape
// UpsertReportingGroup stores (internal/topology/vos.go's
// reportingGroupContact/reportingGroupFQAN).
type reportingGroupJSONContact struct {
	ID   string `json:"ID"`
	Name string `json:"Name"`
}
type reportingGroupJSONFQAN struct {
	GroupName string `json:"GroupName"`
	Role      string `json:"Role"`
}

// reportingGroupsXML expands a VO's ReportingGroups (a plain list of names)
// into the full registry-backed structure, matching v1's
// _expand_reporting_groups: reporting-group contacts never carry a
// CILogonID (only Name -- v1 also shows Email/Phone/SMSAddress when
// authorized, not reproduced here since no contact-details registry is
// imported for reporting groups yet).
func reportingGroupsXML(names []string, registry map[string]db.ReportingGroupRow) ReportingGroupsXML {
	out := ReportingGroupsXML{}
	sortedNames := append([]string(nil), names...)
	sort.Strings(sortedNames)
	for _, name := range sortedNames {
		row, ok := registry[name]
		if !ok {
			continue
		}
		rg := ReportingGroupXML{Name: name}
		if len(row.Contacts) > 0 {
			var contacts []reportingGroupJSONContact
			if err := json.Unmarshal(row.Contacts, &contacts); err == nil {
				for _, c := range contacts {
					rg.Contacts.Contacts = append(rg.Contacts.Contacts, VOContactXML{Name: c.Name})
				}
			}
		}
		if len(row.FQANs) > 0 {
			var fqans []reportingGroupJSONFQAN
			if err := json.Unmarshal(row.FQANs, &fqans); err == nil {
				for _, f := range fqans {
					rg.FQANs.FQANs = append(rg.FQANs.FQANs, FQANXML{GroupName: f.GroupName, Role: f.Role})
				}
			}
		}
		out.Groups = append(out.Groups, rg)
	}
	return out
}

func oasisXML(v interface{}) *OASISXML {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	x := &OASISXML{UseOASIS: getBool(m, "UseOASIS")}
	for _, item := range asSlice(m["Managers"]) {
		mm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		mgr := ManagerXML{Name: getStr(mm, "Name")}
		if id := getStr(mm, "ID"); len(id) >= 3 && id[:3] == "OSG" {
			mgr.CILogonID = id
		}
		mgr.DNs = DNsXML{DN: stringList(mm["DNs"])}
		x.Managers.Managers = append(x.Managers.Managers, mgr)
	}
	x.OASISRepoURLs = URLsXML{URL: stringList(m["OASISRepoURLs"])}
	return x
}

func credentialsXML(v interface{}) *CredentialsXML {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	issuers := asSlice(m["TokenIssuers"])
	if len(issuers) == 0 {
		return nil // TokenIssuers requires >=1 issuer; omit Credentials otherwise
	}
	ti := &TokenIssuersXML{}
	for _, item := range issuers {
		im, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		ti.Issuers = append(ti.Issuers, TokenIssuerXML{
			URL: getStr(im, "URL"), DefaultUnixUser: getStr(im, "DefaultUnixUser"),
			Description: getStr(im, "Description"), Subject: getStr(im, "Subject"),
		})
	}
	if len(ti.Issuers) == 0 {
		return nil
	}
	return &CredentialsXML{TokenIssuers: ti}
}

// ---- small map helpers ----

func mstr(m map[string]interface{}, key string) string { return getStr(m, key) }
func mbool(m map[string]interface{}, key string) bool  { return getBool(m, key) }

func asSlice(v interface{}) []interface{} {
	if s, ok := v.([]interface{}); ok {
		return s
	}
	return nil
}

// stringList coerces a value that may be a single string or a list of strings.
func stringList(v interface{}) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []interface{}:
		var out []string
		for _, item := range x {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
