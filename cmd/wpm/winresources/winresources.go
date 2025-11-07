// Package winresources is used to embed Windows resources into wpm.exe.
//
// These resources are used to provide:
// * Version information
// * An icon
// * A Windows manifest declaring Windows version support
//
// The resource object files are generated when building with goversioninfo
// in scripts/build/binary and are located in cmd/wpm/winresources.
// This occurs automatically when you build against Windows OS.
package winresources
