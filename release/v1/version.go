package release

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version is a strict release version without build metadata.
type Version struct {
	Major, Minor, Patch int
	Prerelease          string
}

var versionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// ParseVersion accepts vMAJOR.MINOR.PATCH with an optional SemVer prerelease.
func ParseVersion(s string) (Version, error) {
	parts := versionPattern.FindStringSubmatch(s)
	if parts == nil {
		return Version{}, fmt.Errorf("version %q: %w", s, ErrInvalidManifest)
	}
	var v Version
	fields := []*int{&v.Major, &v.Minor, &v.Patch}
	for i, field := range fields {
		n, err := strconv.Atoi(parts[i+1])
		if err != nil {
			return Version{}, fmt.Errorf("version %q component: %w: %w", s, ErrInvalidManifest, err)
		}
		*field = n
	}
	for _, id := range strings.Split(parts[4], ".") {
		if id == "" {
			continue
		}
		if len(id) > 1 && id[0] == '0' && isNumeric(id) {
			return Version{}, fmt.Errorf("version %q numeric prerelease identifier: %w", s, ErrInvalidManifest)
		}
	}
	v.Prerelease = parts[4]
	return v, nil
}

func isNumeric(s string) bool {
	for i := range s {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// CompareVersions returns -1, 0, or 1 according to SemVer precedence.
func CompareVersions(a, b string) (int, error) {
	av, err := ParseVersion(a)
	if err != nil {
		return 0, fmt.Errorf("first version: %w", err)
	}
	bv, err := ParseVersion(b)
	if err != nil {
		return 0, fmt.Errorf("second version: %w", err)
	}
	for i, n := range [...]int{av.Major, av.Minor, av.Patch} {
		other := [...]int{bv.Major, bv.Minor, bv.Patch}[i]
		if n < other {
			return -1, nil
		}
		if n > other {
			return 1, nil
		}
	}
	if av.Prerelease == bv.Prerelease {
		return 0, nil
	}
	if av.Prerelease == "" {
		return 1, nil
	}
	if bv.Prerelease == "" {
		return -1, nil
	}
	aa, bb := strings.Split(av.Prerelease, "."), strings.Split(bv.Prerelease, ".")
	for i := 0; i < len(aa) && i < len(bb); i++ {
		if aa[i] == bb[i] {
			continue
		}
		an, bn := isNumeric(aa[i]), isNumeric(bb[i])
		if an && !bn {
			return -1, nil
		}
		if !an && bn {
			return 1, nil
		}
		if an {
			if len(aa[i]) < len(bb[i]) {
				return -1, nil
			}
			if len(aa[i]) > len(bb[i]) {
				return 1, nil
			}
		}
		if aa[i] < bb[i] {
			return -1, nil
		}
		return 1, nil
	}
	if len(aa) < len(bb) {
		return -1, nil
	}
	return 1, nil
}
