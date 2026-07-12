package xmlapi

import (
	"context"
	"time"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/topology"
)

// Downtimes is the /rgdowntime/xml root, bucketing downtimes by timeframe.
type Downtimes struct {
	XMLName        struct{}      `xml:"Downtimes"`
	XsiNS          string        `xml:"xmlns:xsi,attr"`
	SchemaLocation string        `xml:"xsi:schemaLocation,attr"`
	Past           *DowntimeList `xml:"PastDowntimes"`
	Current        *DowntimeList `xml:"CurrentDowntimes"`
	Future         *DowntimeList `xml:"FutureDowntimes"`
}

type DowntimeList struct {
	Downtimes []DowntimeXML `xml:"Downtime"`
}

type DowntimeXML struct {
	ID            int64             `xml:"ID"`
	ResourceID    int64             `xml:"ResourceID"`
	ResourceGroup DowntimeRGXML     `xml:"ResourceGroup"`
	ResourceName  string            `xml:"ResourceName"`
	ResourceFQDN  string            `xml:"ResourceFQDN"`
	StartTime     string            `xml:"StartTime"`
	EndTime       string            `xml:"EndTime"`
	Class         string            `xml:"Class"`
	Severity      string            `xml:"Severity"`
	CreatedTime   string            `xml:"CreatedTime,omitempty"`
	UpdateTime    string            `xml:"UpdateTime"`
	Services      DowntimeSvcsXML   `xml:"Services"`
	Description   string            `xml:"Description"`
}

type DowntimeRGXML struct {
	GroupName string `xml:"GroupName"`
	GroupID   int64  `xml:"GroupID"`
}
type DowntimeSvcsXML struct {
	Services []DowntimeSvcXML `xml:"Service"`
}
type DowntimeSvcXML struct {
	ID          int64  `xml:"ID"`
	Name        string `xml:"Name"`
	Description string `xml:"Description"`
}

// downtimeTimeLayouts covers the formats seen in the legacy data.
var downtimeTimeLayouts = []string{
	"Jan 02, 2006 15:04 -0700",
	"Jan 2, 2006 15:04 -0700",
	"Jan 02, 2006 15:04 MST",
}

func parseDowntimeTime(s string) (time.Time, bool) {
	for _, l := range downtimeTimeLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// BuildDowntimes assembles /rgdowntime output, bucketed relative to now.
func BuildDowntimes(ctx context.Context, q *db.Queries, f Filters, now time.Time) (*Downtimes, error) {
	rgs, err := q.ListResourceGroups(ctx)
	if err != nil {
		return nil, err
	}
	rgByName := map[string]db.ResourceGroupRow{}
	for _, rg := range rgs {
		rgByName[rg.Name] = rg
	}
	resources, err := q.ListResources(ctx)
	if err != nil {
		return nil, err
	}
	resByName := map[string]db.ResourceRow{}
	for _, r := range resources {
		resByName[r.Name] = r
	}
	dts, err := q.ListDowntimes(ctx)
	if err != nil {
		return nil, err
	}

	out := &Downtimes{
		XsiNS:          "http://www.w3.org/2001/XMLSchema-instance",
		SchemaLocation: "https://topology.opensciencegrid.org/schema/rgdowntime.xsd",
	}
	var past, current, future []DowntimeXML

	for _, d := range dts {
		rg := rgByName[d.RGName]
		if !idAny(f.RGIDs) && !f.RGIDs[rg.GroupID] {
			continue
		}
		res := resByName[d.ResourceName]
		var svcs []DowntimeSvcXML
		for _, name := range d.Services {
			id, ok := q.ServiceIDByName(ctx, name)
			if !ok {
				id = topology.GenID(name)
			}
			svcs = append(svcs, DowntimeSvcXML{ID: id, Name: name})
		}
		dx := DowntimeXML{
			ID:            d.DtID,
			ResourceID:    res.TopologyID,
			ResourceGroup: DowntimeRGXML{GroupName: rg.Name, GroupID: rg.GroupID},
			ResourceName:  d.ResourceName,
			ResourceFQDN:  res.FQDN,
			StartTime:     d.StartTime,
			EndTime:       d.EndTime,
			Class:         d.Class,
			Severity:      d.Severity,
			CreatedTime:   d.CreatedTime,
			UpdateTime:    "Not Available",
			Services:      DowntimeSvcsXML{Services: svcs},
			Description:   d.Description,
		}
		start, okS := parseDowntimeTime(d.StartTime)
		end, okE := parseDowntimeTime(d.EndTime)
		switch {
		case okE && end.Before(now):
			past = append(past, dx)
		case okS && start.After(now):
			future = append(future, dx)
		default:
			current = append(current, dx)
		}
	}
	if len(past) > 0 {
		out.Past = &DowntimeList{Downtimes: past}
	}
	if len(current) > 0 {
		out.Current = &DowntimeList{Downtimes: current}
	}
	if len(future) > 0 {
		out.Future = &DowntimeList{Downtimes: future}
	}
	return out, nil
}
