package parser

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Constants for known section keys.
const (
	sectionDescription              = "description"
	sectionInstallation             = "installation"
	sectionFAQ                      = "faq"
	sectionFrequentlyAskedQuestions = "frequently_asked_questions"
	sectionScreenshots              = "screenshots"
	sectionScreenshot               = "screenshot"
	sectionChangelog                = "changelog"
	sectionChangeLog                = "change_log"
	sectionUpgradeNotice            = "upgrade_notice"
)

var (
	screenshotLineRegex = regexp.MustCompile(`^(\d+)\.\s+(.+)$`)
)

type ReadmeParser struct {
	Name            string
	MetaDescription string
	Contributors    []string
	Tags            []string
	Requires        string
	Tested          string
	RequiresPHP     string
	StableTag       string
	License         string
	LicenseURI      string
	DonateLink      string

	Sections      map[string]string
	Screenshots   map[int]string
	FAQ           map[string]string
	UpgradeNotice map[string]string
}

func NewReadmeParser() *ReadmeParser {
	return &ReadmeParser{
		Sections:      make(map[string]string),
		Screenshots:   make(map[int]string),
		FAQ:           make(map[string]string),
		UpgradeNotice: make(map[string]string),
	}
}

// Parse reads and parses the readme.txt content.
func (p *ReadmeParser) Parse(content string) {
	normalizedContent := strings.ReplaceAll(content, "\r\n", "\n")
	normalizedContent = strings.ReplaceAll(normalizedContent, "\r", "\n")
	lines := strings.Split(normalizedContent, "\n")

	lines = p.stripBOM(lines)

	lineIndex := 0

	// Parse plugin name from === Plugin Name === OR # Plugin Name
	lineIndex = p.skipEmptyLines(lines, lineIndex)
	if lineIndex < len(lines) {
		trimmedLine := strings.TrimSpace(lines[lineIndex])

		// Check Classic (===)
		if strings.HasPrefix(trimmedLine, "===") && strings.HasSuffix(trimmedLine, "===") && len(trimmedLine) > 6 {
			p.Name = p.parsePluginName(trimmedLine, "===")
			lineIndex++
		} else if strings.HasPrefix(trimmedLine, "# ") {
			p.Name = p.parsePluginName(trimmedLine, "#")
			lineIndex++
		}
	}

	lineIndex = p.parseHeaders(lines, lineIndex)
	lineIndex = p.parseMetaDescription(lines, lineIndex)
	p.parseSections(lines, lineIndex)
	p.processSpecialSections()
}

func (p *ReadmeParser) stripBOM(lines []string) []string {
	if len(lines) > 0 && strings.HasPrefix(lines[0], "\xEF\xBB\xBF") {
		lines[0] = strings.TrimPrefix(lines[0], "\xEF\xBB\xBF")
	}
	return lines
}

// parsePluginName extracts the plugin name stripping markers.
func (p *ReadmeParser) parsePluginName(line, marker string) string {
	name := strings.TrimPrefix(line, marker)
	name = strings.TrimSuffix(name, marker)
	return strings.TrimSpace(name)
}

func (p *ReadmeParser) skipEmptyLines(lines []string, start int) int {
	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			return i
		}
	}
	return len(lines)
}

func parseCommaSeparatedList(value string) []string {
	items := strings.Split(value, ",")
	var result []string
	for _, item := range items {
		trimmedItem := strings.TrimSpace(item)
		if trimmedItem != "" {
			result = append(result, trimmedItem)
		}
	}
	return result
}

