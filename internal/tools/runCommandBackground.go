package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"strings"
	"sync"
	"time"

	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registRunCommandBackground() {
	properties := commandProperties()
	maps.Copy(properties, map[string]any{
		"timeout_min": map[string]any{
			"type":        "integer",
			"description": fmt.Sprintf("Minutes before the command is killed and reported as timed out, 1-%d. Set it near the expected run time so a hung command fails early instead of holding the task for the full window; a watcher stops here with its output so far.", configs.MAX_BACKGROUND_TIMEOUT_MIN),
			"minimum":     1,
			"maximum":     configs.MAX_BACKGROUND_TIMEOUT_MIN,
			"default":     configs.MAX_BACKGROUND_TIMEOUT_MIN,
		},
		"keywords": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "Case-insensitive words that flag a new output line as a possible problem; a flagged stretch resumes this task with the last output while the command keeps running. Replace the default only when the tool reports failures in other words, e.g. ['warn', 'denied'].",
			"default":     configs.WATCH_KEYWORDS,
		},
	})

	toolregister.Regist(toolregister.Def{
		Name:        "background_command",
		SystemUse:   false,
		AlwaysAllow: false,
		Concurrent:  false,
		Description: fmt.Sprintf(`Starts one binary in the work directory and returns a task id at once; its output resumes this task when it exits, and also whenever new output hits an error keyword while it keeps running, with the last output to explain. Sandbox, network and confirmation rules match run_command.
Use for a watcher (--watch, chokidar, npm run sass/build/dev), which only runs here, for a job very likely over %d minutes whose output does not decide your next tool call (full build, large install or download, long data job), or when the user says 跑完告訴我 / 背景跑. Apart from a watcher, use it only for the gathering that opens a request, starting every background command in one go: the resumed task sees only the objective and that output, none of this turn's tool results.
Anything shorter, or a test or build whose failures you fix next → run_command. 進行到哪了 / 跑到哪 on a running task → read_files the log path its start result gave, never guess from missing reports.`, configs.BACKGROUND_THRESHOLD_MIN),
		Parameters: map[string]any{
			"type":       "object",
			"properties": properties,
			"required":   []string{"argv"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				commandParams
				TimeoutMin int      `json:"timeout_min"`
				Keywords   []string `json:"keywords"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			watch := newWatch(params.Keywords)
			return runCommand(ctx, e, params.Argv, params.WritePaths, params.Network, params.unsandboxed(), true, params.TimeoutMin, &watch)
		},
	})
}

func startBackgroundCommand(ctx context.Context, e *tooltypes.Executor, argv, binds []string, sandboxOpt *go_pkg_sandbox.Option, timeoutMin int, watch *watchSpec) (string, error) {
	if timeoutMin <= 0 || timeoutMin > configs.MAX_BACKGROUND_TIMEOUT_MIN {
		timeoutMin = configs.MAX_BACKGROUND_TIMEOUT_MIN
	}
	description := go_pkg_utils.TruncateString(strings.Join(argv, " "), 48)
	id, err := interactive.StartBackground(ctx, e.SessionID, e.PendingTask, e.WorkDir, "cmd", description, func(ctx context.Context, report func(string), log io.Writer) string {
		ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMin)*time.Minute)
		defer cancel()

		cmd, err := go_pkg_sandbox.Wrap(ctx, argv[0], argv[1:], e.WorkDir, sandboxOpt)
		if err != nil {
			return "failed: " + utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_sandbox.Wrap", "", err).Error()
		}
		sink := &tailWriter{limit: configs.MAX_PENDING_RESULT_BYTES - tailHeadroom}
		buf := newWatchBuffer(watch.keywords)
		out := io.MultiWriter(sink, log, buf)
		done := make(chan struct{})
		watchDone := make(chan struct{})
		go func() {
			defer close(watchDone)
			watchOutput(ctx, *watch, buf, done, report)
		}()
		cmd.Stdout = out
		cmd.Stderr = out

		err = go_pkg_sandbox.Run(cmd)
		close(done)
		<-watchDone
		output := sink.text()
		switch {
		case ctx.Err() == context.DeadlineExceeded:
			return fmt.Sprintf("failed: [Timeout] %s: exceeded %d minutes\n%s", argv[0], timeoutMin, output)
		case err != nil:
			return fmt.Sprintf("failed: %s\n%s%s", err.Error(), output, sandboxWriteHint(output, binds, sandboxOpt.AllowAll))
		case strings.TrimSpace(output) == "":
			return "done: exit 0, printed nothing"
		default:
			return "done: exit 0\n" + output
		}
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("background command %s started. Keep working on this turn; if it ends first, the task stays open and resumes with this command's output as soon as it finishes, without waiting for other background tasks. Output lines that hit an error keyword also resume this task with the last output while the command keeps running. Live output is appended to %s until the command ends (then the file is removed): when asked how far it got, read its tail with read_files before answering.", id, filesystem.BackgroundLogPath(e.SessionID, id)), nil
}

const tailHeadroom = 1 << 10

type tailWriter struct {
	mu      sync.Mutex
	buf     []byte
	dropped int
	limit   int
}

func (w *tailWriter) Write(raw []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf = append(w.buf, raw...)
	if over := len(w.buf) - w.limit; over > 0 {
		w.dropped += over
		copy(w.buf, w.buf[over:])
		w.buf = w.buf[:w.limit]
	}
	return len(raw), nil
}

func (w *tailWriter) text() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.dropped == 0 {
		return string(w.buf)
	}
	return fmt.Sprintf("...[%d earlier bytes dropped]...\n", w.dropped) + strings.ToValidUTF8(string(w.buf), "")
}
