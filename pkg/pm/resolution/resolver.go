package resolution

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/sync/errgroup"

	"go.wpm.so/cli/pkg/pm/registry"
	"go.wpm.so/cli/pkg/pm/wpmjson"
	"go.wpm.so/cli/pkg/pm/wpmjson/manifest"
	"go.wpm.so/cli/pkg/pm/wpmjson/types"
	"go.wpm.so/cli/pkg/pm/wpmlock"
)

type Node struct {
	Name         string
	Version      string
	Type         types.PackageType
	Signatures   []manifest.Signature
	Digest       string              // Sha256 digest of the tarball
	Bin          *types.Bin          `json:"bin,omitempty"`
	Dependencies *types.Dependencies `json:"dependencies,omitempty"`
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
}

func New(rootConfig *wpmjson.Config, lockfile *wpmlock.Lockfile, client registry.Client) *Resolver {
	return &Resolver{
		rootConfig: rootConfig,
		lockfile:   lockfile,
		client:     client,
	}
}

type ProgressReporter interface {
	StartProgressIndicator(w io.Writer)
	StopProgressIndicator()
	Stream(w io.Writer, msg string)
}

type fetchResult struct {
	req      dependencyRequest
	manifest *manifest.Package
}

func (r *Resolver) Resolve(ctx context.Context, progress ProgressReporter, w io.Writer) (map[string]Node, error) {
	resolved := make(map[string]Node)
	queue := r.seedQueue()

	progress.StartProgressIndicator(w)
	defer func() {
		progress.Stream(w, "")
		progress.StopProgressIndicator()
	}()

	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		uniqueRequests := dedupeRequests(queue, resolved)
		queue = nil // clear queue for next iteration

		results, err := r.fetchAll(ctx, uniqueRequests, progress, w)
		if err != nil {
			return nil, err
		}

		for _, res := range results {
			children, err := r.applyResult(res, resolved)
			if err != nil {
				return nil, err
			}
			queue = append(queue, children...)
		}
	}

	return resolved, nil
}

func (r *Resolver) seedQueue() []dependencyRequest {
	var queue []dependencyRequest
	if r.rootConfig.Dependencies != nil {
		for name, version := range *r.rootConfig.Dependencies {
			queue = append(queue, dependencyRequest{name: name, version: version, requestor: "<root>"})
		}
	}
	if r.rootConfig.DevDependencies != nil {
		for name, version := range *r.rootConfig.DevDependencies {
			queue = append(queue, dependencyRequest{name: name, version: version, requestor: "<root>"})
		}
	}
	return queue
}

// dedupeRequests drops requests already satisfied at the same version
// and folds identical name@version pairs in this iteration into one entry.
func dedupeRequests(queue []dependencyRequest, resolved map[string]Node) map[string]dependencyRequest {
	uniqueRequests := make(map[string]dependencyRequest)
	for _, req := range queue {
		if exists, ok := resolved[req.name]; ok && exists.Version == req.version {
			continue
		}
		uniqueRequests[req.name+"@"+req.version] = req
	}
	return uniqueRequests
}

