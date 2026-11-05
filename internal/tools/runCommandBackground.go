package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"strconv"
	"strings"
	"sync"
	"time"

	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
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
		"interval": map[string]any{
			"type":        "integer",
			"description": fmt.Sprintf("Set together with prompt. Seconds between checks of the output printed since the previous check, at least %d; a check whose lines meet prompt resumes this task with them while the command keeps running. After a report, checks pause for %d seconds, and output that only repeats the reported situation is not reported again. Leave both out to hear back only when it exits.", configs.WATCH_MIN_INTERVAL_SEC, configs.WATCH_REPORT_COOLDOWN_SEC),
			"minimum":     configs.WATCH_MIN_INTERVAL_SEC,
		},
		"prompt": map[string]any{
			"type":        "string",
			"description": "Set together with interval. Condition one output line must meet, checked by a small classifier that sees only this interval's lines and does no arithmetic. Write it as keyword present or value above / below a literal number. Do the math yourself before calling: turn a relative threshold into absolute bounds and list every bound joined with or. Examples: 'any line contains error or failed', 'loss above 2.5 or loss is NaN', 'eta above 600'.",
		},
	})

	toolregister.Regist(toolregister.Def{
		Name:        "run_command_background",
		SystemUse:   false,
		AlwaysAllow: false,
		Concurrent:  false,
		Description: fmt.Sprintf(`Starts one binary in the work directory and returns a task id at once; its output resumes this task when it exits, and with interval + prompt also each time new output meets prompt while it keeps running. Sandbox, network and confirmation rules match run_command.
Use for a watcher (--watch, chokidar, npm run sass/build/dev), which only runs here, for a job very likely over %d minutes whose output does not decide your next tool call (full build, large install or download, long data job), or when the user says 跑完告訴我 / 背景跑. Apart from a watcher, use it only for the gathering that opens a request, starting every background command in one go: the resumed task sees only the objective and that output, none of this turn's tool results.
Anything shorter, or a test or build whose failures you fix next → run_command; a live WebSocket feed → run_websocket.`, configs.BACKGROUND_THRESHOLD_MIN),
		Parameters: map[string]any{
			"type":       "object",
			"properties": properties,
			"required":   []string{"argv"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				commandParams
				TimeoutMin int    `json:"timeout_min"`
				Interval   int    `json:"interval"`
				Prompt     string `json:"prompt"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			background := interactive.BackgroundAllowed(ctx)
			var watch *watchSpec
			if params.Interval != 0 || strings.TrimSpace(params.Prompt) != "" {
				if !background {
					return "", utilsmsg.New(utilsmsg.Invalid, "interval", "needs a session that can resume a background task; call run_command without interval and prompt")
				}
				spec, err := parseWatch(params.Interval, params.Prompt)
				if err != nil {
					return "", err
				}
				watch = &spec
			}
			return runCommand(ctx, e, params.Argv, params.WritePaths, params.Network, params.unsandboxed(), background, params.TimeoutMin, watch)
		},
	})
}

func startBackgroundCommand(ctx context.Context, e *tooltypes.Executor, argv, binds []string, sandboxOpt *go_pkg_sandbox.Option, timeoutMin int, watch *watchSpec) (string, error) {
	if timeoutMin <= 0 || timeoutMin > configs.MAX_BACKGROUND_TIMEOUT_MIN {
		timeoutMin = configs.MAX_BACKGROUND_TIMEOUT_MIN
	}
	description := go_pkg_utils.TruncateString(strings.Join(argv, " "), 48)
	id, err := interactive.StartBackground(ctx, e.SessionID, e.PendingTask, e.WorkDir, "cmd", description, func(ctx context.Context, report func(string)) string {
		ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMin)*time.Minute)
		defer cancel()

		cmd, err := go_pkg_sandbox.Wrap(ctx, argv[0], argv[1:], e.WorkDir, sandboxOpt)
		if err != nil {
			return "failed: " + utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_sandbox.Wrap", "", err).Error()
		}
		sink := &tailWriter{limit: configs.MAX_PENDING_RESULT_BYTES - tailHeadroom}
		var out io.Writer = sink
		var watchErr chan error
		done := make(chan struct{})
		if watch != nil {
			buf := &watchBuffer{limit: configs.WATCH_BUFFER_MAX_BYTES}
			out = io.MultiWriter(sink, buf)
			watchErr = make(chan error, 1)
			go func() {
				_, tail, err := watchOutput(ctx, *watch, buf, done, report)
				if err != nil {
					report("interval checks stopped: " + err.Error() + "; the command keeps running. Output not yet checked, last " + strconv.Itoa(configs.WATCH_BUFFER_MAX_BYTES>>10) + " KiB:\n" + tail)
				}
				watchErr <- err
			}()
		}
		cmd.Stdout = out
		cmd.Stderr = out

		err = go_pkg_sandbox.Run(cmd)
		close(done)
		output := sink.text()
		if watchErr != nil {
			if err := <-watchErr; err != nil {
				output = "[interval checks stopped: " + err.Error() + "]\n" + output
			}
		}
		switch {
		case ctx.Err() == context.DeadlineExceeded:
			return fmt.Sprintf("failed: [Timeout] %s: exceeded %d minutes\n%s", argv[0], timeoutMin, output)
		case err != nil:
			return fmt.Sprintf("failed: %s\n%s%s", err.Error(), output, sandboxWriteHint(output, binds))
		case strings.TrimSpace(output) == "":
			return "done: exit 0, printed nothing"
		default:
			return "done: exit 0\n" + output
		}
	})
	if err != nil {
		return "", err
	}
	text := fmt.Sprintf("background command %s started. Keep working on this turn; if it ends first, the task stays open and resumes with this command's output as soon as it finishes, without waiting for other background tasks.", id)
	if watch != nil {
		text += fmt.Sprintf(" Every %ds its new output is checked against prompt, and each check that meets it also resumes this task while the command keeps running.", int(watch.interval/time.Second))
	}
	return text, nil
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
