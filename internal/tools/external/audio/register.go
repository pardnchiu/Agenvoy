package audio

import (
	"context"
	"encoding/json"
	"time"

	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func Register() {
	InstallTranscriber()

	toolregister.Regist(toolregister.Def{
		Name:        "generate_audio",
		SystemUse:   false,
		AlwaysAllow: false,
		Concurrent:  false,
		Timeout:     5 * time.Minute,
		Description: `[system-default] Speaks text with the configured text-to-speech model and writes the audio to disk.
Use for "read this aloud", "say this", "turn this into a voice file", "make an audio clip of this".
Transcribing an existing recording → read_files; drawing an image → generate_image.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{
					"type":        "string",
					"description": "Exactly the words to speak, written as they should be read aloud, not a description of them.",
				},
				"voice": map[string]any{
					"type":        "string",
					"description": "Provider voice name, e.g. alloy for openai, Kore for gemini; blank uses the provider default.",
					"default":     "",
				},
				"output_file": map[string]any{
					"type":        "string",
					"description": "File name, e.g. 'greeting'; always saved in the output directory: any directory part is dropped and the extension is replaced by the format the model returns. Blank auto-names it audio-<timestamp>.",
					"default":     "",
				},
			},
			"required": []string{"text"},
		},
		Handler: func(ctx context.Context, e *tooltypes.Executor, args json.RawMessage) (string, error) {
			var params struct {
				Text       string `json:"text"`
				Voice      string `json:"voice"`
				OutputFile string `json:"output_file"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return "", utilsmsg.Wrap(utilsmsg.Invalid, "json.Unmarshal", "", err)
			}
			return Generate(ctx, Request{
				Text:       params.Text,
				Voice:      params.Voice,
				OutputFile: params.OutputFile,
			})
		},
	})
}
