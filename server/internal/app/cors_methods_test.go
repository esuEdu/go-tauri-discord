package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/esuEdu/go-tauri-discord/internal/platform/httpx"
)

// CORS is enforced by browsers and ignored by Go's http.Client, so no test that
// makes a request can notice a method missing from Access-Control-Allow-Methods
// -- and the harness allows every origin anyway. The list has to be checked
// against the routes themselves.
//
// PUT was absent for as long as reactions, avatars and guild icons have
// existed. Nothing broke until a packaged client talked to a server on another
// origin, because a proxied dev server makes the request same-origin and the
// preflight never happens.
func TestEveryRegisteredMethodSurvivesAPreflight(t *testing.T) {
	found := registeredMethods(t)
	if len(found) == 0 {
		t.Fatal("no routes discovered; the scan below stopped matching how routes are registered")
	}

	for _, method := range found {
		if !slices.Contains(httpx.AllowedMethods, method) {
			t.Errorf("routes are registered for %s but httpx.AllowedMethods is %v, "+
				"so a browser refuses every one of them without sending the request",
				method, httpx.AllowedMethods)
		}
	}
}

// registeredMethods reads the method prefix out of every route pattern passed to
// a HandleFunc or Handle call anywhere under internal/.
func registeredMethods(t *testing.T) []string {
	t.Helper()

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("locating internal/: %v", err)
	}

	var methods []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return nil
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "HandleFunc" && sel.Sel.Name != "Handle") {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			pattern, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			method, _, found := strings.Cut(pattern, " ")
			if found && method == strings.ToUpper(method) && method != "" {
				if !slices.Contains(methods, method) {
					methods = append(methods, method)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking internal/: %v", err)
	}

	slices.Sort(methods)
	return methods
}
