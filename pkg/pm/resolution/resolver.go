package resolution

import (
	"context"
	"fmt"
	"wpm/pkg/pm/registry"
	"wpm/pkg/pm/wpmjson"
	"wpm/pkg/pm/wpmlock"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Node struct {
	Name         string
	Version      string
	Type         wpmjson.PackageType
	Resolved     string                // Tarball URL
	Digest       string                // Sha256 digest of the tarball
	Bin          *wpmjson.Bin          `json:"bin,omitempty"`
	Dependencies *wpmjson.Dependencies `json:"dependencies,omitempty"`
}

type dependencyRequest struct {
	name      string
	version   string
	requestor string
}

type Resolver struct {
	rootConfig *wpmjson.Config
	lockfile   *wpmlock.Lockfile
	client     registry.Client
	runtimeWp  string
	runtimePhp string
}

func NewResolver(rootConfig *wpmjson.Config, lockfile *wpmlock.Lockfile, client registry.Client, runtimeWp, runtimePhp string) *Resolver {
	return &Resolver{
		rootConfig: rootConfig,
		lockfile:   lockfile,
		client:     client,
		runtimeWp:  runtimeWp,
		runtimePhp: runtimePhp,
	}
}

type fetchResult struct {
	req      dependencyRequest
	manifest *wpmjson.PackageManifest
	err      error
}

func (r *Resolver) Resolve(ctx context.Context) (map[string]Node, error) {
	resolved := make(map[string]Node)
	queue := make([]dependencyRequest, 0)

	// Seed the queue with root dependencies
	if r.rootConfig.Dependencies != nil {
		for name, version := range *r.rootConfig.Dependencies {
			queue = append(queue, dependencyRequest{
				name:      name,
				version:   version,
				requestor: "<root>",
			})
		}
	}
	if r.rootConfig.DevDependencies != nil {
		for name, version := range *r.rootConfig.DevDependencies {
			queue = append(queue, dependencyRequest{
				name:      name,
				version:   version,
				requestor: "<root>",
			})
		}
	}

	for len(queue) > 0 {
		uniqueRequests := make(map[string]dependencyRequest)
		for _, req := range queue {
			// If already resolved with the same version, skip
			if exists, ok := resolved[req.name]; ok && exists.Version == req.version {
				continue
			}

			uniqueRequests[req.name+"@"+req.version] = req
		}

		queue = nil // Clear queue for next iteration

		results := make(chan fetchResult, len(uniqueRequests))
		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(16) // Limit concurrent fetches

		for _, req := range uniqueRequests {
			g.Go(func() error {
				manifest, err := r.fetchMetadata(ctx, req.name, req.version)
				results <- fetchResult{req: req, manifest: manifest, err: err}
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return nil, err
		}
		close(results)

		for res := range results {
			if res.err != nil {
				return nil, fmt.Errorf("failed to fetch metadata for %s@%s required by %s: %w", res.req.name, res.req.version, res.req.requestor, res.err)
			}

			// --- Conflict Resolution ---
			if existing, ok := resolved[res.req.name]; ok {
				if existing.Version == res.req.version {
					continue // Same version already resolved
				}

				if err := r.resolveConflict(res.req, existing); err != nil {
					return nil, err
				}

				continue
			}

			// -- Runtime Compatibility Check ---
			if err := r.checkRuntimeCompatibility(res.manifest); err != nil {
				return nil, fmt.Errorf(
					"package %s@%s incompatible:\n"+
						"  %w",
					res.req.name, res.req.version, err,
				)
			}

			// Add to resolved map
			resolved[res.req.name] = Node{
				Name:         res.manifest.Name,
				Version:      res.manifest.Version,
				Type:         res.manifest.Type,
				Resolved:     "/" + res.manifest.Name + "/" + res.manifest.Version + ".tar.zst",
				Digest:       res.manifest.Dist.Digest,
				Bin:          res.manifest.Bin,
				Dependencies: res.manifest.Dependencies,
			}

			// Enqueue child dependencies
			if res.manifest.Dependencies != nil {
				for name, version := range *res.manifest.Dependencies {
					queue = append(queue, dependencyRequest{name, version, res.req.name})
				}
			}
		}
	}

	return resolved, nil
}

func (r *Resolver) resolveConflict(req dependencyRequest, existing Node) error {
	// Check if root wpm.json has a direct dependency on this package
	rootVersion := ""
	if r.rootConfig.Dependencies != nil {
		if v, ok := (*r.rootConfig.Dependencies)[req.name]; ok {
			rootVersion = v
		}
	}
	if rootVersion == "" && r.rootConfig.DevDependencies != nil {
		if v, ok := (*r.rootConfig.DevDependencies)[req.name]; ok {
			rootVersion = v
		}
	}

	if rootVersion != "" {
		// The 'existing' node in the map MUST match 'rootVersion' because root dependencies are processed first.
		//
		// This indicates a bug in the resolver logic if this invariant is violated.
		if existing.Version != rootVersion {
			return fmt.Errorf("invariant violation: existing version %s does not match root version %s for package %s", existing.Version, rootVersion, req.name)
		}

		reqV, err := semver.NewVersion(req.version)
		if err != nil {
			// If not valid semver (e.g. dist-tag 'latest'), we can't strictly compare, assume conflict.
			return fmt.Errorf("version conflict for %s: root pins %s, %s asks for %s (non-semver)", req.name, rootVersion, req.requestor, req.version)
		}

		rootV, err := semver.NewVersion(rootVersion)
		if err != nil {
			// If root version is not valid semver, we can't strictly compare, assume conflict.
			return fmt.Errorf("version conflict for %s: root pins %s (non-semver), %s asks for %s", req.name, rootVersion, req.requestor, req.version)
		}

		if reqV.GreaterThan(rootV) {
			//nolint:staticcheck
			return fmt.Errorf(
				"version downgrade detected for %s:\n"+
					"  currently resolved: %s\n"+
					"  %s requires: %s\n"+
					"Action: Upgrade %s in your wpm.json to %s or higher.",
				req.name, rootVersion, req.requestor, req.version, req.name, req.version,
			)
		}

		// If we reach here, the root version satisfies the request, so we can ignore the conflict.
		return nil
	}

	// Unresolvable conflict, user must intervene.
	return fmt.Errorf( //nolint:staticcheck
		"Dependency version conflict for package %s:\n"+
			"  currently resolved: %s\n"+
			"  %s requires: %s\n"+
			`Action: Add "%s": "%s" (or %s) to the root wpm.json to force a resolution.`,
		req.name,
		existing.Version,
		req.requestor,
		req.version,
		req.name,
		req.version,
		existing.Version,
	)
}

func (r *Resolver) checkRuntimeCompatibility(manifest *wpmjson.PackageManifest) error {
	if manifest == nil {
		return errors.New("manifest is nil")
	}

	// If runtime strict mode is disabled, skip checks.
	if r.rootConfig.Config != nil && !*r.rootConfig.Config.RuntimeStrict {
		return nil
	}

	// If manifest has no requirements, skip
	if manifest.Requires == nil {
		return nil
	}

	// Check WordPress runtime compatibility.
	if manifest.Requires.WP != "" && r.runtimeWp != "" {
		c, err := semver.NewConstraint(manifest.Requires.WP)
		if err != nil {
			return errors.Wrap(err, "Invalid wp requirement in package")
		}

		v, err := semver.NewVersion(r.runtimeWp)
		if err != nil {
			return errors.Wrap(err, "Invalid runtime-wp version provided")
		}

		if !c.Check(v) {
			return fmt.Errorf("requires WordPress %s, but runtime is %s", manifest.Requires.WP, r.runtimeWp)
		}
	}

	// Check PHP
	if manifest.Requires.PHP != "" && r.runtimePhp != "" {
		c, err := semver.NewConstraint(manifest.Requires.PHP)
		if err != nil {
			return errors.Wrap(err, "Invalid php requirement in package")
		}

		v, err := semver.NewVersion(r.runtimePhp)
		if err != nil {
			return errors.Wrap(err, "Invalid runtime-php version provided")
		}

		if !c.Check(v) {
			return fmt.Errorf("requires PHP %s, but runtime is %s", manifest.Requires.PHP, r.runtimePhp)
		}
	}

	return nil
}

func (r *Resolver) fetchMetadata(ctx context.Context, name, version string) (*wpmjson.PackageManifest, error) {
	// Try to resolve the manifest from lockfile first
	if r.lockfile != nil && r.lockfile.Packages != nil {
		if lockPkg, ok := r.lockfile.Packages[name]; ok {
			if lockPkg.Version == version {
				return &wpmjson.PackageManifest{
					Name:         name,
					Version:      lockPkg.Version,
					Type:         lockPkg.Type,
					Bin:          lockPkg.Bin,
					Dependencies: lockPkg.Dependencies,
					Dist: wpmjson.Dist{
						Digest: lockPkg.Digest,
					},
				}, nil
			}
		}
	}

	return r.client.GetPackageManifest(ctx, name, version)
}
