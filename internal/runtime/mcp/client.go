package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type Client interface {
	List(ctx context.Context) ([]Tool, error)
	Call(ctx context.Context, name string, args map[string]any) (string, error)
	Instructions() string
	Close() error
}

type SessionError struct {
	Err error
}

func (e *SessionError) Error() string {
	return e.Err.Error()
}

func (e *SessionError) Unwrap() error {
	return e.Err
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type sdkClient struct {
	session *mcpsdk.ClientSession
}

func newClient(ctx context.Context, name string, cfg ServerConfig, onToolsChanged func()) (Client, error) {
	transport, err := cfg.Expand().toTransport(name)
	if err != nil {
		return nil, utilsmsg.Trace("server", strconv.Quote(name), err)
	}

	var opts *mcpsdk.ClientOptions
	if onToolsChanged != nil {
		opts = &mcpsdk.ClientOptions{
			ToolListChangedHandler: func(context.Context, *mcpsdk.ToolListChangedRequest) {
				go onToolsChanged()
			},
		}
	}

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "agenvoy", Version: "1.0.0"}, opts)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, utilsmsg.Wrap(utilsmsg.Unavailable, "connect", strconv.Quote(name), err)
	}
	return &sdkClient{session: session}, nil
}

func (c *sdkClient) List(ctx context.Context) ([]Tool, error) {
	res, err := c.session.ListTools(ctx, nil)
	if err != nil {
		return nil, &SessionError{Err: utilsmsg.Wrap(utilsmsg.Upstream, "tools/list", "", err)}
	}

	list := make([]Tool, 0, len(res.Tools))
	for _, tool := range res.Tools {
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			continue
		}
		list = append(list, Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: raw,
		})
	}
	return list, nil
}

func (c *sdkClient) Call(ctx context.Context, name string, args map[string]any) (string, error) {
	if args == nil {
		args = map[string]any{}
	}

	res, err := c.session.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return "", &SessionError{Err: utilsmsg.Wrap(utilsmsg.Upstream, "tools/call", strconv.Quote(name), err)}
	}

	var sb strings.Builder
	var images []string
	for _, content := range res.Content {
		switch c := content.(type) {
		case *mcpsdk.TextContent:
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(c.Text)
		case *mcpsdk.ImageContent:
			images = append(images, "data:"+c.MIMEType+";base64,"+base64.StdEncoding.EncodeToString(c.Data))
		}
	}

	if !res.IsError && len(images) > 0 {
		dic := make(map[string]string, len(images)+1)
		for i, url := range images {
			dic["image_"+strconv.Itoa(i)] = url
		}
		if sb.Len() > 0 {
			dic["text"] = sb.String()
		}
		raw, err := json.Marshal(dic)
		if err != nil {
			return "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "", err)
		}
		return string(raw), nil
	}

	if res.IsError {
		if sb.Len() == 0 {
			return "", utilsmsg.New(utilsmsg.Upstream, "tools/call", "tool error")
		}
		return "", utilsmsg.New(utilsmsg.Upstream, "tools/call", "tool error: %s", sb.String())
	}
	if sb.Len() > 0 {
		return sb.String(), nil
	}

	raw, err := json.Marshal(res)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "json.Marshal", "", err)
	}
	return string(raw), nil
}

func (c *sdkClient) Instructions() string {
	init := c.session.InitializeResult()
	if init == nil {
		return ""
	}
	return strings.TrimSpace(init.Instructions)
}

func (c *sdkClient) Close() error {
	return c.session.Close()
}
