package semver

import (
	"testing"

	"github.com/chrisdietr/coolify-patrol/pkg/types"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected *Version
		wantErr  bool
	}{
		{
			input: "1.2.3",
			expected: &Version{
				Major:    1,
				Minor:    2,
				Patch:    3,
				Original: "1.2.3",
			},
		},
		{
			input: "v2.0.1",
			expected: &Version{
				Major:    2,
				Minor:    0,
				Patch:    1,
				Original: "v2.0.1",
			},
		},
		{
			input: "1.0.0-alpha.1",
			expected: &Version{
				Major:      1,
				Minor:      0,
				Patch:      0,
				Prerelease: "alpha.1",
				Original:   "1.0.0-alpha.1",
			},
		},
		{
			input: "1.0.0+build.1",
			expected: &Version{
				Major:    1,
				Minor:    0,
				Patch:    0,
				Build:    "build.1",
				Original: "1.0.0+build.1",
			},
		},
		{
			input:   "latest",
			wantErr: true,
		},
		{
			input:   "not-a-version",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseVersion(tt.input)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %s, got nil", tt.input)
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error for input %s: %v", tt.input, err)
				return
			}
			
			if result.Major != tt.expected.Major ||
				result.Minor != tt.expected.Minor ||
				result.Patch != tt.expected.Patch ||
				result.Prerelease != tt.expected.Prerelease ||
				result.Build != tt.expected.Build ||
				result.Original != tt.expected.Original {
				t.Errorf("expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.1.0", -1},
		{"1.1.0", "1.0.0", 1},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "1.0.0-alpha", 1}, // stable > prerelease
		{"1.0.0-alpha", "1.0.0", -1}, // prerelease < stable
		{"1.0.0-alpha", "1.0.0-beta", -1},
	}

	for _, tt := range tests {
		t.Run(tt.v1+"_vs_"+tt.v2, func(t *testing.T) {
			v1, err := ParseVersion(tt.v1)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", tt.v1, err)
			}
			
			v2, err := ParseVersion(tt.v2)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", tt.v2, err)
			}
			
			result := v1.Compare(v2)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestIsUpdateAllowed(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		latest   string
		policy   types.UpdatePolicy
		pin      string
		allowed  bool
		reason   string
	}{
		{
			name:    "patch update allowed with auto-patch",
			current: "1.2.3",
			latest:  "1.2.4",
			policy:  types.AutoPatch,
			allowed: true,
			reason:  "patch update allowed",
		},
		{
			name:    "minor update blocked with auto-patch",
			current: "1.2.3",
			latest:  "1.3.0",
			policy:  types.AutoPatch,
			allowed: false,
			reason:  "auto-patch policy only allows patch updates",
		},
		{
			name:    "minor update allowed with auto-minor",
			current: "1.2.3",
			latest:  "1.3.0",
			policy:  types.AutoMinor,
			allowed: true,
			reason:  "minor/patch update allowed",
		},
		{
			name:    "major update blocked with auto-minor",
			current: "1.2.3",
			latest:  "2.0.0",
			policy:  types.AutoMinor,
			allowed: false,
			reason:  "auto-minor policy only allows minor and patch updates",
		},
		{
			name:    "major update allowed with auto-all",
			current: "1.2.3",
			latest:  "2.0.0",
			policy:  types.AutoAll,
			allowed: true,
			reason:  "auto-all policy allows all updates",
		},
		{
			name:    "any update blocked with notify-only",
			current: "1.2.3",
			latest:  "1.2.4",
			policy:  types.NotifyOnly,
			allowed: false,
			reason:  "notify-only policy - update available but not applied",
		},
		{
			name:    "update blocked by pin constraint",
			current: "17.1.0",
			latest:  "18.0.0",
			policy:  types.AutoAll,
			pin:     "17",
			allowed: false,
			reason:  "update would cross pin boundary (pinned to major 17)",
		},
		{
			name:    "update allowed within pin constraint",
			current: "17.1.0",
			latest:  "17.2.0",
			policy:  types.AutoMinor,
			pin:     "17",
			allowed: true,
			reason:  "minor/patch update allowed",
		},
		{
			name:    "non-semver tag with auto-all",
			current: "latest",
			latest:  "stable",
			policy:  types.AutoAll,
			allowed: true,
			reason:  "non-semver update (auto-all policy)",
		},
		{
			name:    "non-semver tag blocked by other policies",
			current: "latest",
			latest:  "stable",
			policy:  types.AutoPatch,
			allowed: false,
			reason:  "non-semver tag, policy auto-patch requires explicit semver",
		},
		{
			name:    "no update needed",
			current: "1.2.3",
			latest:  "1.2.3",
			policy:  types.AutoAll,
			allowed: false,
			reason:  "latest version is not newer than current",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, reason := IsUpdateAllowed(tt.current, tt.latest, tt.policy, tt.pin)
			
			if allowed != tt.allowed {
				t.Errorf("expected allowed=%v, got %v", tt.allowed, allowed)
			}
			
			if reason != tt.reason {
				t.Errorf("expected reason='%s', got '%s'", tt.reason, reason)
			}
		})
	}
}

func TestFilterPrereleaseTags(t *testing.T) {
	tags := []string{
		"1.0.0",
		"1.0.1-alpha",
		"1.0.1-beta",
		"1.0.1-rc1",
		"1.0.1",
		"1.0.2-dev",
		"1.0.2",
		"1.1.0-nightly",
		"1.1.0",
	}
	
	excludePatterns := []string{"-alpha", "-beta", "-rc", "-dev", "-nightly"}
	
	result := FilterPrereleaseTags(tags, excludePatterns)
	expected := []string{"1.0.0", "1.0.1", "1.0.2", "1.1.0"}
	
	if len(result) != len(expected) {
		t.Errorf("expected %d tags, got %d", len(expected), len(result))
		return
	}
	
	for i, tag := range expected {
		if result[i] != tag {
			t.Errorf("expected tag %s at position %d, got %s", tag, i, result[i])
		}
	}
}

func TestFindLatestVersion(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected string
		wantErr  bool
	}{
		{
			name:     "semver tags",
			tags:     []string{"1.0.0", "1.0.1", "1.1.0", "2.0.0", "1.0.2"},
			expected: "2.0.0",
		},
		{
			name:     "mixed semver and non-semver",
			tags:     []string{"1.0.0", "latest", "stable", "1.1.0"},
			expected: "1.1.0",
		},
		{
			name:     "only non-semver tags",
			tags:     []string{"latest", "stable", "edge"},
			expected: "stable", // lexicographically largest
		},
		{
			name:     "prereleases filtered out",
			tags:     []string{"1.0.0", "1.0.1-alpha", "1.0.2-beta"},
			expected: "1.0.0",
		},
		{
			name:    "no tags",
			tags:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FindLatestVersion(tt.tags)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}