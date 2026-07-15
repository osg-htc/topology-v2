package xmlapi

import (
	"context"
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
	Groups []struct{} `xml:"ReportingGroup"`
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
	Name      string `xml:"Name"`
	CILogonID string `xml:"CILogonID,omitempty"`
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
func BuildVOSummary(ctx context.Context, q *db.Queries) (*VOSummary, error) {
	vos, err := q.ListVOs(ctx)
	if err != nil {
		return nil, err
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
			Active:                !v.Disable,
			Disable:               v.Disable,
			ContactTypes:          voContactTypesXML(m["Contacts"]),
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

func voContactTypesXML(v interface{}) ContactTypesXML {
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
			if id := getStr(cm, "ID"); len(id) >= 3 && id[:3] == "OSG" {
				c.CILogonID = id
			}
			ct.Contacts.Contacts = append(ct.Contacts.Contacts, c)
		}
		out.Types = append(out.Types, ct)
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
