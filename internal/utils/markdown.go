package utils

import (
	"regexp"
)

func ExtractImageURLs(markdown string) []string {
	var urls []string
	// Regex for ![alt](url)
	imgMD := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	// Regex for <img src="url">
	imgHTML := regexp.MustCompile(`<img[^>]*src=["']([^"'>]+)["'][^>]*>`)

	for _, match := range imgMD.FindAllStringSubmatch(markdown, -1) {
		if len(match) > 1 {
			urls = append(urls, match[1])
		}
	}
	for _, match := range imgHTML.FindAllStringSubmatch(markdown, -1) {
		if len(match) > 1 {
			urls = append(urls, match[1])
		}
	}
	return urls
}
