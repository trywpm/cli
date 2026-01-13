package manifest

import "wpm/pkg/pm/wpmjson/types"

// Dist struct to define the distribution metadata
type Dist struct {
	Digest       string `json:"digest"`
	TotalFiles   int64  `json:"totalFiles"`
	PackedSize   int64  `json:"packedSize"`
	UnpackedSize int64  `json:"unpackedSize"`
}

// PackageManifest struct to define the package manifest in registry
//
// It will act as the source of truth for publishing and installing packages.
type PackageManifest struct {
	Name            string                  `json:"name"`
	Description     string                  `json:"description,omitempty"`
	Type            types.PackageType       `json:"type"`
	Version         string                  `json:"version"`
	Bin             *types.Bin              `json:"bin,omitempty"`
	Requires        *types.Requires         `json:"requires,omitempty"`
	License         string                  `json:"license,omitempty"`
	Homepage        string                  `json:"homepage,omitempty"`
	Tags            []string                `json:"tags,omitempty"`
	Team            []string                `json:"team,omitempty"`
	Dependencies    *types.Dependencies     `json:"dependencies,omitempty"`
	DevDependencies *types.Dependencies     `json:"devDependencies,omitempty"`
	Tag             string                  `json:"tag"`
	Dist            Dist                    `json:"dist"`
	Wpm             string                  `json:"_wpm"`
	Visibility      types.PackageVisibility `json:"visibility"`
	Readme          string                  `json:"readme,omitempty"`
}
