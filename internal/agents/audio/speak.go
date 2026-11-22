// * last modified: 26/11/18
package audio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var mimeExt = map[string]string{
	"audio/ogg":   ".ogg",
	"audio/opus":  ".ogg",
	"audio/wav":   ".wav",
	"audio/x-wav": ".wav",
	"audio/mpeg":  ".mp3",
	"audio/mp3":   ".mp3",
	"audio/aac":   ".aac",
	"audio/flac":  ".flac",
}

type TTSRequest struct {
	Text       string
	Voice      string
	OutputFile string
	WorkDir    string
}

func Speak(ctx context.Context, req TTSRequest) (string, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "text", "empty; provide the content to convert to speech")
	}

	result, modelName, err := speakSend(ctx, text, go_llm_router.TTSOptions{
		Voice: strings.TrimSpace(req.Voice),
		// NOTE: OpenAI TTS 指定 ogg/opus (針對 tg / dc 的 bubble player) 其他的自動判斷 (純傳檔案)
		Format: "opus",
	})
	if err != nil {
		return "", err
	}
	if len(result.Audio) == 0 {
		return "", utilsmsg.New(utilsmsg.Upstream, modelName, "returned empty audio; retry once, then ask the user to switch the text-to-speech model")
	}

	path := outputPath(req, result.MimeType)
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(path), true); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, result.Audio, 0644); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "os.WriteFile", path, err)
	}
	return fmt.Sprintf("saved: %s\nmodel: %s\nbytes: %d", path, modelName, len(result.Audio)), nil
}

func speakSend(ctx context.Context, text string, opts go_llm_router.TTSOptions) (*go_llm_router.TTSResult, string, error) {
	name := Model(TTS)
	if name == "" {
		return nil, "", utilsmsg.New(utilsmsg.Unavailable, "text-to-speech model", "not configured; ask the user to select one")
	}

	agent, err := new(ctx, name)
	if err != nil {
		return nil, name, err
	}

	// NOTE: 限制為 TTSAgent, 防呆
	tts, ok := agent.(go_llm_router.TTSAgent)
	if !ok {
		return nil, name, utilsmsg.New(utilsmsg.Invalid, name, "configured but not a text-to-speech model; ask the user to reconfigure it")
	}

	result, err := tts.Speak(ctx, text, opts)
	if err != nil {
		return nil, name, utilsmsg.Wrap(utilsmsg.Upstream, "Speak", name, err)
	}
	return result, name, nil
}

func outputPath(req TTSRequest, mime string) string {
	ext := audioExt(mime)

	target := strings.TrimSpace(req.OutputFile)
	if target == "" {
		return filepath.Join(filesystem.OutputDir(), "audio-"+time.Now().Format("20060102-150405")+ext)
	}
	target = strings.TrimSuffix(target, filepath.Ext(target)) + ext
	if filepath.IsAbs(target) {
		return target
	}
	if dir := strings.TrimSpace(req.WorkDir); dir != "" {
		return filepath.Join(dir, target)
	}
	return filepath.Join(filesystem.OutputDir(), target)
}

func audioExt(mime string) string {
	base, _, _ := strings.Cut(mime, ";")
	if ext, ok := mimeExt[strings.ToLower(strings.TrimSpace(base))]; ok {
		return ext
	}
	return ".wav"
}
