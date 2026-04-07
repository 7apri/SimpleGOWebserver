package templates

import (
	"strings"
)

func (mgr *TemplateManager) Get(lang string, key TemplateKey) *TemplateWrapper {
	s := mgr.snapshot.Load()
	if s == nil {
		return nil
	}

	if l, ok := s.templates[lang]; ok {
		if val, ok := l[key]; ok {
			return val
		}
	}
	if l, ok := s.templates["en"]; ok {
		if val, ok := l[key]; ok {
			return val
		}
	}

	return nil
}

type metadata map[string]string

func getMetadata(content string) (metadata, string) {
	metadata := make(map[string]string)

	if !strings.HasPrefix(content, "---") {
		return metadata, content
	}

	endIndex := strings.Index(content[3:], "---")
	if endIndex == -1 {
		return metadata, content
	}

	endIndex += 3

	frontmatter := content[3:endIndex]
	remaining := content[endIndex+3:]

	lines := strings.SplitSeq(strings.ReplaceAll(frontmatter, "\r\n", "\n"), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		val = strings.Trim(val, `"'`)
		metadata[key] = val
	}

	return metadata, strings.TrimSpace(remaining)
}
