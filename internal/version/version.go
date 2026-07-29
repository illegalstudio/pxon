package version

import "fmt"

// Set through linker flags in release builds.
var (
	Version = "dev"
	Commit  = "none"
)

// String returns the version shown by pxon --version.
func String() string {
	return fmt.Sprintf("pxon v%s (%s)", Version, Commit)
}
