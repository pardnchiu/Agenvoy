package exec

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

var (
	sqlDropRegex    = regexp.MustCompile(`(?i)\b(drop\s+(table|database|schema|view|index)|truncate)\b`)
	sqlUnboundRegex = regexp.MustCompile(`(?i)\b(delete\s+from|update\s+\S+\s+set)\b`)
	sqlWhereRegex   = regexp.MustCompile(`(?i)\bwhere\b`)
	secretVarRegex  = regexp.MustCompile(`\$\{?[A-Za-z0-9_]*(?i:key|token|secret|passw(or)?d|credential)[A-Za-z0-9_]*`)
	shellBins       = []string{"sh", "bash", "zsh", "dash", "fish"}
	interpreterBins = []string{"sh", "bash", "zsh", "dash", "fish", "python", "python3", "perl", "ruby", "node", "php"}
	fetchBins       = []string{"curl", "wget", "http", "https", "nc", "ncat"}
	sqlBins         = []string{"sqlite3", "duckdb", "psql", "mysql", "mariadb"}
	secretReaders   = []string{"security find-generic-password", "secret-tool lookup", "printenv"}
)

var subcommandRules = map[string][][]string{
	"terraform":  {{"destroy"}, {"apply"}},
	"tofu":       {{"destroy"}, {"apply"}},
	"pulumi":     {{"destroy"}, {"up"}},
	"cdk":        {{"destroy"}, {"deploy"}},
	"kubectl":    {{"delete"}, {"apply"}, {"replace"}, {"drain"}},
	"helm":       {{"install"}, {"upgrade"}, {"uninstall"}, {"delete"}, {"rollback"}},
	"firebase":   {{"deploy"}},
	"vercel":     {{"deploy"}, {"--prod"}, {"remove"}},
	"netlify":    {{"deploy"}},
	"wrangler":   {{"deploy"}, {"publish"}, {"delete"}},
	"fly":        {{"deploy"}, {"destroy"}},
	"flyctl":     {{"deploy"}, {"destroy"}},
	"serverless": {{"deploy"}, {"remove"}},
	"sls":        {{"deploy"}, {"remove"}},
	"alembic":    {{"upgrade"}, {"downgrade"}},
	"goose":      {{"up"}, {"down"}, {"reset"}, {"redo"}},
	"dbmate":     {{"up"}, {"down"}, {"drop"}, {"migrate"}, {"rollback"}},
	"flyway":     {{"migrate"}, {"clean"}, {"undo"}},
	"liquibase":  {{"update"}, {"rollback"}, {"drop-all"}},
	"prisma":     {{"migrate", "deploy"}, {"migrate", "reset"}, {"db", "push"}},
	"gh":         {{"pr", "merge"}, {"repo", "delete"}, {"repo", "edit"}, {"repo", "rename"}, {"release", "delete"}, {"secret", "set"}, {"secret", "delete"}},
	"glab":       {{"mr", "merge"}, {"repo", "delete"}},
}

var migrateTokens = []string{"db:migrate", "db:drop", "db:reset", "db:rollback", "db:schema:load", "migrate:latest", "migrate:rollback", "migrate:fresh", "migrate:reset"}

func isIrreversibleCommand(toolName, toolArgs string) bool {
	if toolName != "run_command" && toolName != "background_command" {
		return false
	}
	var p struct {
		Argv []string `json:"argv"`
	}
	if json.Unmarshal([]byte(toolArgs), &p) != nil || len(p.Argv) == 0 {
		return false
	}

	if len(p.Argv) >= 3 && slices.Contains(shellBins, filepath.Base(p.Argv[0])) && p.Argv[1] == "-c" {
		return isIrreversibleScript(p.Argv[2])
	}
	return isIrreversibleArgv(p.Argv)
}

func isIrreversibleScript(script string) bool {
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil {
		return false
	}
	var bins []string
	found := false
	syntax.Walk(file, func(node syntax.Node) bool {
		if found {
			return false
		}
		call, ok := node.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		argv := make([]string, 0, len(call.Args))
		for _, w := range call.Args {
			argv = append(argv, wordText(w))
		}
		bin := filepath.Base(argv[0])
		bins = append(bins, bin)
		if isIrreversibleArgv(argv) || slices.Contains(sqlBins, bin) && isDestructiveSQL(script) {
			found = true
		}
		return !found
	})
	if found {
		return true
	}

	fetches := slices.ContainsFunc(bins, func(b string) bool { return slices.Contains(fetchBins, b) })
	if !fetches {
		return false
	}
	if slices.ContainsFunc(bins, func(b string) bool { return slices.Contains(interpreterBins, b) }) {
		return true
	}
	if secretVarRegex.MatchString(script) {
		return true
	}
	return slices.ContainsFunc(secretReaders, func(r string) bool { return strings.Contains(script, r) })
}

