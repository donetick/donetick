package utils

import "regexp"

// Username validation regex: only lowercase a-z, digits 0-9, dots, and hyphens
var usernameRegex = regexp.MustCompile(`^[a-z0-9.-]+$`)

// IsValidUsername checks if username contains only lowercase letters, digits, dots, and hyphens
func IsValidUsername(username string) bool {
	return usernameRegex.MatchString(username)
}