package tools

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var shellAllow = map[string]bool{
	":": true, "true": true, "false": true,
	"cd": true, "pwd": true,
	"echo": true, "printf": true, "read": true,
	"test": true, "[": true,
	"set": true, "unset": true, "shift": true,
	"export": true, "readonly": true, "local": true, "declare": true, "typeset": true,
	"return": true, "exit": true, "break": true, "continue": true,
	"trap": true, "wait": true, "umask": true,
	"alias": true, "unalias": true, "hash": true, "type": true,
	"getopts": true, "let": true,
}

func validateShellScript(script string, denied []string) error {
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Invalid, "syntax.Parse", "sh -c", err)
	}
	var bad error
	syntax.Walk(file, func(node syntax.Node) bool {
		if bad != nil {
			return false
		}
		call, ok := node.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		bin, ok := staticWord(call.Args[0])
		if !ok {
			bad = utilsmsg.New(utilsmsg.Denied, "sh -c", "dynamic command (variable or substitution) not allowed")
			return false
		}
		base := filepath.Base(bin)
		if bin != base {
			bad = utilsmsg.New(utilsmsg.Invalid, "sh -c", "%q must be a bare command name (%q), not a path", bin, base)
			return false
		}
		if base == "rm" {
			bad = utilsmsg.New(utilsmsg.Denied, "sh -c", "rm is not allowed inside sh -c")
			return false
		}
		if base == "sudo" {
			bad = errSudoInShell
			return false
		}
		if base == "open" && runtime.GOOS == "darwin" {
			bad = errOpenCommand
			return false
		}
		if shellAllow[base] {
			return true
		}
		if slices.Contains(denied, base) {
			bad = deniedCommandErr(base)
			return false
		}
		if (base == "sh" || base == "bash") && len(call.Args) >= 3 {
			flag, ok := staticWord(call.Args[1])
			if !ok || flag != "-c" {
				return true
			}
			inner, ok := staticWord(call.Args[2])
			if !ok {
				bad = utilsmsg.New(utilsmsg.Denied, "sh -c", "nested %s -c with dynamic script not allowed", base)
				return false
			}
			if err := validateShellScript(inner, denied); err != nil {
				bad = err
				return false
			}
		}
		return true
	})
	return bad
}

func staticWord(w *syntax.Word) (string, bool) {
	if w == nil {
		return "", false
	}
	var sb strings.Builder
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			sb.WriteString(x.Value)
		case *syntax.SglQuoted:
			sb.WriteString(x.Value)
		case *syntax.DblQuoted:
			for _, pp := range x.Parts {
				lit, ok := pp.(*syntax.Lit)
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