func isIrreversibleArgv(argv []string) bool {
	argv = stripLauncher(argv)
	if len(argv) == 0 {
		return false
	}
	bin := filepath.Base(argv[0])
	args := argv[1:]

	switch bin {
	case "git":
		return isIrreversibleGit(args)
	case "gcloud", "az", "doctl":
		return slices.ContainsFunc(positional(args), func(a string) bool {
			return a == "delete" || a == "deploy" || strings.HasSuffix(a, "-iam-policy-binding") || a == "set-iam-policy"
		})
	case "aws":
		return isIrreversibleAWS(positional(args))
	case "gsutil":
		return slices.Contains(args, "rb") || slices.Contains(args, "rm") && slices.ContainsFunc(args, func(a string) bool { return a == "-r" || a == "-R" })
	case "gh":
		if pos := positional(args); len(pos) > 0 && pos[0] == "api" {
			return isMutatingGhAPI(args)
		}
	}
	if slices.Contains(sqlBins, bin) {
		return isDestructiveSQL(strings.Join(args, " "))
	}
	if slices.Contains(fetchBins, bin) && hasSensitiveUpload(args) {
		return true
	}
	if slices.ContainsFunc(args, func(a string) bool { return slices.Contains(migrateTokens, a) }) {
		return true
	}
	if slices.Contains(args, "manage.py") && slices.Contains(args, "migrate") || bin == "manage.py" && slices.Contains(args, "migrate") {
		return true
	}
	if rules, ok := subcommandRules[bin]; ok {
		pos := positional(args)
		for _, rule := range rules {
			if strings.HasPrefix(rule[0], "--") {
				if slices.Contains(args, rule[0]) {
					return true
				}
				continue
			}
			if len(pos) >= len(rule) && slices.Equal(pos[:len(rule)], rule) {
				return true
			}
		}
	}
	return false
}

func isIrreversibleGit(args []string) bool {
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		if (args[0] == "-C" || args[0] == "-c") && len(args) > 1 {
			args = args[2:]
			continue
		}
		args = args[1:]
	}
	if len(args) == 0 {
		return false
	}
	rest := args[1:]
	switch args[0] {
	case "reset":
		return slices.Contains(rest, "--hard")
	case "clean":
		return !slices.ContainsFunc(rest, func(a string) bool {
			return a == "--dry-run" || !strings.HasPrefix(a, "--") && strings.HasPrefix(a, "-") && strings.Contains(a, "n")
		})
	case "push":
		return slices.ContainsFunc(rest, func(a string) bool {
			switch {
			case a == "-f", a == "--force", a == "--delete", a == "-d", a == "--mirror", a == "--prune":
				return true
			case strings.HasPrefix(a, "--force-with-lease"), strings.HasPrefix(a, "--force-if-includes"):
				return true
			case !strings.HasPrefix(a, "--") && strings.HasPrefix(a, "-") && strings.ContainsAny(a[1:], "fd"):
				return true
			case strings.HasPrefix(a, "+"), strings.HasPrefix(a, ":"):
				return true
			}
			return false
		})
	}
	return false
}

func isIrreversibleAWS(pos []string) bool {
	if len(pos) < 2 {
		return false
	}
	service, op := pos[0], pos[1]
	switch {
	case service == "iam":
		return !strings.HasPrefix(op, "get-") && !strings.HasPrefix(op, "list-") && !strings.HasPrefix(op, "generate-")
	case service == "s3":
		return op == "rb" || op == "rm" || op == "mv" || op == "sync"
	case service == "deploy":
		return true
	}
	return strings.HasPrefix(op, "delete-") || strings.HasPrefix(op, "terminate-") || strings.HasPrefix(op, "remove-") || strings.HasPrefix(op, "deregister-") || op == "deploy"
}

func stripLauncher(argv []string) []string {
	for len(argv) > 0 {
		bin := filepath.Base(argv[0])
		switch {
		case bin == "npx" || bin == "bunx" || bin == "pnpx":
			argv = dropFlags(argv[1:])
		case (bin == "pnpm" || bin == "yarn" || bin == "npm" || bin == "bun") && len(argv) > 1 && (argv[1] == "exec" || argv[1] == "dlx" || argv[1] == "x"):
			argv = dropFlags(argv[2:])
		case (bin == "uv" || bin == "poetry" || bin == "pipenv") && len(argv) > 1 && argv[1] == "run":
			argv = dropFlags(argv[2:])
		case (bin == "python" || bin == "python3") && len(argv) > 2 && argv[1] == "-m":
			argv = argv[2:]
		default:
			return argv
		}
	}
	return argv
}

func dropFlags(argv []string) []string {
	for len(argv) > 0 && strings.HasPrefix(argv[0], "-") {
		argv = argv[1:]
	}
	return argv
}

func positional(args []string) []string {
	var out []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			out = append(out, a)
		}
	}
	return out
}

func isMutatingGhAPI(args []string) bool {
	method := ""
	hasFields := false
	for i, a := range args {
		switch {
		case (a == "-X" || a == "--method") && i+1 < len(args):
			method = strings.ToUpper(args[i+1])
		case strings.HasPrefix(a, "--method="):
			method = strings.ToUpper(strings.TrimPrefix(a, "--method="))
		case strings.HasPrefix(a, "-X") && len(a) > 2:
			method = strings.ToUpper(a[2:])
		case a == "-f" || a == "-F" || a == "--field" || a == "--raw-field" || a == "--input":
			hasFields = true
		}
	}
	if method == "" {
		return hasFields
	}
	return method != "GET" && method != "HEAD"
}

func hasSensitiveUpload(args []string) bool {
	return slices.ContainsFunc(args, func(a string) bool {
		_, path, ok := strings.Cut(a, "@")
		return ok && path != "" && filesystem.IsSensitivePath(path)
	})
}

func isDestructiveSQL(text string) bool {
	if sqlDropRegex.MatchString(text) {
		return true
	}
	return sqlUnboundRegex.MatchString(text) && !sqlWhereRegex.MatchString(text)
}

func wordText(w *syntax.Word) string {
	var sb strings.Builder
	for _, part := range w.Parts {
		switch v := part.(type) {
		case *syntax.Lit:
			sb.WriteString(v.Value)
		case *syntax.SglQuoted:
			sb.WriteString(v.Value)
		case *syntax.DblQuoted:
			for _, inner := range v.Parts {
				if lit, ok := inner.(*syntax.Lit); ok {
					sb.WriteString(lit.Value)
				}
			}
		}
	}
	return sb.String()
}
