package version

import (
	"fmt"
	"runtime/debug"
)

var (
	// These variables are set via -ldflags during build
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Get returns the full version string, attempting to resolve from debug.BuildInfo
// if the package was installed as a module dependency.
func Get() string {
	// If it's a dev build (no ldflags), try to get info from runtime
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, dep := range info.Deps {
				// If used as a library, find our own module version
				if dep.Path == "github.com/mydehq/autotitle" {
					return dep.Version
				}
			}
			// If running from source (go run), main module info might be available
			if info.Main.Path == "github.com/mydehq/autotitle" {
				return info.Main.Version
			}
		}
	}
	return Version
}

// String returns a formatted version string
func String() string {
	return fmt.Sprintf("%s (Commit: %s, Built: %s)", Get(), Commit, Date)
}
