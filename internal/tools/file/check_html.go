package file

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
)

var (
	regexScript   = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
	regexAttrId   = regexp.MustCompile(`\bid\s*=\s*["']([^"']+)["']`)
	regexAttrSrc  = regexp.MustCompile(`(?i)\bsrc\s*=`)
	regexAttrType = regexp.MustCompile(`(?i)\btype\s*=\s*["']?([^"'\s>]+)`)
	regexTInit    = regexp.MustCompile("T\\.init\\(\\s*['\"`]([^'\"`$]+)['\"`]\\s*,")
	regexQuickUI  = regexp.MustCompile(`(?i)<script[^>]+src\s*=\s*["'][^"']*@pardnchiu/quickui@`)
	regexMustache = regexp.MustCompile(`\{\{([^}]*)\}\}`)
	regexExpr     = regexp.MustCompile(`&&|\|\||=>|\.toFixed\(|\?[^:]*:`)
	regexIf       = regexp.MustCompile(`:(?:if|else-if)\s*=\s*"([^"]*)"`)
)

func checkHTML(html string) []string {
	var problems []string

	var sb strings.Builder
	for i, match := range regexScript.FindAllStringSubmatch(html, -1) {
		attrs, body := match[1], match[2]
		if regexAttrSrc.MatchString(attrs) || strings.TrimSpace(body) == "" {
			continue
		}
		if t := regexAttrType.FindStringSubmatch(attrs); t != nil {
			switch strings.ToLower(t[1]) {
			case "text/javascript", "application/javascript", "module":
			default:
				continue
			}
		}
		if _, err := js.Parse(parse.NewInputString(body), js.Options{}); err != nil {
			problems = append(problems, fmt.Sprintf("<script> #%d does not parse, so nothing on the page renders: %v", i+1, err))
		}
		sb.WriteString(body)
		sb.WriteString("\n")
	}
	script := sb.String()

	markup := regexScript.ReplaceAllString(html, "")
	ids := make(map[string]bool)
	for _, m := range regexAttrId.FindAllStringSubmatch(markup, -1) {
		ids[m[1]] = true
	}
	var missing []string
	for _, m := range regexTInit.FindAllStringSubmatch(script, -1) {
		if ids[m[1]] {
			continue
		}
		quoted := strings.Count(script, `"`+m[1]+`"`) + strings.Count(script, `'`+m[1]+`'`) + strings.Count(script, "`"+m[1]+"`")
		if quoted < 2 {
			missing = append(missing, m[1])
		}
	}
	if len(missing) > 0 {
		problems = append(problems, fmt.Sprintf("T.init targets with no container in the markup or the script (the chart silently draws nothing): %s", strings.Join(slices.Compact(slices.Sorted(slices.Values(missing))), ", ")))
	}

	if regexQuickUI.MatchString(html) {
		var exprs []string
		for _, m := range regexMustache.FindAllStringSubmatch(markup, -1) {
			if regexExpr.MatchString(m[1]) {
				exprs = append(exprs, "{{"+m[1]+"}}")
			}
		}
		if len(exprs) > 0 {
			problems = append(problems, fmt.Sprintf("{{ }} evaluates no expressions and renders these literally; precompute the value in data: %s", strings.Join(slices.Compact(slices.Sorted(slices.Values(exprs))), ", ")))
		}

		var conds []string
		for _, m := range regexIf.FindAllStringSubmatch(markup, -1) {
			if regexExpr.MatchString(m[1]) {
				conds = append(conds, m[1])
			}
		}
		if len(conds) > 0 {
			problems = append(problems, fmt.Sprintf(":if takes one comparison, no && / || / ternary; precompute a flag in data: %s", strings.Join(slices.Compact(slices.Sorted(slices.Values(conds))), ", ")))
		}
	}

	return problems
}
