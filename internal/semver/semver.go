package semver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/chrisdietr/coolify-patrol/pkg/types"
)

var semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9\-\.]+))?(?:\+([a-zA-Z0-9\-\.]+))?$`)

// Version represents a parsed semantic version
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
	Original   string
}

// ParseVersion parses a version string into a Version struct
func ParseVersion(v string) (*Version, error) {
	matches := semverRegex.FindStringSubmatch(v)
	if matches == nil {
		return nil, fmt.Errorf("not a valid semver: %s", v)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return &Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: matches[4],
		Build:      matches[5],
		Original:   v,
	}, nil
}

// String returns the string representation of the version
func (v *Version) String() string {
	return v.Original
}

// Compare compares two versions. Returns:
// -1 if v < other
//  0 if v == other
//  1 if v > other
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	
	// Compare prerelease versions
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1 // stable version > prerelease
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1 // prerelease < stable version
	}
	if v.Prerelease != "" && other.Prerelease != "" {
		return strings.Compare(v.Prerelease, other.Prerelease)
	}
	
	return 0
}

// IsUpdateAllowed checks if updating from current to latest is allowed by policy
func IsUpdateAllowed(current, latest string, policy types.UpdatePolicy, pin string) (bool, string) {
	// Parse versions
	currentVer, err := ParseVersion(current)
	if err != nil {
		// Non-semver tags - only allow if policy is auto-all or notify-only
		if policy == types.AutoAll {
			return current != latest, "non-semver update (auto-all policy)"
		}
		return false, fmt.Sprintf("non-semver tag, policy %s requires explicit semver", policy)
	}
	
	latestVer, err := ParseVersion(latest)
	if err != nil {
		return false, fmt.Sprintf("latest tag %s is not semver", latest)
	}
	
	// Check if we're moving backwards (should never happen but safety check)
	if latestVer.Compare(currentVer) <= 0 {
		return false, "latest version is not newer than current"
	}
	
	// Check pin constraint
	if pin != "" {
		pinMajor, err := strconv.Atoi(pin)
		if err != nil {
			return false, fmt.Sprintf("invalid pin value: %s", pin)
		}
		if latestVer.Major != pinMajor {
			return false, fmt.Sprintf("update would cross pin boundary (pinned to major %d)", pinMajor)
		}
	}
	
	// Apply update policy
	switch policy {
	case types.NotifyOnly:
		return false, "notify-only policy - update available but not applied"
		
	case types.AutoPatch:
		if latestVer.Major != currentVer.Major || latestVer.Minor != currentVer.Minor {
			return false, "auto-patch policy only allows patch updates"
		}
		return true, "patch update allowed"
		
	case types.AutoMinor:
		if latestVer.Major != currentVer.Major {
			return false, "auto-minor policy only allows minor and patch updates"
		}
		return true, "minor/patch update allowed"
		
	case types.AutoAll:
		return true, "auto-all policy allows all updates"
		
	default:
		return false, fmt.Sprintf("unknown update policy: %s", policy)
	}
}

// FilterPrereleaseTags removes prerelease tags based on exclude patterns
func FilterPrereleaseTags(tags []string, excludePatterns []string) []string {
	var filtered []string
	
	for _, tag := range tags {
		excluded := false
		for _, pattern := range excludePatterns {
			if strings.Contains(tag, pattern) {
				excluded = true
				break
			}
		}
		if !excluded {
			filtered = append(filtered, tag)
		}
	}
	
	return filtered
}

// FindLatestVersion finds the latest semantic version from a list of tags
func FindLatestVersion(tags []string) (string, error) {
	var versions []*Version
	
	// Parse all valid semver tags
	for _, tag := range tags {
		if version, err := ParseVersion(tag); err == nil && version.Prerelease == "" {
			versions = append(versions, version)
		}
	}
	
	if len(versions) == 0 {
		// No valid semver found, return lexicographically largest tag
		if len(tags) == 0 {
			return "", fmt.Errorf("no tags found")
		}
		
		latest := tags[0]
		for _, tag := range tags[1:] {
			if tag > latest {
				latest = tag
			}
		}
		return latest, nil
	}
	
	// Find the highest version
	latest := versions[0]
	for _, version := range versions[1:] {
		if version.Compare(latest) > 0 {
			latest = version
		}
	}
	
	return latest.Original, nil
}