package version

import "fmt"

const (
	// APIVersion is the currently served Kubernetes API version for custom resources.
	APIVersion = "v1alpha1"
)

var (
	// Version identifies the operator build version.
	Version = "0.1.0-dev"
	// GitCommit identifies the source revision used for the build.
	GitCommit = "unknown"
	// BuildDate identifies when the binary was built.
	BuildDate = "unknown"
)

// String returns a compact human-readable build identifier.
func String() string {
	return fmt.Sprintf("%s (commit=%s, built=%s)", Version, GitCommit, BuildDate)
}
