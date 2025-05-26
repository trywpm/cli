package parser

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

const (
	phpFileExtension = ".php"
	cssFileExtension = ".css"
	maxHeaderBytes   = 8 * 1024
)

type PluginFileHeaders struct {
	Author          string
	Description     string
	License         string
	PluginURI       string
	Version         string
	RequiresPHP     string
	RequiresWP      string
	RequiresPlugins []string
	Tags            []string
}

var pluginFileHeaders = map[string]string{
	"Author":          "Author",
	"Description":     "Description",
	"License":         "License",
	"PluginURI":       "Plugin URI",
	"Version":         "Version",
	"RequiresPHP":     "Requires PHP",
	"RequiresWP":      "Requires at least",
	"RequiresPlugins": "Requires Plugins",
	"Tags":            "Tags",
}

type ThemeFileHeaders struct {
	Author      string
	Description string
	License     string
	ThemeURI    string
	Version     string
	RequiresWP  string
	RequiresPHP string
	Tags        []string
}

var themeFileHeaders = map[string]string{
	"Author":      "Author",
	"Description": "Description",
	"License":     "License",
	"ThemeURI":    "Theme URI",
	"Version":     "Version",
	"RequiresWP":  "Requires at least",
	"RequiresPHP": "Requires PHP",
	"Tags":        "Tags",
}

var headerCleanupRe = regexp.MustCompile(`\s*(?:\*\/|\?>).*`)

// getRawFileHeaders reads the first part of a file and extracts raw header values.
// Returns nil map if filePath has wrong extension or does not exist.
func getRawFileHeaders(filePath string, expectedExtension string, headerSpecs map[string]string) (map[string]string, error) {
	if len(filePath) < len(expectedExtension)+1 || filePath[len(filePath)-len(expectedExtension):] != expectedExtension {
		return nil, nil
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		// Mimic PHP's behavior of proceeding with empty data on most read failures.
		return processHeaderData("", headerSpecs)
	}
	defer file.Close()

	buffer := make([]byte, maxHeaderBytes)
	n, readErr := file.Read(buffer)
	if readErr != nil && readErr != io.EOF {
		// Process whatever was read, even on error.
		return processHeaderData(string(buffer[:n]), headerSpecs)
	}

	return processHeaderData(string(buffer[:n]), headerSpecs)
}

// processHeaderData extracts and cleans header values from a given string content.
func processHeaderData(fileDataString string, headerSpecs map[string]string) (map[string]string, error) {
	fileDataString = strings.ReplaceAll(fileDataString, "\r", "\n")
	extractedValues := make(map[string]string)

	for fieldName, headerTextInFile := range headerSpecs {
		pattern := fmt.Sprintf(`(?im)^(?:[ \t]*<\?php)?[ \t/*#@]*%s:(.*)$`, regexp.QuoteMeta(headerTextInFile))

		// Compile regex per iteration. For typical header counts, this is acceptable.
		// If performance critical with many headers, consider pre-compiling or a single complex regex.
		re, err := regexp.Compile(pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error compiling regex for field %s (header: '%s'): %v. Pattern: %s\n", fieldName, headerTextInFile, err, pattern)
			extractedValues[fieldName] = "" // Set to empty on regex compilation error
			continue
		}

		matches := re.FindStringSubmatch(fileDataString)
		var extractedVal string
		if len(matches) > 1 && matches[1] != "" {
			extractedVal = matches[1]
			extractedVal = headerCleanupRe.ReplaceAllString(extractedVal, "")
			extractedVal = strings.TrimSpace(extractedVal)
		} else {
			extractedVal = ""
		}
		extractedValues[fieldName] = extractedVal
	}

	return extractedValues, nil
}

// GetPluginHeaders retrieves headers for a WordPress plugin file.
func GetPluginHeaders(filePath string) (PluginFileHeaders, error) {
	rawHeaders, err := getRawFileHeaders(filePath, phpFileExtension, pluginFileHeaders)
	if err != nil {
		return PluginFileHeaders{}, err
	}
	if rawHeaders == nil {
		return PluginFileHeaders{}, nil
	}

	headers := PluginFileHeaders{}
	headers.Author = rawHeaders["Author"]
	headers.Description = rawHeaders["Description"]
	headers.License = rawHeaders["License"]
	headers.PluginURI = rawHeaders["PluginURI"]
	headers.Version = rawHeaders["Version"]
	headers.RequiresWP = rawHeaders["RequiresWP"]
	headers.RequiresPHP = rawHeaders["RequiresPHP"]
	headers.Tags = parseCommaSeparatedList(rawHeaders["Tags"])
	headers.RequiresPlugins = parseCommaSeparatedList(rawHeaders["RequiresPlugins"])

	return headers, nil
}

// GetThemeHeaders retrieves headers for a WordPress theme stylesheet.
func GetThemeHeaders(filePath string) (ThemeFileHeaders, error) {
	rawHeaders, err := getRawFileHeaders(filePath, cssFileExtension, themeFileHeaders)
	if err != nil {
		return ThemeFileHeaders{}, err
	}
	if rawHeaders == nil {
		return ThemeFileHeaders{}, nil
	}

	headers := ThemeFileHeaders{}
	headers.Author = rawHeaders["Author"]
	headers.Description = rawHeaders["Description"]
	headers.License = rawHeaders["License"]
	headers.ThemeURI = rawHeaders["ThemeURI"]
	headers.Version = rawHeaders["Version"]
	headers.RequiresWP = rawHeaders["RequiresWP"]
	headers.RequiresPHP = rawHeaders["RequiresPHP"]
	headers.Tags = parseCommaSeparatedList(rawHeaders["Tags"])

	return headers, nil
}
