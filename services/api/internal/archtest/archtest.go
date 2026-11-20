// Package archtest enforces the dependency-direction rules of ARCHITECTURE AD-1 by parsing imports:
//
//	http -> app -> domain;  adapters -> ports;  modules talk to each other only through the other module's ports.
//
// internal/platform is shared infrastructure and may be imported by anyone but may not import any module.
package archtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Violation is one forbidden import.
type Violation struct{ File, Import, Rule string }

func (v Violation) String() string { return v.File + " imports " + v.Import + ": " + v.Rule }

var layers = map[string]bool{"domain": true, "app": true, "ports": true, "adapters": true, "http": true}

// Check walks <root>/internal (root = the Go module directory) and returns every violation.
// modulePath is the Go module path, e.g. "veyrogram/api".
func Check(root, modulePath string) ([]Violation, error) {
	internal := filepath.Join(root, "internal")
	prefix := modulePath + "/internal/"
	var out []Violation
	err := filepath.WalkDir(internal, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(internal, path)
		seg := strings.Split(filepath.ToSlash(rel), "/")
		if len(seg) < 2 || seg[0] == "archtest" { // files directly in internal/ or the checker itself
			return nil
		}
		from := seg[0] // "platform" or a module name
		fromLayer := ""
		if from != "platform" && len(seg) >= 3 && layers[seg[1]] {
			fromLayer = seg[1]
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, im := range f.Imports {
			ip, _ := strconv.Unquote(im.Path.Value)
			if strings.HasPrefix(ip, modulePath+"/cmd/") {
				out = append(out, Violation{rel, ip, "internal packages must not import cmd/*"})
				continue
			}
			if !strings.HasPrefix(ip, prefix) {
				if fromLayer == "domain" && isThirdParty(ip) {
					out = append(out, Violation{rel, ip, "domain must depend only on the standard library and internal/platform (no drivers/frameworks)"})
				}
				continue
			}
			tseg := strings.Split(strings.TrimPrefix(ip, prefix), "/")
			to := tseg[0]
			if to == "platform" {
				continue
			}
			toLayer := ""
			if len(tseg) >= 2 && layers[tseg[1]] {
				toLayer = tseg[1]
			}
			switch {
			case from == "platform":
				out = append(out, Violation{rel, ip, "internal/platform must not import any module"})
			case to != from: // cross-module
				if toLayer != "ports" {
					out = append(out, Violation{rel, ip, "modules may import another module only through its ports package"})
				}
			default: // same module: layer rules
				if bad := layerRule(fromLayer, toLayer); bad != "" {
					out = append(out, Violation{rel, ip, bad})
				}
			}
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].File+out[i].Import < out[j].File+out[j].Import })
	return out, err
}

func isThirdParty(ip string) bool { return strings.Contains(strings.SplitN(ip, "/", 2)[0], ".") }

// layerRule returns a non-empty reason when `from` may not import `to` inside one module.
func layerRule(from, to string) string {
	allowed := map[string][]string{
		"domain":   {"domain"},
		"ports":    {"ports", "domain"},
		"app":      {"app", "domain", "ports"},
		"adapters": {"adapters", "ports", "domain"},
		"http":     {"http", "app", "domain", "ports"},
	}
	if from == "" || to == "" {
		return ""
	}
	for _, ok := range allowed[from] {
		if ok == to {
			return ""
		}
	}
	return from + " must not import " + to + " (direction is http -> app -> domain, adapters -> ports)"
}
