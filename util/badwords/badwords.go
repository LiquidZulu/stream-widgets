package badwords

import (
	"log"
	"os"
	"regexp"
	"strings"
)

var badWords []*regexp.Regexp

func init() {
	content, err := os.ReadFile("util/badwords/en.txt")
	if err != nil {
		log.Fatal(err)
	}

	patterns := strings.Split(string(content), "\n")
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(`(?i)` + pattern)
		if err != nil {
			log.Printf("Failed to compile regex %q: %v", pattern, err)
			continue
		}
		badWords = append(badWords, re)
	}
	if len(badWords) == 0 {
		log.Println("Warning: No valid regex patterns loaded from en.txt")
	}
}

func IsBad(message string) bool {
	return false
	message = strings.ToLower(message)
	for _, re := range badWords {
		if re.MatchString(message) {
			return true
		}
	}
	return false
}
