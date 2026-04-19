package utils

import (
	"regexp"
	"strings"
)

func GenerateSlug(input string) string {
	// 1. Lowercase
	slug := strings.ToLower(input)
	
	// 2. Remove all non-alphanumeric (except spaces)
	reg, _ := regexp.Compile("[^a-z0-9 ]+")
	slug = reg.ReplaceAllString(slug, "")
	
	// 3. Trim space and replace spaces with hyphen
	slug = strings.TrimSpace(slug)
	slug = strings.ReplaceAll(slug, " ", "-")
	
	// 4. Remove consecutive hyphens
	regConc, _ := regexp.Compile("-+")
	slug = regConc.ReplaceAllString(slug, "-")
	
	return slug
}
