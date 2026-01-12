package prompt

import "strings"

// ParseModelResponse splits markdown-ish responses into text and fenced code blocks.
// It is deliberately conservative: if fences are unbalanced, it returns the raw text as one PartText.
func ParseModelResponse(raw string) ParsedResponse {
	out := ParsedResponse{Raw: raw}

	// Fast path
	if !strings.Contains(raw, "```") {
		out.Parts = append(out.Parts, ResponsePart{Type: PartText, Content: raw, StartPos: 0, EndPos: len(raw)})
		return out
	}

	parts := []ResponsePart{}
	i := 0
	for {
		start := strings.Index(raw[i:], "```")
		if start == -1 {
			// tail text
			if i < len(raw) {
				parts = append(parts, ResponsePart{Type: PartText, Content: raw[i:], StartPos: i, EndPos: len(raw)})
			}
			break
		}
		start += i

		// text before fence
		if start > i {
			parts = append(parts, ResponsePart{Type: PartText, Content: raw[i:start], StartPos: i, EndPos: start})
		}

		// parse fence header
		headerStart := start + 3
		headerEnd := strings.IndexByte(raw[headerStart:], '\n')
		if headerEnd == -1 {
			// malformed; bail out
			return ParsedResponse{Raw: raw, Parts: []ResponsePart{{Type: PartText, Content: raw, StartPos: 0, EndPos: len(raw)}}}
		}
		headerEnd += headerStart
		lang := strings.TrimSpace(raw[headerStart:headerEnd])

		// find closing fence
		codeStart := headerEnd + 1
		endFence := strings.Index(raw[codeStart:], "```")
		if endFence == -1 {
			// unbalanced; bail out
			return ParsedResponse{Raw: raw, Parts: []ResponsePart{{Type: PartText, Content: raw, StartPos: 0, EndPos: len(raw)}}}
		}
		endFence += codeStart
		code := raw[codeStart:endFence]

		parts = append(parts, ResponsePart{Type: PartCode, Lang: lang, Content: code, StartPos: start, EndPos: endFence + 3})
		i = endFence + 3
		if i >= len(raw) {
			break
		}
	}

	out.Parts = parts
	return out
}
