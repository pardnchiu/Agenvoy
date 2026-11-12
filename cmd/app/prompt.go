package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	agentstypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/app"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func prompt(args []string) {
	fs := flag.NewFlagSet("agen", flag.ExitOnError)
	content := fs.String("p", "", "prompt to send; read from stdin when empty")
	var model, reasoning string
	fs.StringVar(&model, "model", "", "model name (default: auto)")
	fs.StringVar(&model, "m", "", "alias of --model")
	fs.StringVar(&reasoning, "reasoning", "", "reasoning level (default: medium)")
	fs.StringVar(&reasoning, "r", "", "alias of --reasoning")
	stream := fs.Bool("stream", false, "print the reply as it streams")
	workDir := fs.String("work-dir", "", "working directory (default: current directory)")
	allowEdit := fs.Bool("allow-edit", false, "allow writes and non-read-only commands")
	skipSandbox := fs.Bool("skip-sandbox", false, "allow paths outside $HOME, sudo and sandbox: false")
	_ = fs.Parse(args)
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "[Invalid] args: unexpected %q; quote the prompt after -p\n", strings.Join(fs.Args(), " "))
		os.Exit(2)
	}

	text := strings.TrimSpace(*content)
	if text == "" {
		if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice == 0 {
			raw, _ := io.ReadAll(os.Stdin)
			text = strings.TrimSpace(string(raw))
		}
	}
	if text == "" {
		fmt.Fprintln(os.Stderr, "[Invalid] -p: required")
		os.Exit(2)
	}

	dir, err := filepath.Abs(*workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Invalid] --work-dir: %v\n", err)
		os.Exit(2)
	}

	if err := filesystem.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "filesystem.Init: %v\n", err)
		os.Exit(1)
	}
	if !runtime.IsCurrent() {
		if err := app.SpawnDaemon(); err != nil {
			fmt.Fprintf(os.Stderr, "app.SpawnDaemon: %v\n", err)
			os.Exit(1)
		}
	}
	addr := "127.0.0.1:" + filesystem.Port
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			fmt.Fprintf(os.Stderr, "[Timeout] daemon: %s not listening within 30s\n", addr)
			os.Exit(1)
		}
	}

	body, _ := json.Marshal(map[string]any{
		"content":        text,
		"stream":         *stream,
		"model":          model,
		"reasoning":      reasoning,
		"work_dir":       dir,
		"enable_edit":    *allowEdit,
		"enable_sandbox": !*skipSandbox,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+addr+"/v1/send", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "http.NewRequest: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "[Canceled] /v1/send: canceled")
			os.Exit(130)
		}
		fmt.Fprintf(os.Stderr, "[Unavailable] daemon: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		var res struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &res) == nil && res.Error != "" {
			fmt.Fprintln(os.Stderr, res.Error)
		} else {
			fmt.Fprintf(os.Stderr, "[Upstream] /v1/send: %d %s\n", resp.StatusCode, strings.TrimSpace(string(raw)))
		}
		os.Exit(1)
	}

	if !*stream {
		var res struct {
			Text     string `json:"text"`
			Canceled bool   `json:"canceled"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			if ctx.Err() != nil {
				fmt.Fprintln(os.Stderr, "[Canceled] /v1/send: canceled")
				os.Exit(130)
			}
			fmt.Fprintf(os.Stderr, "[Upstream] /v1/send: %v\n", err)
			os.Exit(1)
		}
		if res.Canceled {
			fmt.Fprintln(os.Stderr, "[Canceled] /v1/send: canceled")
			os.Exit(130)
		}
		fmt.Println(res.Text)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64<<10), 16<<20)
	midLine := false
	for scanner.Scan() {
		data, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if !ok {
			continue
		}
		var event agentstypes.Event
		if json.Unmarshal([]byte(data), &event) != nil {
			continue
		}
		trace := ""
		switch event.Type {
		case agentstypes.EventAgentResult:
			if event.Source == "" && event.Model != "" {
				trace = "⏵ Model(" + event.Model + ")"
			}
		case agentstypes.EventTextDelta:
			if event.Source == "" {
				fmt.Print(event.Text)
				midLine = !strings.HasSuffix(event.Text, "\n")
			}
			continue
		case agentstypes.EventTextDone:
			if event.Source == "" && midLine {
				fmt.Println()
				midLine = false
			}
			continue
		case agentstypes.EventError:
			if midLine {
				fmt.Println()
			}
			fmt.Fprintln(os.Stderr, event.Text)
			os.Exit(1)
		case agentstypes.EventCanceled:
			if midLine {
				fmt.Println()
			}
			fmt.Fprintln(os.Stderr, "[Canceled] /v1/send: canceled")
			os.Exit(130)
		case agentstypes.EventDone:
			if midLine {
				fmt.Println()
			}
			return
		case agentstypes.EventToolCall:
			if label := strings.Join(strings.Fields(utils.FormatToolEvent(event.ToolName, event.ToolArgs)), " "); label != "" {
				trace = "⏵ " + label
			}
		case agentstypes.EventReasoning:
			if text := strings.TrimSpace(strings.NewReplacer("**", "", "__", "", "`", "").Replace(event.Text)); text != "" {
				trace = "✻ " + strings.Join(strings.Fields(text), " ")
			}
		case agentstypes.EventToolSkipped:
			trace = "⏵ skipped " + event.ToolName
		default:
			trace = utils.FormatTraceLine(event)
		}
		if trace == "" {
			continue
		}
		if event.Source != "" {
			trace = "[" + event.Source + "] " + trace
		}
		if midLine {
			fmt.Println()
			midLine = false
		}
		fmt.Fprintln(os.Stderr, trace)
	}
	if midLine {
		fmt.Println()
	}
	if ctx.Err() != nil {
		fmt.Fprintln(os.Stderr, "[Canceled] /v1/send: canceled")
		os.Exit(130)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[Upstream] /v1/send: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "[Upstream] /v1/send: stream closed before done")
	os.Exit(1)
}
