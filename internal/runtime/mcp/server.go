package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

const protocolVersion = "2024-11-05"

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
}

type toolContent struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type toolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

var emptySchema = json.RawMessage(`{"type":"object","properties":{}}`)

type Server struct {
	readMu  sync.RWMutex
	writeMu sync.Mutex
	stdout  io.Writer
	toolBox *toolbox
}

func NewServer() *Server {
	server := &Server{stdout: os.Stdout}
	server.toolBox = scanTools()
	return server
}

func (s *Server) Run(ctx context.Context) error {
	s.watch(ctx)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 4<<20), 4<<20)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			s.error(nil, -32700, "parse error")
			continue
		}

		s.handle(ctx, &req)
	}

	if err := scanner.Err(); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "stdin scanner", "", err)
	}
	return nil
}

func (s *Server) handle(ctx context.Context, req *request) {
	switch req.Method {
	case "initialize":
		s.handleInit(req)
	case "notifications/initialized", "notifications/cancelled":
		// pass
	case "tools/list":
		s.handleList(req)
	case "tools/call":
		s.handleCall(ctx, req)
	case "ping":
		s.send(req.ID, json.RawMessage(`{}`))
	default:
		if req.ID != nil {
			s.error(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
		}
	}
}

func (s *Server) handleInit(req *request) {
	raw, _ := json.Marshal(map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{"listChanged": true},
		},
		"serverInfo": map[string]any{
			"name":    "agenvoy",
			"version": "0.1.0",
		},
		"instructions": `This server provides tools for live data, computation, and API access.

MANDATORY: When the user asks for anything requiring live external data (weather, prices, currency, geocoding, etc.),
you MUST use tools from this server or a public API found through public_api_list — do NOT fall back to web search or browsing.
Search existing tools first (script_*, ext_*). If no tool matches, call tool_generate_guide: it starts from public_api_list,
calls a fitting API directly, and builds a script tool only when none fits or a user-supplied API needs a key: edit_tool(mode=write) → test_tool → call it.
Never answer with data not obtained through these tools or the API they point to.`,
	})
	s.send(req.ID, raw)
}

func (s *Server) handleList(req *request) {
	toolBox := scanTools()

	s.readMu.Lock()
	s.toolBox = toolBox
	s.readMu.Unlock()

	raw, _ := json.Marshal(map[string]any{"tools": toolBox.tools})
	s.send(req.ID, raw)
}

func (s *Server) handleCall(ctx context.Context, req *request) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.error(req.ID, -32602, "invalid params")
		return
	}

	s.readMu.RLock()
	toolBox := s.toolBox
	s.readMu.RUnlock()

	result, err := toolBox.dispatch(ctx, params.Name, params.Arguments)
	if err != nil {
		raw, _ := json.Marshal(toolResult{
			Content: []toolContent{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
		s.send(req.ID, raw)
		return
	}

	raw, _ := json.Marshal(toolResult{
		Content: []toolContent{{Type: "text", Text: result}},
	})
	s.send(req.ID, raw)
}

func (s *Server) send(id *int64, result json.RawMessage) {
	s.write(response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) error(id *int64, code int, msg string) {
	s.write(response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
}

func (s *Server) write(v any) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	raw, err := json.Marshal(v)
	if err != nil {
		slog.Error("json.Marshal",
			slog.String("error", err.Error()))
		return
	}
	raw = append(raw, '\n')
	if _, err := s.stdout.Write(raw); err != nil {
		slog.Error("stdout.Write",
			slog.String("error", err.Error()))
	}
}
