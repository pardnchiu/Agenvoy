package main

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	agentstypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/app"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

func prompt(args []string) {
	os.Exit(runPrompt(args))
}

func runPrompt(args []string) int {
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
	skipSandbox := fs.Bool("skip-sandbox", false, "DANGEROUS: run outside the sandbox with paths outside $HOME, sudo and sandbox: false, all without confirmation")
	_ = fs.Parse(args)
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "[Invalid] args: unexpected %q; quote the prompt after -p\n", strings.Join(fs.Args(), " "))
		return 2
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
		return 2
	}

	dir, err := filepath.Abs(*workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Invalid] --work-dir: %v\n", err)
		return 2
	}

	if err := filesystem.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "filesystem.Init: %v\n", err)
		return 1
	}
	if err := filesystem.LoadRuntime(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\nfix %s, or run agen to reset it\n", err, filesystem.ConfigPath)
		return 1
	}
	closeLocal, err := openLocal()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	defer closeLocal()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	_, events, err := app.Send(ctx, app.SendInput{
		Content:     text,
		Model:       model,
		Reasoning:   reasoning,
		WorkDir:     dir,
		EnableEdit:  allowEdit,
		SkipSandbox: *skipSandbox,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	if !*stream {
		var sb strings.Builder
		for event := range events {
			switch event.Type {
			case agentstypes.EventText:
				if sb.Len() > 0 {
					sb.WriteByte('\n')
				}
				sb.WriteString(event.Text)
			case agentstypes.EventCanceled:
				fmt.Fprintln(os.Stderr, "[Canceled] agen -p: canceled")
				return 130
			case agentstypes.EventError:
				fmt.Fprintln(os.Stderr, cmp.Or(event.Text, errText(event.Err)))
				return 1
			case agentstypes.EventDone:
				fmt.Println(sb.String())
				return 0
			}
		}
		fmt.Fprintln(os.Stderr, "[Internal] agen -p: event stream closed before done")
		return 1
	}

	midLine := false
	for event := range events {
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
			fmt.Fprintln(os.Stderr, cmp.Or(event.Text, errText(event.Err)))
			return 1
		case agentstypes.EventCanceled:
			if midLine {
				fmt.Println()
			}
			fmt.Fprintln(os.Stderr, "[Canceled] agen -p: canceled")
			return 130
		case agentstypes.EventDone:
			if midLine {
				fmt.Println()
			}
			return 0
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
		fmt.Fprintln(os.Stderr, "[Canceled] agen -p: canceled")
		return 130
	}
	fmt.Fprintln(os.Stderr, "[Internal] agen -p: event stream closed before done")
	return 1
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
