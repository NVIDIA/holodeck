/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package provisioner_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// provisionerImportPath is the import path New/WithSSHConfig are defined
// under. Call sites are matched by resolved import path, not by the local
// package identifier, so an import alias can't hide a call site from this
// test.
const provisionerImportPath = "github.com/NVIDIA/holodeck/pkg/provisioner"

// newCallSite is one qualified provisioner.New(...) call found in production
// source, plus whether its enclosing function also calls WithSSHConfig.
type newCallSite struct {
	file  string
	line  int
	wired bool
}

// TestProvisionerNewCallSitesWireSSHConfig is a repository-wide contract
// test: every production call to provisioner.New must wire WithSSHConfig
// (directly in the call, or via an Option built elsewhere in the same
// function), so that auth.sshConfig (bastion hop, TOFU/strict/off host-key
// policy, timeouts, retries) actually reaches the Dialer instead of silently
// falling back to New's accept-new default.
//
// It complements the per-call-site behavioral proving tests
// (TestGetKubeConfig_StrictPolicyReachesDialer in pkg/utils,
// TestRunSingleNodeProvision_StrictPolicyReachesDialer in cmd/cli/create,
// TestRunProvision_StrictPolicyReachesDialer in cmd/cli/update) for the one
// site where driving a fake SSH server end-to-end is not practical: the CI
// entrypoint (cmd/action/ci/entrypoint.go), which requires a real AWS
// provider to run before it ever reaches provisioner.New.
//
// Call sites are discovered by walking the module's own source tree, so this
// test cannot silently rot: dropping WithSSHConfig from any site, or adding a
// brand-new unwired provisioner.New call site anywhere in the module, fails
// it. A hardcoded site count would not catch the latter.
func TestProvisionerNewCallSitesWireSSHConfig(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()

	var sites []newCallSite

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" {
				return filepath.SkipDir
			}
			if d.Name() != "." && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}

		alias, imported := provisionerAlias(f)
		if !imported {
			return nil
		}

		sites = append(sites, findNewCallSites(fset, f, alias)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walking module source tree from %s: %v", root, err)
	}

	if len(sites) < 4 {
		t.Fatalf("expected at least 4 provisioner.New call sites in production code, found %d: %+v",
			len(sites), sites)
	}

	for _, s := range sites {
		if !s.wired {
			t.Errorf("%s:%d: provisioner.New is called without WithSSHConfig anywhere in the "+
				"enclosing function — auth.sshConfig (bastion/TOFU/strict/off, timeouts, retries) "+
				"would silently never reach the Dialer at this call site", s.file, s.line)
		}
	}
}

// provisionerAlias reports the local identifier used to refer to
// pkg/provisioner in f, resolving import aliases. imported is false if f
// does not import pkg/provisioner at all.
func provisionerAlias(f *ast.File) (alias string, imported bool) {
	for _, imp := range f.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		if p != provisionerImportPath {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name, true
		}
		return "provisioner", true
	}
	return "", false
}

// findNewCallSites returns every qualified <alias>.New(...) call in f,
// marking each as wired if its enclosing function also contains a
// <alias>.WithSSHConfig(...) call anywhere in its body.
func findNewCallSites(fset *token.FileSet, f *ast.File, alias string) []newCallSite {
	var sites []newCallSite

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		var calls []*ast.CallExpr
		hasWithSSHConfig := false

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok || ident.Name != alias {
				return true
			}
			switch sel.Sel.Name {
			case "New":
				calls = append(calls, call)
			case "WithSSHConfig":
				hasWithSSHConfig = true
			}
			return true
		})

		for _, call := range calls {
			pos := fset.Position(call.Pos())
			sites = append(sites, newCallSite{file: pos.Filename, line: pos.Line, wired: hasWithSSHConfig})
		}
	}

	return sites
}

// moduleRoot locates the repository root (the directory containing go.mod)
// relative to this test file, so the walk works regardless of the package
// this test lives in or the directory `go test` is invoked from.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to resolve this test file's path")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate module root (go.mod) starting from %s", file)
		}
		dir = parent
	}
}
