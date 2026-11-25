package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"

	"github.com/pardnchiu/agenvoy/configs"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func call(ctx context.Context, script Script, meta any) (bool, string, error) {
	if !go_pkg_filesystem_reader.IsFile(script.Path) {
		return true, "", nil
	}

	var stdin []byte
	// NOTE: 每個 state 的 meta data 不一樣所以用 any
	if meta != nil {
		raw, err := json.Marshal(meta)
		if err != nil {
			return false, "", utilsmsg.Wrap(utilsmsg.Internal, "json: Marshal", script.Path, err)
		}
		stdin = raw
	}

	hookCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), configs.HOOK_TIMEOUT)
	defer cancel()

	cmd, err := go_pkg_sandbox.Wrap(hookCtx, script.Runtime, []string{script.Path}, script.WorkDir, nil)
	if err != nil {
		return false, "", utilsmsg.Wrap(utilsmsg.Denied, "go_pkg_sandbox: Wrap", script.Path, err)
	}
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	raw := strings.TrimSpace(stdout.String())
	if i := strings.LastIndexByte(raw, '\n'); i >= 0 {
		if logs := strings.TrimSpace(raw[:i]); logs != "" {
			slog.Debug("[Hook Function Log] "+script.Path+": stdout", "log", logs)
		}
		raw = strings.TrimSpace(raw[i+1:])
	}
	if logs := strings.TrimSpace(stderr.String()); logs != "" {
		slog.Debug("[Hook Function Log] "+script.Path+": stderr", "log", logs)
	}

	if err := runErr; err != nil {
		if hookCtx.Err() == context.DeadlineExceeded {
			return false, "", utilsmsg.New(utilsmsg.Timeout, "hook "+script.Path, "exceeded %s", configs.HOOK_TIMEOUT)
		}

		reason := strings.TrimSpace(stderr.String())
		if reason == "" {
			reason = err.Error()
		}
		return false, "", utilsmsg.New(utilsmsg.Upstream, "hook "+script.Path, "%s", reason)
	}

	var out struct {
		Success *int   `json:"success"`
		Debug   string `json:"debug"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out.Success == nil {
		return false, "", utilsmsg.New(utilsmsg.Invalid, "hook "+script.Path, `stdout must be {"success": 1|0, "debug": string, "error": string}, got %q`, raw)
	}
	if out.Error != "" {
		return false, out.Error, nil
	}
	if *out.Success == 1 {
		return true, out.Debug, nil
	}
	return false, out.Debug, nil
}

func Test(ctx context.Context, name string, input json.RawMessage) (bool, string, error) {
	script, ok := Load("")[name]
	if !ok {
		return false, "", utilsmsg.New(utilsmsg.NotFound, "hook", "%q is not installed", name)
	}
	return call(ctx, script, input)
}
