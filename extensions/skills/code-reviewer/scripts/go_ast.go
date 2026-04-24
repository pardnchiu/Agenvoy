// go_ast.go — AST analyzer for Go projects.
// Invoked by analyze_go.py via `go run go_ast.go <project_root>`.
// Outputs JSON: { functions, issues, max_nesting_depth }.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type Issue struct {
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	CodeSnippet string `json:"code_snippet"`
	Suggestion  string `json:"suggestion"`
}

type FunctionInfo struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	LineCount int    `json:"line_count"`
	HasDoc    bool   `json:"has_doc"`
}

type Result struct {
	Functions       []FunctionInfo `json:"functions"`
	Issues          []Issue        `json:"issues"`
	MaxNestingDepth int            `json:"max_nesting_depth"`
}

const (
	longFunctionThreshold = 50
	deepNestingThreshold  = 3
)

var ignoredDirs = map[string]struct{}{
	".git": {}, "node_modules": {}, "vendor": {}, "dist": {}, "build": {},
	".idea": {}, ".vscode": {}, "__pycache__": {}, ".next": {}, ".nuxt": {},
	"target": {}, "coverage": {}, ".nyc_output": {}, "venv": {}, ".venv": {},
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go_ast <project_root>")
		os.Exit(1)
	}
	root := os.Args[1]
	result := &Result{
		Functions: []FunctionInfo{},
		Issues:    []Issue{},
	}

	fset := token.NewFileSet()
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if _, skip := ignoredDirs[info.Name()]; skip {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		analyzeFile(fset, path, root, result)
		return nil
	})

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
}

func analyzeFile(fset *token.FileSet, path, root string, result *Result) {
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}

	used := collectUsedIdents(file)
	for _, imp := range file.Imports {
		checkUnusedImport(fset, imp, used, rel, result)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		it, ok := n.(*ast.InterfaceType)
		if !ok {
			return true
		}
		if it.Methods == nil || len(it.Methods.List) == 0 {
			pos := fset.Position(it.Pos())
			result.Issues = append(result.Issues, Issue{
				Severity:    "low",
				Category:    "quality",
				Title:       "使用 interface{}",
				Description: "Go 1.18+ 應使用 any 取代 interface{}",
				File:        rel,
				Line:        pos.Line,
				Suggestion:  "將 interface{} 替換為 any",
			})
		}
		return true
	})

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		analyzeFunction(fset, fn, rel, result)
	}

	ast.Inspect(file, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			checkDiscardedReturn(fset, assign, rel, result)
		}
		return true
	})
}

func collectUsedIdents(file *ast.File) map[string]bool {
	used := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok {
				used[id.Name] = true
			}
		case *ast.Ident:
			used[x.Name] = true
		}
		return true
	})
	return used
}

func checkUnusedImport(fset *token.FileSet, imp *ast.ImportSpec, used map[string]bool, rel string, result *Result) {
	path := strings.Trim(imp.Path.Value, `"`)
	if path == "" {
		return
	}
	var name string
	if imp.Name != nil {
		name = imp.Name.Name
		if name == "_" || name == "." {
			return
		}
	} else {
		parts := strings.Split(path, "/")
		name = parts[len(parts)-1]
		if len(parts) >= 2 {
			last := parts[len(parts)-1]
			if isVersionSegment(last) {
				name = parts[len(parts)-2]
			}
		}
	}
	if !used[name] {
		pos := fset.Position(imp.Pos())
		result.Issues = append(result.Issues, Issue{
			Severity:    "low",
			Category:    "quality",
			Title:       "未使用的 import",
			Description: fmt.Sprintf("套件 '%s' 可能未被使用", path),
			File:        rel,
			Line:        pos.Line,
			Suggestion:  "移除未使用的 import；若為 side-effect 請改為 `_ \"...\"`",
		})
	}
}

