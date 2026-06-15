package completion

import (
	"os"
	"sort"

	"github.com/spf13/cobra"

	"go.wpm.so/cli/pkg/pm/wpmjson"
	"go.wpm.so/cli/pkg/pm/wpmjson/types"
	"go.wpm.so/cli/pkg/pm/wpmlock"
)

// PackagesFromWpmJson offers completion for package names declared in
// wpm.json's dependencies and devDependencies.
func PackagesFromWpmJson() cobra.CompletionFunc {
	return Unique(func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		cfg, err := wpmjson.Read(cwd)
		if err != nil || cfg == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var names []string
		if cfg.Dependencies != nil {
			for name := range *cfg.Dependencies {
				names = append(names, name)
			}
		}
		if cfg.DevDependencies != nil {
			for name := range *cfg.DevDependencies {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		return names, cobra.ShellCompDirectiveNoFileComp
	})
}

// PackagesFromLockfile offers completion for package names recorded in
// wpm.lock.
func PackagesFromLockfile() cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		lock, err := wpmlock.Read(cwd)
		if err != nil || lock == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		names := make([]string, 0, len(lock.Packages))
		for name := range lock.Packages {
			names = append(names, name)
		}
		sort.Strings(names)
		return names, cobra.ShellCompDirectiveNoFileComp
	}
}

// PackageTypes offers completion for the closed set of valid package types.
func PackageTypes() cobra.CompletionFunc {
	return FromList(
		string(types.TypePlugin),
		string(types.TypeTheme),
	)
}

// PackageVisibility offers completion for the closed set of valid visibility.
func PackageVisibility() cobra.CompletionFunc {
	return FromList(
		string(types.VisibilityPublic),
		string(types.VisibilityPrivate),
	)
}

// DistTags suggests a non-exhaustive list of common distribution tags, used by
// `wpm publish --tag` and `wpm dist-tag add`.
func DistTags() cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"latest", "next", "beta", "alpha"}, cobra.ShellCompDirectiveNoFileComp
	}
}

// PackageLicenses suggests a non-exhaustive list of common SPDX licenses.
func PackageLicenses() cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"GPL-2.0-or-later", "GPL-3.0-or-later"}, cobra.ShellCompDirectiveNoFileComp
	}
}

// FileNames opts a flag or positional argument back into the shell's default
// file completion.
func FileNames() cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	}
}

// FromList offers completion for the given list of options.
func FromList(options ...string) cobra.CompletionFunc {
	return Unique(cobra.FixedCompletions(options, cobra.ShellCompDirectiveNoFileComp))
}

// Unique wraps a completion func and removes completion results that are
// already consumed (i.e., appear in "args").
//
// For example:
//
//	# initial completion: args is empty, so all results are shown
//	command <tab>
//	one two three
//
//	# "one" is already used so omitted
//	command one <tab>
//	two three
func Unique(fn cobra.CompletionFunc) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		all, dir := fn(cmd, args, toComplete)
		if len(all) == 0 || len(args) == 0 {
			return all, dir
		}

		alreadyCompleted := make(map[string]struct{}, len(args))
		for _, a := range args {
			alreadyCompleted[a] = struct{}{}
		}

		out := make([]string, 0, len(all))
		for _, c := range all {
			if _, ok := alreadyCompleted[c]; !ok {
				out = append(out, c)
			}
		}

		return out, dir
	}
}
