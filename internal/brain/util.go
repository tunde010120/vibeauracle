package brain

import (
	"strings"
	"regexp"
)

// ShortenModelName cleans up long model identifiers for UI display.
func ShortenModelName(name string) string {
	original := name

	// 1. Remove common prefixes and suffixes
	name = strings.TrimPrefix(name, "azure-sdk-for-go/")
	name = strings.TrimSuffix(name, ":latest")

	// 2. Handle Meta Llama models
	if strings.Contains(name, "Meta-Llama") || strings.Contains(name, "Llama") {
		// Example: Meta-Llama-3.1-405B-Instruct -> Llama 3.1 405B
		// Example: meta-llama-3.1-8b-instruct -> Llama 3.1 8B
		re := regexp.MustCompile(`(?i)(?:Meta-)?Llama-?([\d\.]+)-?(\d+[BM])-?(?:Instruct|Chat)?`)
		matches := re.FindStringSubmatch(name)
		if len(matches) >= 3 {
			return "Llama " + matches[1] + " " + strings.ToUpper(matches[2])
		}
	}

	// 3. Handle OpenAI models with dates
	if strings.HasPrefix(name, "gpt-") {
		// Example: gpt-4o-2024-05-13 -> GPT-4o
		// Example: gpt-3.5-turbo-0125 -> GPT-3.5 Turbo
		re := regexp.MustCompile(`(?i)gpt-([\d\.a-z\-]+)(?:-\d{4}-\d{2}-\d{2}|-\d{4})?`)
		matches := re.FindStringSubmatch(name)
		if len(matches) >= 2 {
			res := strings.ToUpper(matches[1])
			res = strings.ReplaceAll(res, "TURBO", "Turbo")
			return "GPT-" + res
		}
	}

	// 4. Handle Phi models
	if strings.Contains(name, "Phi-") {
		re := regexp.MustCompile(`(?i)Phi-([^-]+)`)
		matches := re.FindStringSubmatch(name)
		if len(matches) >= 2 {
			return "Phi-" + matches[1]
		}
	}

	// 5. Handle Mistral/Mixtral
	if strings.Contains(name, "Mistral-") || strings.Contains(name, "Mixtral-") {
		re := regexp.MustCompile(`(?i)(Mi[sx]tral-[^-]+)`)
		matches := re.FindStringSubmatch(name)
		if len(matches) >= 2 {
			return matches[1]
		}
	}

	// 6. Generic cleanup: replace hyphens with spaces and capitalize
	if len(name) > 20 {
		return original // If it's too complex and we didn't match, keep it original
	}

	return name
}
