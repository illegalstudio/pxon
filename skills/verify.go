package skills

import "fmt"

// VerifyStatus compares an installed skill with the bundled version.
type VerifyStatus string

const (
	VerifyNotInstalled VerifyStatus = "not-installed"
	VerifyUpToDate     VerifyStatus = "up-to-date"
	VerifyOutdated     VerifyStatus = "outdated"
	VerifyModified     VerifyStatus = "modified"
)

func (s VerifyStatus) Stale() bool {
	return s == VerifyOutdated || s == VerifyModified
}

// Verify is read-only, including when an installation marker is missing.
func Verify(destination string) (VerifyStatus, error) {
	if destination == "" {
		return "", fmt.Errorf("skill destination is required")
	}
	digest, err := digestBundledSkill()
	if err != nil {
		return "", err
	}
	state, err := inspectInstalledSkill(destination, digest)
	if err != nil {
		return "", err
	}
	switch {
	case !state.exists:
		return VerifyNotInstalled, nil
	case !state.isDir:
		return "", fmt.Errorf("skill destination is not a directory (or is a symlink): %s", destination)
	case state.current:
		return VerifyUpToDate, nil
	case state.managedUnmodified:
		return VerifyOutdated, nil
	default:
		return VerifyModified, nil
	}
}
