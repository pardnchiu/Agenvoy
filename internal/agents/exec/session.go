package exec

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	agentstypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	sessionhistory "github.com/pardnchiu/agenvoy/internal/session/history"
	sessionsummary "github.com/pardnchiu/agenvoy/internal/session/summary"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func session(ctx context.Context, meta ExecuteMeta) (*agentstypes.Session, error) {
	// * step1: assemble session history
	sessionID := strings.TrimSpace(meta.SessionID)
	if sessionID == "" {
		return nil, utilsmsg.New(utilsmsg.Invalid, "session id", "required")
	}

	sessionDir := filesystem.SessionDir(sessionID)
	if !go_pkg_filesystem_reader.IsDir(sessionDir) {
		return nil, utilsmsg.New(utilsmsg.NotFound, "session", "%q", sessionID)
	}

	var maxHistory []sessionhistory.Record
	if !meta.IgnoreHistory {
		_, maxHistory = sessionhistory.Get(sessionID)
	}
	session := agentstypes.Session{
		SystemPrompts:    BuildSystemPrompts(meta.WorkDir, meta.ExtraSystemPrompt, sessionID, meta.Agent.Name()),
		ToolResults:      []go_llm_router.Message{},
		ContextHistories: sessionhistory.Messages(maxHistory),
		ToolHistories:    []go_llm_router.Message{},
	}
	if !meta.IgnoreHistory {
		session.SummaryData = sessionsummary.Load(sessionID, GetOldestMessageTime(maxHistory))
	}

	// * step2: assemble user input
	userInput := strings.TrimSpace(meta.Input)
	if userInput == "" {
		userInput = strings.TrimSpace(meta.Task)
	}

	historyInput := userInput
	if content := strings.TrimSpace(meta.HistoryContent); content != "" {
		// * not save full resume input in history
		historyInput = content
	}

	session.Sender = meta.Sender
	session.SendAt = time.Now().UnixNano()
	// * add timestamp prefix to user input, for send at tracking
	prefix := sessionhistory.Record{
		SendAt: session.SendAt,
		Sender: session.Sender,
	}.Prefix()

	session.RawHistories = append(session.RawHistories, go_llm_router.Message{
		Role:    "user",
		Content: sessionhistory.WithPrefix(prefix, historyInput),
	})
	session.UserMessage = go_llm_router.Message{
		Role:    "user",
		Content: sessionhistory.WithPrefix(prefix, buildInput(userInput, meta.ImageInputs, meta.FileInputs)),
	}
	SaveUserInputHistory(ctx, sessionID, historyInput)

	session.ID = sessionID
	return &session, nil
}

func GetOldestMessageTime(histories []sessionhistory.Record) time.Time {
	for _, record := range histories {
		if record.SendAt > 0 {
			return time.Unix(0, record.SendAt)
		}
	}
	return time.Time{}
}

func buildInput(userInput string, imageInputs []string, fileInputs []string) any {
	// * no file/image, directily return inpur
	if len(imageInputs) == 0 && len(fileInputs) == 0 {
		return userInput
	}

	parts := []go_llm_router.ContentPart{
		{Type: "text", Text: userInput},
	}

	// * if file is exist, append file content to user input
	for _, path := range fileInputs {
		content, err := go_pkg_filesystem.ReadText(path)
		if err != nil {
			continue
		}
		parts = append(parts, go_llm_router.ContentPart{
			Type: "text",
			Text: fmt.Sprintf("---\npath: %s\n---\n%s", filepath.Base(path), content),
		})
	}

	// * if image is exist, append image content to user input
	for _, path := range imageInputs {
		b64, err := convertToBase64(path)
		if err != nil {
			continue
		}
		data := "data:image/jpeg;base64," + b64
		parts = append(parts, go_llm_router.ContentPart{
			Type:     "image_url",
			ImageURL: &go_llm_router.ImageURL{URL: data, Detail: "auto"},
		})
	}
	return parts
}

func convertToBase64(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", utilsmsg.New(utilsmsg.NotFound, "image", "%s: %w", path, err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return "", utilsmsg.New(utilsmsg.Invalid, "image", "%s: decode: %w", path, err)
	}

	// * need to be use jpeg before send in claude/gemini model
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return "", utilsmsg.New(utilsmsg.Internal, "image", "%s: jpeg encode: %w", path, err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
