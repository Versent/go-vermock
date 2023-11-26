package mock

import (
	"context"

	"golang.org/x/tools/go/packages"
)

// load typechecks the packages that match the given patterns and
// includes source for all transitive dependencies. The patterns are
// defined by the underlying build system. For the go tool, this is
// described at https://golang.org/cmd/go/#hdr-Package_lists_and_patterns
//
// wd is the working directory and env is the set of environment
// variables to use when loading the packages specified by patterns. If
// env is nil or empty, the current environment is used.
// In case of duplicate environment variables, the last one in the list
// takes precedence.
func load(ctx context.Context, wd string, env []string, buildflags []string, patterns []string) ([]*packages.Package, []error) {
	cfg := &packages.Config{
		Context:    ctx,
		Mode:       packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedDeps,
		Dir:        wd,
		Env:        env,
		BuildFlags: buildflags,
	}
	escaped := make([]string, len(patterns))
	for i := range patterns {
		escaped[i] = "pattern=" + patterns[i]
	}
	pkgs, err := packages.Load(cfg, escaped...)
	if err != nil {
		return nil, []error{err}
	}
	var errs []error
	for _, p := range pkgs {
		for _, e := range p.Errors {
			errs = append(errs, e)
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return pkgs, nil
}
