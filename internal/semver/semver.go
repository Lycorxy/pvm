package semver

import (
	"fmt"
	"sort"
	"strings"
)

// Version represents a parsed semantic version
type Version struct {
	Major int
	Minor int
	Patch int
	Raw   string
}

// Parse parses a version string like "1.22.0" or "20.11.0"
func Parse(s string) Version {
	v := Version{Raw: s}
	s = strings.TrimPrefix(s, "v")
	parts := strings.SplitN(s, ".", 3)

	if len(parts) >= 1 {
		_, _ = fmt.Sscanf(parts[0], "%d", &v.Major) // 忽略错误，使用默认值 0
	}
	if len(parts) >= 2 {
		_, _ = fmt.Sscanf(parts[1], "%d", &v.Minor) // 忽略错误，使用默认值 0
	}
	if len(parts) >= 3 {
		// Handle patch versions like "0-rc1"
		patchStr := parts[2]
		if idx := strings.IndexAny(patchStr, "-+"); idx >= 0 {
			patchStr = patchStr[:idx]
		}
		_, _ = fmt.Sscanf(patchStr, "%d", &v.Patch) // 忽略错误，使用默认值 0
	}

	return v
}

// Compare returns >0 if a>b, <0 if a<b, 0 if a==b
func Compare(a, b Version) int {
	if a.Major != b.Major {
		return a.Major - b.Major
	}
	if a.Minor != b.Minor {
		return a.Minor - b.Minor
	}
	return a.Patch - b.Patch
}

// SortStringsDesc sorts version strings in descending semver order
func SortStringsDesc(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		return Compare(Parse(versions[i]), Parse(versions[j])) > 0
	})
}
