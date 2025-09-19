package utils

import "strings"

// NormalizeString trims whitespace and converts to lowercase
// This is a common pattern used throughout the codebase for normalizing user input
func NormalizeString(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ValidateStringInSet checks if a normalized string is in the provided valid set
// Returns the normalized string and a boolean indicating if it's valid
func ValidateStringInSet(input string, validSet map[string]bool) (string, bool) {
	normalized := NormalizeString(input)
	return normalized, validSet[normalized]
}
