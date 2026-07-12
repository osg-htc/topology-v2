// Package version carries build-time version metadata, injected via -ldflags.
package version

// These are overridden at build time:
//
//	-ldflags "-X github.com/bbockelm/topology-v2/internal/version.Version=... -X ...Commit=..."
var (
	Version = "dev"
	Commit  = "none"
)