func isVersionSegment(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func analyzeFunction(fset *token.FileSet, fn *ast.FuncDecl, rel string, result *Result) {
	start := fset.Position(fn.Pos())
	end := fset.Position(fn.End())
	lineCount := end.Line - start.Line + 1

	sig := formatFuncSignature(fn)

	result.Functions = append(result.Functions, FunctionInfo{
		Name:      fn.Name.Name,
		Signature: sig,
		File:      rel,
		Line:      start.Line,
		LineCount: lineCount,
		HasDoc:    fn.Doc != nil && fn.Doc.Text() != "",
	})

	if lineCount > longFunctionThreshold {
		result.Issues = append(result.Issues, Issue{
			Severity:    "medium",
			Category:    "quality",
			Title:       "過長的函式",
			Description: fmt.Sprintf("函式 '%s' 有 %d 行", fn.Name.Name, lineCount),
			File:        rel,
			Line:        start.Line,
			Suggestion:  "拆分為多個小函式，遵循單一職責原則",
		})
	}

	depth := blockDepth(fn.Body, 0)
	if depth > result.MaxNestingDepth {
		result.MaxNestingDepth = depth
	}
	if depth > deepNestingThreshold {
		result.Issues = append(result.Issues, Issue{
			Severity:    "medium",
			Category:    "quality",
			Title:       "過深的巢狀結構",
			Description: fmt.Sprintf("函式 '%s' 巢狀深度 %d 層", fn.Name.Name, depth),
			File:        rel,
			Line:        start.Line,
			Suggestion:  "使用 early return 或抽出子函式降低巢狀深度",
		})
	}
}

func formatFuncSignature(fn *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("func ")
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		b.WriteString("(")
		writeFieldList(&b, fn.Recv.List)
		b.WriteString(") ")
	}
	b.WriteString(fn.Name.Name)
	b.WriteString("(")
	if fn.Type.Params != nil {
		writeFieldList(&b, fn.Type.Params.List)
	}
	b.WriteString(")")
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		b.WriteString(" ")
		multi := len(fn.Type.Results.List) > 1 || (len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) > 0)
		if multi {
			b.WriteString("(")
		}
		writeFieldList(&b, fn.Type.Results.List)
		if multi {
			b.WriteString(")")
		}
	}
	return b.String()
}

func writeFieldList(b *strings.Builder, fields []*ast.Field) {
	for i, field := range fields {
		if i > 0 {
			b.WriteString(", ")
		}
		for j, name := range field.Names {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString(name.Name)
		}
		if len(field.Names) > 0 {
			b.WriteString(" ")
		}
		b.WriteString(exprString(field.Type))
	}
}

func exprString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return "*" + exprString(x.X)
	case *ast.SelectorExpr:
		return exprString(x.X) + "." + x.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprString(x.Elt)
	case *ast.MapType:
		return "map[" + exprString(x.Key) + "]" + exprString(x.Value)
	case *ast.InterfaceType:
		if x.Methods == nil || len(x.Methods.List) == 0 {
			return "any"
		}
		return "interface{...}"
	case *ast.Ellipsis:
		return "..." + exprString(x.Elt)
	case *ast.ChanType:
		return "chan " + exprString(x.Value)
	case *ast.FuncType:
		return "func(...)"
	}
	return "?"
}

func blockDepth(block *ast.BlockStmt, current int) int {
	if block == nil {
		return current
	}
	max := current
	for _, stmt := range block.List {
		d := stmtDepth(stmt, current)
		if d > max {
			max = d
		}
	}
	return max
}

func stmtDepth(stmt ast.Stmt, current int) int {
	if stmt == nil {
		return current
	}
	switch s := stmt.(type) {
	case *ast.IfStmt:
		d := blockDepth(s.Body, current+1)
		if s.Else != nil {
			var d2 int
			switch e := s.Else.(type) {
			case *ast.IfStmt:
				d2 = stmtDepth(e, current)
			case *ast.BlockStmt:
				d2 = blockDepth(e, current+1)
			}
			if d2 > d {
				d = d2
			}
		}
		return d
	case *ast.ForStmt:
		return blockDepth(s.Body, current+1)
	case *ast.RangeStmt:
		return blockDepth(s.Body, current+1)
	case *ast.SwitchStmt:
		return blockDepth(s.Body, current+1)
	case *ast.TypeSwitchStmt:
		return blockDepth(s.Body, current+1)
	case *ast.SelectStmt:
		return blockDepth(s.Body, current+1)
	case *ast.BlockStmt:
		return blockDepth(s, current+1)
	case *ast.CaseClause:
		max := current
		for _, sub := range s.Body {
			d := stmtDepth(sub, current)
			if d > max {
				max = d
			}
		}
		return max
	case *ast.CommClause:
		max := current
		for _, sub := range s.Body {
			d := stmtDepth(sub, current)
			if d > max {
				max = d
			}
		}
		return max
	}
	return current
}

func checkDiscardedReturn(fset *token.FileSet, assign *ast.AssignStmt, rel string, result *Result) {
	// Only flag the narrow form `_ = f()`. Multi-return requires type info to
	// decide correctly and is deferred to staticcheck / errcheck.
	if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return
	}
	ident, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || ident.Name != "_" {
		return
	}
	if _, ok := assign.Rhs[0].(*ast.CallExpr); !ok {
		return
	}
	pos := fset.Position(assign.Pos())
	result.Issues = append(result.Issues, Issue{
		Severity:    "medium",
		Category:    "quality",
		Title:       "丟棄函式回傳值",
		Description: "使用 `_ = f()` 顯式丟棄回傳值，需確認是否應處理 error",
		File:        rel,
		Line:        pos.Line,
		Suggestion:  "若回傳含 error 請處理；若為 fire-and-forget 請加註解說明",
	})
}
