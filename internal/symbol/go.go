package symbol

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type Location struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Receiver string `json:"receiver,omitempty"`
	Start    int    `json:"start_line"`
	End      int    `json:"end_line"`
}

// ResolveGo anchors a symbol name to concrete Go declarations at HEAD. The
// query may be "Name", "Type.Method", or "path/to/file.go::Name".
func ResolveGo(repoRoot, query string) ([]Location, error) {
	pathFilter, name := splitQuery(query)
	fset := token.NewFileSet()
	var out []Location
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if path != repoRoot && (base == ".git" || base == "vendor" || base == "node_modules" || base == "dist" || base == ".whyth") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".go") || strings.HasSuffix(path, "_test.go") && strings.HasPrefix(name, "Test") == false {
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if pathFilter != "" && filepath.ToSlash(filepath.Clean(pathFilter)) != rel {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				recv := receiverName(d.Recv)
				qualified := d.Name.Name
				if recv != "" {
					qualified = recv + "." + d.Name.Name
				}
				if name != d.Name.Name && name != qualified {
					continue
				}
				out = append(out, location(fset, rel, qualified, "function", recv, d.Pos(), d.End()))
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if name == s.Name.Name {
							out = append(out, location(fset, rel, s.Name.Name, "type", "", d.Pos(), d.End()))
						}
					case *ast.ValueSpec:
						kind := strings.ToLower(d.Tok.String())
						for _, ident := range s.Names {
							if name == ident.Name {
								out = append(out, location(fset, rel, ident.Name, kind, "", d.Pos(), d.End()))
							}
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func splitQuery(query string) (string, string) {
	if i := strings.LastIndex(query, "::"); i >= 0 {
		return strings.TrimSpace(query[:i]), strings.TrimSpace(query[i+2:])
	}
	return "", strings.TrimSpace(query)
}

func location(fset *token.FileSet, path, name, kind, receiver string, start, end token.Pos) Location {
	return Location{Path: path, Name: name, Kind: kind, Receiver: receiver, Start: fset.Position(start).Line, End: fset.Position(end).Line}
}

func receiverName(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return ""
	}
	expr := fl.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	if idx, ok := expr.(*ast.IndexExpr); ok {
		if id, ok := idx.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	if idx, ok := expr.(*ast.IndexListExpr); ok {
		if id, ok := idx.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}