func (p *ReadmeParser) parseHeaders(lines []string, start int) int {
	// Map of accepted header strings (lowercase) to a canonical key.
	validHeaders := map[string]string{
		"tested up to":      "tested",
		"tested":            "tested",
		"requires at least": "requires",
		"requires":          "requires",
		"requires php":      "requires_php",
		"tags":              "tags",
		"contributors":      "contributors",
		"donate link":       "donate_link",
		"stable tag":        "stable_tag",
		"license":           "license",
		"license uri":       "license_uri",
	}

	i := start
	for i < len(lines) {
		lineContent := lines[i]
		trimmedLine := strings.TrimSpace(lineContent)

		if trimmedLine == "" {
			i++
			continue
		}

		// Headers end when a section starts.
		// Classic: starts with "=="
		// Markdown: starts with "##"
		if strings.HasPrefix(trimmedLine, "==") || strings.HasPrefix(trimmedLine, "##") {
			break
		}

		if strings.Contains(lineContent, ":") {
			parts := strings.SplitN(lineContent, ":", 2)
			// Ensure key part is not empty after trimming
			if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" {
				key := strings.ToLower(strings.TrimSpace(parts[0]))
				value := strings.TrimSpace(parts[1])

				if canonicalKey, ok := validHeaders[key]; ok {
					switch canonicalKey {
					case "contributors":
						p.Contributors = parseCommaSeparatedList(value)
					case "tags":
						p.Tags = parseCommaSeparatedList(value)
					case "requires":
						p.Requires = value
					case "tested":
						p.Tested = value
					case "requires_php":
						p.RequiresPHP = value
					case "stable_tag":
						p.StableTag = value
					case "license":
						p.License = value
					case "license_uri":
						p.LicenseURI = value
					case "donate_link":
						p.DonateLink = value
					}
				}
			} else {
				// Malformed header (e.g., "Key:" with no value, or ": value"), end of headers.
				break
			}
		} else {
			// Line does not contain ':' and is not empty, so it's not a header.
			break
		}
		i++
	}
	return i
}

func (p *ReadmeParser) parseMetaDescription(lines []string, start int) int {
	i := p.skipEmptyLines(lines, start)

	if i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "==") && !strings.HasPrefix(line, "##") {
			p.MetaDescription = line
			i++
		}
	}
	return i
}

func (p *ReadmeParser) parseSections(lines []string, start int) {
	var currentSectionKey string
	var currentContent strings.Builder

	saveSection := func() {
		if currentSectionKey != "" {
			// Trim only leading/trailing newlines from the collected content block.
			content := strings.Trim(currentContent.String(), "\n")
			p.Sections[currentSectionKey] = content
			currentContent.Reset()
		}
	}

	for i := start; i < len(lines); i++ {
		line := lines[i]
		trimmedLine := strings.TrimSpace(line)

		isClassicSection := strings.HasPrefix(trimmedLine, "==") && strings.HasSuffix(trimmedLine, "==") &&
			!strings.HasPrefix(trimmedLine, "===") && len(trimmedLine) >= 4

		isMarkdownSection := strings.HasPrefix(trimmedLine, "##")

		if isClassicSection || isMarkdownSection {
			saveSection()
			var sectionTitle string

			if isClassicSection {
				sectionTitle = strings.TrimSpace(trimmedLine[2 : len(trimmedLine)-2])
			} else {
				sectionTitle = strings.TrimPrefix(trimmedLine, "##")
				sectionTitle = strings.TrimSuffix(sectionTitle, "##")
				sectionTitle = strings.TrimSpace(sectionTitle)
			}

			currentSectionKey = strings.ToLower(strings.ReplaceAll(sectionTitle, " ", "_"))
		} else {
			if currentSectionKey != "" {
				currentContent.WriteString(line + "\n")
			}
		}
	}
	saveSection()
}

// getSectionContent retrieves section content, checking primary and alternative keys.
func (p *ReadmeParser) getSectionContent(primaryKey string, alternateKeys ...string) (content string, keyUsed string, found bool) {
	if content, ok := p.Sections[primaryKey]; ok {
		return content, primaryKey, true
	}
	for _, altKey := range alternateKeys {
		if content, ok := p.Sections[altKey]; ok {
			return content, altKey, true
		}
	}
	return "", "", false
}

