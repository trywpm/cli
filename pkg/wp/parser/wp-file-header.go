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
	Name            string
	PluginURI       string
	Version         string
	Description     string
	Author          string
	AuthorURI       string
	TextDomain      string
	DomainPath      string
	Network         bool
	RequiresPHP     string
	RequiresWP      string
	UpdateURI       string
	RequiresPlugins string
}

var pluginFileHeaders = map[string]string{
	"Name":            "Plugin Name",
	"PluginURI":       "Plugin URI",
	"Version":         "Version",
	"Description":     "Description",
	"Author":          "Author",
	"AuthorURI":       "Author URI",
	"TextDomain":      "Text Domain",
	"DomainPath":      "Domain Path",
	"Network":         "Network",
	"RequiresWP":      "Requires at least",
	"RequiresPHP":     "Requires PHP",
	"UpdateURI":       "Update URI",
	"RequiresPlugins": "Requires Plugins",
}

type ThemeFileHeaders struct {
	Name        string
	ThemeURI    string
	Description string
	Author      string
	AuthorURI   string
	Version     string
	Template    string
	Status      string
	Tags        string
	TextDomain  string
	DomainPath  string
	RequiresWP  string
	RequiresPHP string
	UpdateURI   string
}

var themeFileHeaders = map[string]string{
	"Name":        "Theme Name",
	"ThemeURI":    "Theme URI",
	"Description": "Description",
	"Author":      "Author",
	"AuthorURI":   "Author URI",
	"Version":     "Version",
	"Template":    "Template",
	"Status":      "Status",
	"Tags":        "Tags",
	"TextDomain":  "Text Domain",
	"DomainPath":  "Domain Path",
	"RequiresWP":  "Requires at least",
	"RequiresPHP": "Requires PHP",
	"UpdateURI":   "Update URI",
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
	headers.Name = rawHeaders["Name"]
	headers.PluginURI = rawHeaders["PluginURI"]
	headers.Version = rawHeaders["Version"]
	headers.Description = rawHeaders["Description"]
	headers.Author = rawHeaders["Author"]
	headers.AuthorURI = rawHeaders["AuthorURI"]
	headers.TextDomain = rawHeaders["TextDomain"]
	headers.DomainPath = rawHeaders["DomainPath"]
	networkVal := rawHeaders["Network"]
	if networkVal == "true" || networkVal == "1" {
		headers.Network = true
	}
	headers.RequiresWP = rawHeaders["RequiresWP"]
	headers.RequiresPHP = rawHeaders["RequiresPHP"]
	headers.UpdateURI = rawHeaders["UpdateURI"]
	headers.RequiresPlugins = rawHeaders["RequiresPlugins"]

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
	headers.Name = rawHeaders["Name"]
	headers.ThemeURI = rawHeaders["ThemeURI"]
	headers.Description = rawHeaders["Description"]
	headers.Author = rawHeaders["Author"]
	headers.AuthorURI = rawHeaders["AuthorURI"]
	headers.Version = rawHeaders["Version"]
	headers.Template = rawHeaders["Template"]
	headers.Status = rawHeaders["Status"]
	headers.Tags = rawHeaders["Tags"]
	headers.TextDomain = rawHeaders["TextDomain"]
	headers.DomainPath = rawHeaders["DomainPath"]
	headers.RequiresWP = rawHeaders["RequiresWP"]
	headers.RequiresPHP = rawHeaders["RequiresPHP"]
	headers.UpdateURI = rawHeaders["UpdateURI"]

	return headers, nil
}