// fetchAll fetches metadata for every request concurrently and returns the collected results.
func (r *Resolver) fetchAll(ctx context.Context, requests map[string]dependencyRequest, progress ProgressReporter, w io.Writer) ([]fetchResult, error) {
	results := make(chan fetchResult, len(requests))
	g, gtx := errgroup.WithContext(ctx)
	g.SetLimit(16)

	count := 0
	for _, req := range requests {
		count++
		progress.Stream(w, fmt.Sprintf("  Resolving %s@%s [%d/%d]", req.name, req.version, count, len(requests)))

		g.Go(func() error {
			manifest, err := r.fetchMetadata(gtx, req.name, req.version)
			if err != nil {
				return fmt.Errorf("failed to fetch metadata for %s@%s required by %s: %w", req.name, req.version, req.requestor, err)
			}
			results <- fetchResult{req: req, manifest: manifest}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	close(results)

	collected := make([]fetchResult, 0, len(requests))
	for res := range results {
		collected = append(collected, res)
	}
	return collected, nil
}

// applyResult validates and registers a single fetch result into `resolved`,
// returning any newly discovered child dependencies to enqueue.
func (r *Resolver) applyResult(res fetchResult, resolved map[string]Node) ([]dependencyRequest, error) {
	if existing, ok := resolved[res.req.name]; ok {
		if existing.Version == res.req.version {
			return nil, nil
		}
		if err := r.resolveConflict(res.req, existing); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if err := r.checkRuntimeCompatibility(res.manifest); err != nil {
		return nil, fmt.Errorf(
			"package %s@%s incompatible:\n"+
				"  %w",
			res.req.name, res.req.version, err,
		)
	}

	resolved[res.req.name] = Node{
		Name:         res.manifest.Name,
		Version:      res.manifest.Version,
		Type:         res.manifest.Type,
		Signatures:   res.manifest.Dist.Signatures,
		Digest:       res.manifest.Dist.Digest,
		Bin:          res.manifest.Bin,
		Dependencies: res.manifest.Dependencies,
	}

	if res.manifest.Dependencies == nil {
		return nil, nil
	}
	children := make([]dependencyRequest, 0, len(*res.manifest.Dependencies))
	for name, version := range *res.manifest.Dependencies {
		children = append(children, dependencyRequest{name, version, res.req.name})
	}
	return children, nil
}

type ResolutionError struct {
	Header string
	Detail []string
	Action string
}

func (e *ResolutionError) Error() string {
	msg := e.Header + "\n"
	var builder strings.Builder
	for _, d := range e.Detail {
		builder.WriteString("  ")
		builder.WriteString(d)
		builder.WriteString("\n")
	}
	msg += builder.String()
	msg += "Action: " + e.Action
	return msg
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
			return &ResolutionError{
				Header: fmt.Sprintf("Version downgrade detected for package %s:", req.name),
				Detail: []string{
					"currently resolved: " + rootVersion,
					fmt.Sprintf("%s requires: %s", req.requestor, req.version),
				},
				Action: fmt.Sprintf("Upgrade %s in your wpm.json to %s or higher.", req.name, req.version),
			}
		}

		// If we reach here, the root version satisfies the request, so we can ignore the conflict.
		return nil
	}

	// Unresolvable conflict, user must intervene.
	return &ResolutionError{
		Header: fmt.Sprintf("Dependency version conflict for package %s:", req.name),
		Detail: []string{
			"currently resolved: " + existing.Version,
			fmt.Sprintf("%s requires: %s", req.requestor, req.version),
		},
		Action: fmt.Sprintf(`Add "%s": "%s" (or %s) to the root wpm.json to force a resolution.`, req.name, req.version, existing.Version),
	}
}

func (r *Resolver) checkRuntimeCompatibility(pkg *manifest.Package) error {
	if pkg == nil {
		return errors.New("manifest is nil")
	}

	// If runtime strict mode is disabled, skip checks.
	if !r.rootConfig.RuntimeStrict() {
		return nil
	}

	// If manifest has no requirements, skip
	if pkg.Requires == nil {
		return nil
	}

	// PHP and WordPress version constraints
	requiresWP := pkg.Requires.WP
	requiresPHP := pkg.Requires.PHP

	// PHP and WordPress runtime versions
	runtimeWP := r.rootConfig.Config.Runtime.WP
	runtimePHP := r.rootConfig.Config.Runtime.PHP

	// Check WordPress runtime compatibility.
	if err := checkVersionCompatibility("WordPress", requiresWP, runtimeWP); err != nil {
		return err
	}

	// Check PHP runtime compatibility.
	if err := checkVersionCompatibility("PHP", requiresPHP, runtimePHP); err != nil {
		return err
	}

	return nil
}

func checkVersionCompatibility(name, required, runtime string) error {
	if required == "" || runtime == "" {
		return nil
	}

	constraint, err := semver.NewConstraint(required)
	if err != nil {
		return fmt.Errorf("invalid %s requirement in package: %w", name, err)
	}

	version, err := semver.NewVersion(runtime)
	if err != nil {
		return fmt.Errorf("invalid runtime %s version provided: %w", name, err)
	}

	if !constraint.Check(version) {
		return fmt.Errorf("requires %s %s, but runtime %s version is %s", name, required, name, runtime)
	}

	return nil
}

func (r *Resolver) fetchMetadata(ctx context.Context, name, version string) (*manifest.Package, error) {
	// Try to resolve the manifest from lockfile first
	if r.lockfile != nil && r.lockfile.Packages != nil {
		if lockPkg, ok := r.lockfile.Packages[name]; ok {
			if lockPkg.Version == version {
				return &manifest.Package{
					Name:         name,
					Version:      lockPkg.Version,
					Type:         lockPkg.Type,
					Bin:          lockPkg.Bin,
					Dependencies: lockPkg.Dependencies,
					Dist: manifest.Dist{
						Digest:     lockPkg.Digest,
						Signatures: lockPkg.Signatures,
					},
				}, nil
			}
		}
	}

	return r.client.GetPackageManifest(ctx, name, version, false)
}
