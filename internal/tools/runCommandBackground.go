package tools

import (
	"context"
	"fmt"
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

func startBackgroundCommand(ctx context.Context, e *tooltypes.Executor, argv, binds []string, sandboxOpt *go_pkg_sandbox.Option, timeoutMin int) (string, error) {
	if timeoutMin <= 0 || timeoutMin > configs.MAX_BACKGROUND_TIMEOUT_MIN {
		timeoutMin = configs.MAX_BACKGROUND_TIMEOUT_MIN
	}
	description := go_pkg_utils.TruncateString(strings.Join(argv, " "), 48)
	id, err := interactive.StartBackground(ctx, e.SessionID, e.PendingTask, e.WorkDir, "cmd", description, func(ctx context.Context) string {
		ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMin)*time.Minute)
		defer cancel()

		cmd, err := go_pkg_sandbox.Wrap(ctx, argv[0], argv[1:], e.WorkDir, sandboxOpt)
		if err != nil {
			return "failed: " + utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_sandbox.Wrap", "", err).Error()
		}
		sink := &tailWriter{limit: configs.MAX_PENDING_RESULT_BYTES - tailHeadroom}
		cmd.Stdout = sink
		cmd.Stderr = sink

		err = go_pkg_sandbox.Run(cmd)
		output := sink.text()
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
	return fmt.Sprintf("background command %s started. Keep working on this turn; if it ends before every background task does, the task stays open and resumes with their output once the last one finishes.", id), nil
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
