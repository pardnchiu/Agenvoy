package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/sudo"

	"mvdan.cc/sh/v3/syntax"
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

func validateShellScript(script string, allowed map[string]bool) error {
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil {
		return fmt.Errorf("sh -c parse: %w", err)
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
			bad = fmt.Errorf("sh -c: dynamic command (variable or substitution) not allowed")
			return false
		}
		base := filepath.Base(bin)
		if shellAllow[base] {
			return true
		}
		if !allowed[base] {
			bad = fmt.Errorf("failed to run command: %s is not allowed", bin)
			return false
		}
		if (base == "sh" || base == "bash") && len(call.Args) >= 3 {
			flag, ok := staticWord(call.Args[1])
			if !ok || flag != "-c" {
				return true
			}
			inner, ok := staticWord(call.Args[2])
			if !ok {
				bad = fmt.Errorf("sh -c: nested %s -c with dynamic script not allowed", base)
				return false
			}
			if err := validateShellScript(inner, allowed); err != nil {
				bad = err
				return false
			}
		}
		return true
	})
	return bad
}

func validateShellScriptFloor(script string) error {
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil {
		return fmt.Errorf("sh -c parse: %w", err)
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
		for _, arg := range call.Args {
			if word, ok := staticWord(arg); ok {
				if blocked, hit := sudo.HitFloor(word); hit {
					bad = fmt.Errorf("access denied (floor): %s", blocked)
					return false
				}
			}
		}
		if bin, ok := staticWord(call.Args[0]); ok {
			base := filepath.Base(bin)
			if (base == "sh" || base == "bash") && len(call.Args) >= 3 {
				flag, ok := staticWord(call.Args[1])
				if !ok || flag != "-c" {
					return true
				}
				inner, ok := staticWord(call.Args[2])
				if !ok {
					return true
				}
				if err := validateShellScriptFloor(inner); err != nil {
					bad = err
					return false
				}
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
