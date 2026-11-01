package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	go_pkg_sandbox "github.com/pardnchiu/go-pkg/sandbox"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type BackgroundDeliver func(sessionID, taskHash, workDir, id, result string)

var backgroundDeliver BackgroundDeliver

func SetBackgroundDeliver(fn BackgroundDeliver) {
	backgroundDeliver = fn
}

func startBackgroundCommand(ctx context.Context, e *tooltypes.Executor, argv, binds []string, sandboxOpt *go_pkg_sandbox.Option, timeoutMin int) (string, error) {
	if e.PendingTask == "" || backgroundDeliver == nil {
		return "", utilsmsg.New(utilsmsg.Unavailable, "background", "this request has no session task to resume; run the command without background")
	}
	if err := interactive.AcquireBackgroundSlot(e.SessionID); err != nil {
		return "", err
	}

	if timeoutMin <= 0 || timeoutMin > configs.MAX_BACKGROUND_TIMEOUT_MIN {
		timeoutMin = configs.MAX_BACKGROUND_TIMEOUT_MIN
	}

	id := "cmd_" + strings.ReplaceAll(go_pkg_utils.UUID(), "-", "")[:8]
	description := go_pkg_utils.TruncateString(strings.Join(argv, " "), 48)
	sessionID, taskHash, workDir := e.SessionID, e.PendingTask, e.WorkDir
	if err := interactive.AddBackground(sessionID, taskHash, workDir, id, description); err != nil {
		interactive.ReleaseBackgroundSlot(sessionID)
		return "", err
	}

	bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Duration(timeoutMin)*time.Minute)
	go func() {
		defer interactive.ReleaseBackgroundSlot(sessionID)
		defer cancel()
		result := "failed: background command ended without a result"
		defer func() {
			if r := recover(); r != nil {
				slog.Error("[Internal] background command: panic recovered",
					slog.String("session", sessionID),
					slog.Any("panic", r))
				result = fmt.Sprintf("failed: panic: %v", r)
			}
			backgroundDeliver(sessionID, taskHash, workDir, id, result)
		}()

		cmd, err := go_pkg_sandbox.Wrap(bgCtx, argv[0], argv[1:], workDir, sandboxOpt)
		if err != nil {
			result = "failed: " + utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_sandbox.Wrap", "", err).Error()
			return
		}
		sink := &tailWriter{limit: configs.MAX_PENDING_RESULT_BYTES - tailHeadroom}
		cmd.Stdout = sink
		cmd.Stderr = sink

		err = cmd.Run()
		output := sink.text()
		switch {
		case bgCtx.Err() == context.DeadlineExceeded:
			result = fmt.Sprintf("failed: [Timeout] %s: exceeded %d minutes\n%s", argv[0], timeoutMin, output)
		case err != nil:
			result = fmt.Sprintf("failed: %s\n%s%s", err.Error(), output, sandboxWriteHint(output, binds))
		case strings.TrimSpace(output) == "":
			result = "done: exit 0, printed nothing"
		default:
			result = "done: exit 0\n" + output
		}
	}()

	return fmt.Sprintf("background command %s started. This task pauses after this step and resumes with the command's output once every background task has finished.", id), nil
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
