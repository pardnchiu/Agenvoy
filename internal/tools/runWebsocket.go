package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pardnchiu/go-pkg/filesystem/keychain"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var wsSecretRegex = regexp.MustCompile(`\{\{secret:([A-Za-z0-9_\-.]+)\}\}`)

var errWSClosed = errors.New("closed")

type wsParams struct {
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Subscribe string            `json:"subscribe"`
	Interval  int               `json:"interval"`
	Prompt    string            `json:"prompt"`
	Timeout   int               `json:"timeout"`
}

type wsSpec struct {
	url       string
	header    http.Header
	subscribe string
	timeout   time.Duration
	watch     watchSpec
}

func registRunWebsocket() {
	toolregister.Regist(toolregister.Def{
		Name:        "run_websocket",
		SystemUse:   false,
		AlwaysAllow: false,
		Concurrent:  false,
		Description: `Opens a WebSocket in the background and every interval seconds checks the messages that arrived against prompt; each check that meets it resumes this task with those messages while listening continues until timeout or the stream ends. Returns a task id at once.
Use for 等到 X 出現再通知我 / 盯這個 feed 直到 / tell me when on a live stream (ticker, order book, status stream, event bus).
A one-off request → http_request or fetch_page; a job at a clock time with no stream → schedules; a local command → run_command.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "ws:// or wss:// endpoint.",
				},
				"headers": map[string]any{
					"type":        "object",
					"description": "Handshake headers. Write a credential as {{secret:NAME}} and it is filled from the keychain at start, never the raw value — {\"Authorization\": \"Bearer {{secret:EXCHANGE_TOKEN}}\", \"Origin\": \"https://example.com\"}; store it with store_secret first.",
					"default":     map[string]any{},
				},
				"subscribe": map[string]any{
					"type":        "string",
					"description": "Message sent after every connect, reconnects included, for services that stream only after a subscribe request — '{\"op\":\"subscribe\",\"args\":[\"tickers.BTCUSDT\"]}'. Empty when the url itself selects the stream (wss://stream.binance.com/ws/btcusdt@trade).",
					"default":     "",
				},
				"interval": map[string]any{
					"type":        "integer",
					"description": fmt.Sprintf("Seconds between checks, at least %d. Each check judges only the messages that arrived since the previous one; a window with no messages is skipped. After a report, checks pause for %d seconds, and messages that only repeat the reported situation are not reported again. Shorter catches an event sooner, longer gives the condition more context per check.", configs.WATCH_MIN_INTERVAL_SEC, configs.WATCH_REPORT_COOLDOWN_SEC),
					"minimum":     configs.WATCH_MIN_INTERVAL_SEC,
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "Condition one message must meet, checked by a small classifier that sees only this interval's messages and does no arithmetic. Write it as keyword present, field equals value, or field above / below a literal number. Do the math yourself before calling: turn a percent move, spread or baseline into absolute bounds and list every bound joined with or. Examples: 'p above 182.33 or p below 175.18' (178.75 ± 2%), 'any message contains halt', 'status is not ok', 'bid below 0.9925'.",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": fmt.Sprintf("Minutes to listen after connecting before ending as timed out, 1-%d.", configs.WS_MAX_TIMEOUT_MIN),
					"minimum":     1,
					"maximum":     configs.WS_MAX_TIMEOUT_MIN,
					"default":     configs.MAX_BACKGROUND_TIMEOUT_MIN,
				},
			},
			"required": []string{"url", "interval", "prompt"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params wsParams
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			spec, err := parseWSParams(params)
			if err != nil {
				return "", err
			}

			description := go_pkg_utils.TruncateString(spec.url, 48)
			id, err := interactive.StartBackground(ctx, e.SessionID, e.PendingTask, e.WorkDir, "ws", description, func(ctx context.Context, report func(string)) string {
				return runWebsocket(ctx, spec, report)
			})
			if err != nil {
				return "", err
			}

			return fmt.Sprintf("background websocket %s started (checks every %ds, listens %d min). Keep working on this turn; every check whose messages meet the condition resumes this task with them while the stream keeps running, and the task resumes once more when listening ends.", id, int(spec.watch.interval/time.Second), int(spec.timeout/time.Minute)), nil
		},
	})
}

func parseWSParams(p wsParams) (wsSpec, error) {
	spec := wsSpec{url: strings.TrimSpace(p.URL), subscribe: strings.TrimSpace(p.Subscribe)}
	if !strings.HasPrefix(spec.url, "ws://") && !strings.HasPrefix(spec.url, "wss://") {
		return spec, utilsmsg.New(utilsmsg.Invalid, "url", "must start with ws:// or wss://: %q", spec.url)
	}
	watch, err := parseWatch(p.Interval, p.Prompt)
	if err != nil {
		return spec, err
	}
	spec.watch = watch

	timeoutMin := p.Timeout
	if timeoutMin <= 0 {
		timeoutMin = configs.MAX_BACKGROUND_TIMEOUT_MIN
	}
	spec.timeout = time.Duration(min(timeoutMin, configs.WS_MAX_TIMEOUT_MIN)) * time.Minute

	spec.header = http.Header{}
	for name, value := range p.Headers {
		if strings.TrimSpace(name) == "" {
			return spec, utilsmsg.New(utilsmsg.Invalid, "headers", "empty header name")
		}
		var missing string
		resolved := wsSecretRegex.ReplaceAllStringFunc(value, func(ref string) string {
			key := wsSecretRegex.FindStringSubmatch(ref)[1]
			secret := keychain.Get(key)
			if secret == "" && missing == "" {
				missing = key
			}
			return secret
		})
		if missing != "" {
			return spec, utilsmsg.New(utilsmsg.NotFound, "headers."+name, "secret %s is not in the keychain; store it with store_secret first", missing)
		}
		spec.header.Set(name, resolved)
	}
	return spec, nil
}

func runWebsocket(ctx context.Context, spec wsSpec, report func(string)) string {
	ctx, cancel := context.WithTimeout(ctx, spec.timeout)
	defer cancel()

	buf := &watchBuffer{limit: configs.WATCH_BUFFER_MAX_BYTES}
	var connErr error
	done := make(chan struct{})
	go func() {
		connErr = keepWSConn(ctx, spec, buf)
		close(done)
	}()

	reports, last, err := watchOutput(ctx, spec.watch, buf, done, report)
	switch {
	case err != nil:
		return "failed: " + err.Error() + "\n" + last
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return fmt.Sprintf("done: listened %d minutes, received %d messages, reported %d times\n%s", int(spec.timeout/time.Minute), buf.received(), reports, last)
	case ctx.Err() != nil:
		return "failed: [Canceled] run_websocket: stopped"
	}

	lines, dropped := buf.take()
	tail := formatWatchLines(lines, dropped)
	if tail == "" {
		tail = last
	}
	if errors.Is(connErr, errWSClosed) {
		return fmt.Sprintf("failed: [Unavailable] run_websocket: the server closed the stream (received %d messages, reported %d times)\n%s", buf.received(), reports, tail)
	}
	return "failed: " + connErr.Error() + "\n" + tail
}

func keepWSConn(ctx context.Context, spec wsSpec, buf *watchBuffer) error {
	failures := 0
	for {
		connected, err := readWSConn(ctx, spec, buf)
		if ctx.Err() != nil || errors.Is(err, errWSClosed) {
			return err
		}
		if connected {
			failures = 0
		}
		failures++
		if failures > configs.WS_RECONNECT_MAX {
			return utilsmsg.New(utilsmsg.Unavailable, "run_websocket", "gave up after %d reconnects: %v", configs.WS_RECONNECT_MAX, err)
		}

		delay := min(time.Second<<min(failures, 6), time.Minute)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func readWSConn(ctx context.Context, spec wsSpec, buf *watchBuffer) (bool, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, spec.url, spec.header)
	if err != nil {
		return false, utilsmsg.Wrap(utilsmsg.Unavailable, "websocket.DialContext", spec.url, err)
	}
	defer conn.Close()

	idle := max(2*spec.watch.interval, time.Duration(configs.WS_IDLE_MIN_SEC)*time.Second)
	extend := func() error {
		return conn.SetReadDeadline(time.Now().Add(idle))
	}
	extend()
	conn.SetPongHandler(func(string) error {
		return extend()
	})
	conn.SetPingHandler(func(data string) error {
		extend()
		err := conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(10*time.Second))
		var netErr net.Error
		if errors.Is(err, websocket.ErrCloseSent) || (errors.As(err, &netErr) && netErr.Timeout()) {
			return nil
		}
		return err
	})

	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(idle / 2)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				conn.Close()
				return
			case <-done:
				return
			case <-ticker.C:
				conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second))
			}
		}
	}()

	if spec.subscribe != "" {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(spec.subscribe)); err != nil {
			return false, utilsmsg.Wrap(utilsmsg.Unavailable, "conn.WriteMessage", "subscribe", err)
		}
	}

	conn.SetReadLimit(int64(configs.WATCH_LINE_MAX_BYTES) * 16)
	received := false
	for {
		kind, raw, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return received, errWSClosed
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return received, utilsmsg.New(utilsmsg.Timeout, "conn.ReadMessage", "no message or pong from %s for %s", spec.url, idle)
			}
			return received, utilsmsg.Wrap(utilsmsg.Unavailable, "conn.ReadMessage", spec.url, err)
		}
		extend()
		received = true
		if kind != websocket.TextMessage {
			buf.count()
			continue
		}
		buf.add(string(raw))
	}
}
