package audio

import (
	"context"
	"strings"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"

	"github.com/pardnchiu/agenvoy/configs"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type STTRequest struct {
	Audio    []byte
	MimeType string
}

func Transcribe(ctx context.Context, req STTRequest) (string, error) {
	name := Model(STT)
	if name == "" {
		return "", utilsmsg.New(utilsmsg.Unavailable, "speech-to-text model", "not configured; ask the user to select one")
	}

	agent, err := new(ctx, name)
	if err != nil {
		return "", err
	}

	// NOTE: 限制為 STTAgent, 防呆
	stt, ok := agent.(go_llm_router.STTAgent)
	if !ok {
		return "", utilsmsg.New(utilsmsg.Invalid, name, "configured but not a speech-to-text model; ask the user to reconfigure it")
	}

	result, err := stt.Transcribe(ctx, req.Audio, go_llm_router.STTOptions{
		Prompt:   strings.TrimSpace(configs.TranscriptPrompt),
		MimeType: req.MimeType,
	})
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Upstream, "Transcribe", name, err)
	}
	return result.Text, nil
}
