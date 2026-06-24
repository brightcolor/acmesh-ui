// Package version holds build-time version metadata, injected via -ldflags.
package version

// These are overridden at build time:
//
//	go build -ldflags "-X .../internal/version.Version=v1.0.0 ..."
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// String returns a compact human-readable version line.
func String() string {
	return Version + " (commit " + Commit + ", built " + BuildDate + ")"
}
