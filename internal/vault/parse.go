package vault

import (
	"regexp"
	"strings"
)

var (
	emailPattern   = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	wikiPattern    = regexp.MustCompile(`\[\[([^\]\|#]+)(?:#[^\]\|]*)?(?:\|([^\]]*))?\]\]`)
	imagePattern   = regexp.MustCompile(`!\[\[([^\]\|#]+)(?:#[^\]\|]*)?(?:\|[^\]]*)?\]\]`)
	headingPattern = regexp.MustCompile(`^(#{2,6})\s+(.+?)\s*$`)
	headingMarks   = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	calloutPattern = regexp.MustCompile(`(?m)^>\s*\[![^\]]+\][^\n]*\n?`)
	quotePrefix    = regexp.MustCompile(`(?m)^>\s?`)
	hrPattern      = regexp.MustCompile(`(?m)^-{3,}\s*$`)
	imageExt       = regexp.MustCompile(`(?i)\.(png|jpe?g|gif|webp|svg|bmp|avif)$`)
)

func parseTeacher(name, rel string, body []byte) *Teacher {
	t := &Teacher{
		Name: strings.TrimSpace(name),
		Path: rel,
		Body: string(body),
	}
	if m := emailPattern.FindString(t.Body); m != "" {
		t.Email = m
	}
	t.EmailUnconfirmed = strings.Contains(withoutDiacritics(strings.ToLower(t.Body)), "не подтвержден")
	t.Photo = firstImage(t.Body)
	t.Subjects = subjectLinks(t.Body)
	return t
}

func parseSubject(name, rel string, body []byte) *Subject {
	s := &Subject{
		Name: strings.TrimSpace(name),
		Path: rel,
		Body: string(body),
	}

	var (
		sections    []Section
		current     *Section
		preamble    []string
		seenHeading bool
	)

	for _, line := range strings.Split(s.Body, "\n") {
		trimmed := strings.TrimSpace(line)
		heading := headingPattern.FindStringSubmatch(trimmed)

		if heading != nil {
			seenHeading = true
			title := strings.TrimSpace(heading[2])
			normalized := withoutDiacritics(strings.ToLower(title))

			if strings.Contains(normalized, "кто ведет") {
				s.Teacher = teacherFromValue(afterColon(title))
				current = nil
				continue
			}

			sections = append(sections, Section{Title: title})
			current = &sections[len(sections)-1]
			continue
		}

		if !seenHeading {
			preamble = append(preamble, line)
			continue
		}
		if current != nil {
			if current.Text != "" {
				current.Text += "\n"
			}
			current.Text += line
		}
	}

	// Text before the first heading (rare) is kept under a generic title.
	if text := strings.TrimSpace(strings.Join(preamble, "\n")); text != "" {
		sections = append([]Section{{Title: "Описание", Text: text}}, sections...)
	}

	for i := range sections {
		sections[i].Text = strings.TrimSpace(sections[i].Text)
	}
	s.Sections = sections
	return s
}

func parseNote(name, rel, kind string, body []byte) *Note {
	return &Note{
		Title: strings.TrimSpace(name),
		Path:  rel,
		Kind:  kind,
		Body:  string(body),
	}
}

// subjectLinks returns subjects listed under the "Что ведет" heading.
// Falls back to every non-image wikilink in the note.
func subjectLinks(body string) []string {
	var (
		links []string
		seen  = make(map[string]bool)
	)

	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || imageExt.MatchString(raw) {
			return
		}
		key := strings.ToLower(raw)
		if seen[key] {
			return
		}
		seen[key] = true
		links = append(links, raw)
	}

	inSection := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if headingPattern.MatchString(trimmed) {
			inSection = strings.Contains(withoutDiacritics(strings.ToLower(trimmed)), "что ведет")
			continue
		}
		if !inSection {
			continue
		}
		if !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "*") {
			continue
		}

		item := strings.TrimLeft(trimmed, "-* \t")
		matches := wikiPattern.FindAllStringSubmatch(item, -1)
		if len(matches) == 0 {
			add(PlainText(item))
		}
		for _, m := range matches {
			add(m[1])
		}
	}

	if len(links) > 0 {
		return links
	}

	// No section found: use all non-image links, e.g. when the layout changes.
	for _, m := range wikiPattern.FindAllStringSubmatch(imagePattern.ReplaceAllString(body, ""), -1) {
		add(m[1])
	}
	return links
}

func firstImage(body string) string {
	if m := imagePattern.FindStringSubmatch(body); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func teacherFromValue(value string) string {
	value = strings.TrimSpace(value)
	if m := wikiPattern.FindStringSubmatch(value); m != nil {
		return strings.TrimSpace(m[1])
	}
	return PlainText(value)
}

func afterColon(s string) string {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(s[idx+1:])
}

// PlainText strips the light markdown used by the vault: images, wikilinks,
// heading marks and emphasis markers.
func PlainText(s string) string {
	s = imagePattern.ReplaceAllString(s, "")
	s = wikiPattern.ReplaceAllStringFunc(s, func(m string) string {
		sub := wikiPattern.FindStringSubmatch(m)
		if len(sub) > 2 && strings.TrimSpace(sub[2]) != "" {
			return sub[2]
		}
		return sub[1]
	})
	s = headingMarks.ReplaceAllString(s, "")
	s = calloutPattern.ReplaceAllString(s, "")
	s = quotePrefix.ReplaceAllString(s, "")
	s = hrPattern.ReplaceAllString(s, "")
	s = strings.NewReplacer("**", "", "__", "", "`", "").Replace(s)
	return strings.TrimSpace(s)
}

func withoutDiacritics(s string) string {
	return strings.ReplaceAll(s, "ё", "е")
}