func (p *ReadmeParser) processSpecialSections() {
	if faqContent, _, ok := p.getSectionContent(sectionFrequentlyAskedQuestions, sectionFAQ); ok && faqContent != "" {
		p.FAQ = p.parseBlockStyleItems(faqContent)
	}

	if screenshotsContent, _, ok := p.getSectionContent(sectionScreenshots, sectionScreenshot); ok && screenshotsContent != "" {
		p.parseScreenshots(screenshotsContent)
	}

	if upgradeContent, _, ok := p.getSectionContent(sectionUpgradeNotice); ok && upgradeContent != "" {
		p.UpgradeNotice = p.parseBlockStyleItems(upgradeContent)
	}
}

// parseBlockStyleItems parses sections like FAQ or UpgradeNotice.
// Handles Classic "= Item Title =" and Markdown "### Item Title".
func (p *ReadmeParser) parseBlockStyleItems(content string) map[string]string {
	itemsMap := make(map[string]string)
	lines := strings.Split(content, "\n")
	var currentItemTitle string
	var currentItemContent strings.Builder

	saveItem := func() {
		if currentItemTitle != "" {
			itemsMap[currentItemTitle] = strings.Trim(currentItemContent.String(), "\n")
			currentItemContent.Reset()
		}
	}

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		isClassicItem := strings.HasPrefix(trimmedLine, "=") && strings.HasSuffix(trimmedLine, "=") &&
			!strings.HasPrefix(trimmedLine, "==") && len(trimmedLine) >= 3

		isMarkdownItem := strings.HasPrefix(trimmedLine, "###")

		if isClassicItem || isMarkdownItem {
			saveItem()
			if isClassicItem {
				currentItemTitle = strings.TrimSpace(trimmedLine[1 : len(trimmedLine)-1])
			} else {
				currentItemTitle = strings.TrimPrefix(trimmedLine, "###")
				currentItemTitle = strings.TrimSuffix(currentItemTitle, "###")
				currentItemTitle = strings.TrimSpace(currentItemTitle)
			}
		} else if currentItemTitle != "" { // Only append if we have an active item title
			currentItemContent.WriteString(line + "\n")
		}
	}
	saveItem()
	return itemsMap
}

func (p *ReadmeParser) parseScreenshots(content string) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		if matches := screenshotLineRegex.FindStringSubmatch(trimmedLine); len(matches) == 3 {
			num, err := strconv.Atoi(matches[1])
			if err == nil && num > 0 {
				p.Screenshots[num] = strings.TrimSpace(matches[2])
			}
		}
	}
}

func (p *ReadmeParser) convertSubsections(content string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "=") && strings.HasSuffix(trimmedLine, "=") &&
			!strings.HasPrefix(trimmedLine, "==") && len(trimmedLine) >= 3 {
			subsectionTitle := strings.TrimSpace(trimmedLine[1 : len(trimmedLine)-1])
			result.WriteString("### " + subsectionTitle)
		} else {
			result.WriteString(line)
		}
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	return result.String()
}

type sectionToMarkdownConfig struct {
	markdownTitle string
	primaryKey    string
	alternateKeys []string
	render        func(parser *ReadmeParser, rawSectionContent string, md *strings.Builder) (handled bool)
}

func renderFAQMarkdown(parser *ReadmeParser, _ string, md *strings.Builder) bool {
	if len(parser.FAQ) > 0 {
		questions := make([]string, 0, len(parser.FAQ))
		for q := range parser.FAQ {
			questions = append(questions, q)
		}
		// Sort carefully to keep things somewhat stable
		sort.Strings(questions)
		for _, question := range questions {
			md.WriteString("### " + question + "\n\n")
			md.WriteString(parser.FAQ[question] + "\n\n")
		}
		return true
	}
	return false
}

