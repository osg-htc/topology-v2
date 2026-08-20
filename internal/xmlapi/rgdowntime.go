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
	ID            int64           `xml:"ID"`
	ResourceID    int64           `xml:"ResourceID"`
	ResourceGroup DowntimeRGXML   `xml:"ResourceGroup"`
	ResourceName  string          `xml:"ResourceName"`
	ResourceFQDN  string          `xml:"ResourceFQDN"`
	StartTime     string          `xml:"StartTime"`
	EndTime       string          `xml:"EndTime"`
	Class         string          `xml:"Class"`
	Severity      string          `xml:"Severity"`
	CreatedTime   string          `xml:"CreatedTime,omitempty"`
	UpdateTime    string          `xml:"UpdateTime"`
	Services      DowntimeSvcsXML `xml:"Services"`
	Description   string          `xml:"Description"`
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

// downtimeOutputLayout matches v1's Downtime.TIME_OUTPUT_FMT exactly
// ("%b %d, %Y %H:%M %p %Z", webapp/topology.py) -- including its bug: %p
// (12-hour AM/PM) is rendered alongside %H (24-hour), giving nonsensical but
// real output like "Jun 10, 2025 13:38 PM UTC". Reproduced verbatim, not
// "corrected", per the carbon-copy mandate. Go's "PM" token computes AM/PM
// correctly from the actual hour regardless of the "15" token also present,
// matching Python's %p behavior here exactly.
const downtimeOutputLayout = "Jan 02, 2006 15:04 PM MST"

// formatDowntimeTimeOrRaw renders a parsed time in v1's output format, or
// falls back to the original stored string if parsing failed (rather than
// silently dropping the value).
func formatDowntimeTimeOrRaw(t time.Time, ok bool, raw string) string {
	if !ok {
		return raw
	}
	return t.UTC().Format(downtimeOutputLayout)
}

// formatCreatedTime matches v1's CreatedTime handling: the literal
// "Not Available" when absent, else the same output format as
// StartTime/EndTime.
func formatCreatedTime(raw string) string {
	if raw == "" {
		return "Not Available"
	}
	if t, ok := parseDowntimeTime(raw); ok {
		return t.UTC().Format(downtimeOutputLayout)
	}
	return raw
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
		// A downtime service is only included if it matches one of the
		// resource's actual services (by name), and its Description comes
		// from that resource service, not the downtime record itself --
		// matching v1's _expand_downtime exactly. A downtime with zero
		// matching services is excluded entirely (v1 returns None for it).
		resSvcs, _ := q.ListResourceServices(ctx, res.TopologyID)
		resSvcDescByName := make(map[string]string, len(resSvcs))
		for _, rs := range resSvcs {
			resSvcDescByName[rs.ServiceName] = rs.Description
		}
		var svcs []DowntimeSvcXML
		for _, name := range d.Services {
			desc, matched := resSvcDescByName[name]
			if !matched {
				continue
			}
			id, ok := q.ServiceIDByName(ctx, name)
			if !ok {
				id = topology.GenID(name)
			}
			svcs = append(svcs, DowntimeSvcXML{ID: id, Name: name, Description: desc})
		}
		if len(svcs) == 0 {
			continue
		}
		start, okS := parseDowntimeTime(d.StartTime)
		end, okE := parseDowntimeTime(d.EndTime)
		dx := DowntimeXML{
			ID:            d.DtID,
			ResourceID:    res.TopologyID,
			ResourceGroup: DowntimeRGXML{GroupName: rg.Name, GroupID: rg.GroupID},
			ResourceName:  d.ResourceName,
			ResourceFQDN:  res.FQDN,
			StartTime:     formatDowntimeTimeOrRaw(start, okS, d.StartTime),
			EndTime:       formatDowntimeTimeOrRaw(end, okE, d.EndTime),
			Class:         d.Class,
			Severity:      d.Severity,
			CreatedTime:   formatCreatedTime(d.CreatedTime),
			UpdateTime:    "Not Available",
			Services:      DowntimeSvcsXML{Services: svcs},
			Description:   d.Description,
		}
		switch {
		case okE && end.Before(now):
			// Matches v1's default: past_days == 0 means no past downtime is
			// shown at all (any past downtime has end_age > 0 by
			// definition); past_days == -1 ("all") shows every past
			// downtime unbounded; a positive N shows only those whose
			// end_time is within the last N days. Current/future are never
			// filtered by this.
			if f.PastDays >= 0 {
				endAgeSeconds := now.Sub(end).Seconds()
				if endAgeSeconds > float64(f.PastDays)*86400 {
					continue
				}
			}
			past = append(past, dx)
		case okS && start.After(now):
			future = append(future, dx)
		default:
			current = append(current, dx)
		}
	}
	// v1 always renders all three buckets, even empty (e.g.
	// <PastDowntimes></PastDowntimes>) -- the XSD marks them optional
	// (minOccurs="0"), but matching v1's actual shape is the point, not just
	// minimal schema validity.
	out.Past = &DowntimeList{Downtimes: past}
	out.Current = &DowntimeList{Downtimes: current}
	out.Future = &DowntimeList{Downtimes: future}
	return out, nil
}
