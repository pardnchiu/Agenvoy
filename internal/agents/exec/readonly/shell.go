package readonly

import (
	"path/filepath"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/pardnchiu/agenvoy/internal/tools/file/boundary"
)

var shellWrappers = []string{"sh", "bash"}

var readOnlyShellBuiltins = map[string]bool{
	":": true, "true": true, "false": true, "pwd": true, "cd": true,
	"echo": true, "printf": true, "test": true, "[": true, "type": true,
}

func shellScript(argv []string) (string, bool) {
	if len(argv) != 3 || argv[1] != "-c" || !slices.Contains(shellWrappers, filepath.Base(argv[0])) {
		return "", false
	}
	return argv[2], true
}

func isReadOnlyShell(script string) bool {
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil || len(file.Stmts) == 0 {
		return false
	}
	ok := true
	syntax.Walk(file, func(node syntax.Node) bool {
		if !ok {
			return false
		}
		switch n := node.(type) {
		case *syntax.WhileClause, *syntax.ForClause, *syntax.FuncDecl, *syntax.CoprocClause,
			*syntax.DeclClause, *syntax.LetClause, *syntax.ArithmCmd, *syntax.ArithmExp, *syntax.ParamExp:
			ok = false
		case *syntax.Redirect:
			ok = readOnlyRedirect(n)
		case *syntax.CallExpr:
			ok = readOnlyCall(n)
		}
		return ok
	})
	return ok
}

func readOnlyRedirect(r *syntax.Redirect) bool {
	switch r.Op {
	case syntax.DplIn, syntax.DplOut, syntax.Hdoc, syntax.DashHdoc, syntax.WordHdoc:
		return true
	}
	target, ok := literalWord(r.Word)
	if !ok {
		return false
	}
	switch r.Op {
	case syntax.RdrIn:
		return !boundary.IsSensitivePath(target)
	case syntax.RdrOut, syntax.AppOut, syntax.RdrAll, syntax.AppAll, syntax.ClbOut:
		return target == "/dev/null"
	}
	return false
}

func readOnlyCall(call *syntax.CallExpr) bool {
	if len(call.Assigns) > 0 {
		return false
	}
	if len(call.Args) == 0 {
		return true
	}
	argv := make([]string, 0, len(call.Args))
	for _, word := range call.Args {
		one, ok := literalWord(word)
		if !ok {
			return false
		}
		argv = append(argv, one)
	}
	if readOnlyShellBuiltins[argv[0]] {
		if argv[0] == "printf" && slices.ContainsFunc(argv[1:], func(a string) bool { return strings.HasPrefix(a, "-v") }) {
			return false
		}
		return !slices.ContainsFunc(argv[1:], boundary.IsSensitivePath)
	}
	if _, nested := shellScript(argv); nested {
		return false
	}
	return isReadOnlyArgv(argv)
}

func literalWord(word *syntax.Word) (string, bool) {
	if word == nil {
		return "", false
	}
	var sb strings.Builder
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			if strings.ContainsAny(p.Value, "*?[{") {
				return "", false
			}
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			sb.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, inner := range p.Parts {
				lit, ok := inner.(*syntax.Lit)
				if !ok {
					return "", false
				}
				sb.WriteString(lit.Value)
			}
		default:
			return "", false
		}
	}
	return sb.String(), true
}