func renderScreenshotsMarkdown(parser *ReadmeParser, _ string, md *strings.Builder) bool {
	if len(parser.Screenshots) > 0 {
		keys := make([]int, 0, len(parser.Screenshots))
		for k := range parser.Screenshots {
			keys = append(keys, k)
		}
		sort.Ints(keys)

		for _, k := range keys {
			title := parser.Screenshots[k]
			safeAltTitle := strings.ReplaceAll(title, "\"", "&quot;")

			md.WriteString("### " + title + "\n")
			fmt.Fprintf(md, `<img id="screenshot-%d" src="/screenshot-%d.png" alt="%s" />`+"\n\n", k, k, safeAltTitle)
		}
		return true
	}
	return false
}

func renderUpgradeNoticeMarkdown(parser *ReadmeParser, _ string, md *strings.Builder) bool {
	if len(parser.UpgradeNotice) > 0 {
		versions := make([]string, 0, len(parser.UpgradeNotice))
		for v := range parser.UpgradeNotice {
			versions = append(versions, v)
		}
		sort.Strings(versions)
		for _, version := range versions {
			md.WriteString("### " + version + "\n\n")
			md.WriteString(parser.UpgradeNotice[version] + "\n\n")
		}
		return true
	}
	return false
}

func (p *ReadmeParser) ToMarkdown() string {
	var md strings.Builder

	standardSectionConfigs := []sectionToMarkdownConfig{
		{markdownTitle: "## Description", primaryKey: sectionDescription},
		{markdownTitle: "## Installation", primaryKey: sectionInstallation},
		{markdownTitle: "## Frequently Asked Questions", primaryKey: sectionFrequentlyAskedQuestions, alternateKeys: []string{sectionFAQ}, render: renderFAQMarkdown},
		{markdownTitle: "## Screenshots", primaryKey: sectionScreenshots, alternateKeys: []string{sectionScreenshot}, render: renderScreenshotsMarkdown},
		{markdownTitle: "## Changelog", primaryKey: sectionChangelog, alternateKeys: []string{sectionChangeLog}},
		{markdownTitle: "## Upgrade Notice", primaryKey: sectionUpgradeNotice, render: renderUpgradeNoticeMarkdown},
	}

	renderedSectionKeys := make(map[string]bool)

	for _, config := range standardSectionConfigs {
		content, keyUsed, found := p.getSectionContent(config.primaryKey, config.alternateKeys...)

		if !found {
			continue
		}
		renderedSectionKeys[keyUsed] = true
		if content == "" {
			continue
		}

		md.WriteString(config.markdownTitle + "\n\n")

		handledByCustomRender := false
		if config.render != nil {
			handledByCustomRender = config.render(p, content, &md)
		}
		if !handledByCustomRender {
			md.WriteString(p.convertSubsections(content) + "\n\n")
		}
	}

	// Process "Other sections"
	otherSectionKeys := make([]string, 0, len(p.Sections))
	for key := range p.Sections {
		if !renderedSectionKeys[key] {
			otherSectionKeys = append(otherSectionKeys, key)
		}
	}
	sort.Strings(otherSectionKeys)

	titleCaser := cases.Title(language.English)
	for _, sectionKey := range otherSectionKeys {
		content := p.Sections[sectionKey]
		if content == "" {
			continue
		}
		title := titleCaser.String(strings.ReplaceAll(sectionKey, "_", " "))
		md.WriteString("## " + title + "\n\n")
		md.WriteString(p.convertSubsections(content) + "\n\n")
	}

	return strings.TrimSpace(md.String())
}

func (p *ReadmeParser) GetMetadata() map[string]any {
	return map[string]any{
		"name":             p.Name,
		"meta_description": p.MetaDescription,
		"contributors":     p.Contributors,
		"tags":             p.Tags,
		"requires":         p.Requires,
		"tested":           p.Tested,
		"requires_php":     p.RequiresPHP,
		"stable_tag":       p.StableTag,
		"license":          p.License,
		"license_uri":      p.LicenseURI,
		"donate_link":      p.DonateLink,
	}
}
