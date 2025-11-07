package releasehandler

import (
	"fmt"
	"strings"
)

const maxReleaseLength = 256

// validateRelease ensures the release name is non-empty, reasonably sized, and has no control characters
func validateRelease(value string) error {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) == 0 {
		return fmt.Errorf("`release` must be a non-empty string")
	}

	if len(trimmed) > maxReleaseLength {
		return fmt.Errorf("`release` is too long (max %d)", maxReleaseLength)
	}

	for _, r := range trimmed {
		// Disallow ASCII control characters
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("`release` contains control characters")
		}
	}
	return nil
}


