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
	maps.Copy(properties, interactive.BackgroundGoalProperties())
	maps.Copy(properties, map[string]any{
		"timeout_min": map[string]any{
			"type":        "integer",
			"description": fmt.Sprintf("Minutes before the command is killed and reported as timed out, 1-%d. Set it near the expected run time so a hung command fails early instead of holding the task for the full window; a watcher is stopped here and reports its output so far.", configs.MAX_BACKGROUND_TIMEOUT_MIN),
			"minimum":     1,
			"maximum":     configs.MAX_BACKGROUND_TIMEOUT_MIN,
			"default":     configs.MAX_BACKGROUND_TIMEOUT_MIN,
		},
		"keywords": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "Case-insensitive words that flag a new output line as a possible problem; a flagged stretch is reported with the recent output while the command keeps running. A non-empty list replaces the default set entirely, so pass one only when the tool reports failures in other words, e.g. ['warn', 'denied'].",
			"default":     configs.WATCH_KEYWORDS,
		},
	})

	toolregister.Regist(toolregister.Def{
		Name:        "background_command",
		SystemUse:   false,
		AlwaysAllow: false,
		Concurrent:  false,
		Description: fmt.Sprintf(`Starts one binary in the work directory and returns a task id at once; its output is reported when it exits, and also while it keeps running whenever a new line hits an error keyword, with the recent output to explain. Background legs started in the same turn form one group: while this task is open each report arrives here; once it has ended, the group runs objective + next as its own task when all of its legs finish. argv, network, write_paths and sandbox follow the run_command rules.
Use for a watcher (--watch, chokidar, npm run sass/build/dev), which only runs here; a job very likely over %d minutes whose output does not decide your next tool call (full build, large install or download, long data job); "run it in the background" / "let me know when it finishes". When the user's ask continues after the job ("then deploy", "unpack and summarize it"), put that action in next. Start every leg of a group in the same turn so they share one objective and next. At most %d background tasks run per session, background_subagents included.
Anything shorter, or a test or build whose failures you fix next → run_command. "How far along is it?" on a running task → read_files the log path its start result gave; never guess from missing reports.`, configs.BACKGROUND_THRESHOLD_MIN, configs.MAX_BACKGROUND_TASKS),
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
				Objective  string   `json:"objective"`
				Next       string   `json:"next"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			return runCommand(ctx, e, params.Argv, params.WritePaths, params.Network, params.unsandboxed(), &backgroundSpec{
				timeoutMin: params.TimeoutMin,
				watch:      newWatch(params.Keywords),
				goal:       interactive.BackgroundGoal{Objective: params.Objective, Next: params.Next},
			})
		},
	})
}

type backgroundSpec struct {
	timeoutMin int
	watch      watchSpec
	goal       interactive.BackgroundGoal
}

func startBackgroundCommand(ctx context.Context, e *tooltypes.Executor, argv, binds []string, sandboxOpt *go_pkg_sandbox.Option, spec *backgroundSpec) (string, error) {
	timeoutMin, watch := spec.timeoutMin, &spec.watch
	if timeoutMin <= 0 || timeoutMin > configs.MAX_BACKGROUND_TIMEOUT_MIN {
		timeoutMin = configs.MAX_BACKGROUND_TIMEOUT_MIN
	}
	description := go_pkg_utils.TruncateString(strings.Join(argv, " "), 48)
	id, err := interactive.StartBackground(ctx, e.SessionID, e.PendingTask, e.BackgroundGroup, e.WorkDir, spec.goal, "cmd", description, func(ctx context.Context, report func(string), log io.Writer) string {
		ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMin)*time.Minute)
		defer cancel()

		cmd, err := go_pkg_sandbox.Wrap(ctx, argv[0], argv[1:], e.WorkDir, sandboxOpt)
		if err != nil {
			return "failed: " + utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_sandbox.Wrap", "", err).Error()
		}
		relaySudoCancel(cmd, argv)
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
	return fmt.Sprintf("background command %s started. Keep working; its output arrives in this task while it is open, including output lines that hit an error keyword while the command keeps running. If this task ends first, the group runs its objective and next step on its own once every leg finishes. Live output is appended to %s until the command ends (then the file is removed): when asked how far it got, read its tail with read_files before answering.", id, filesystem.BackgroundLogPath(e.SessionID, id)), nil
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
