// * last modified: 26/11/18
package tools

import (
	"context"
	"encoding/json"

	agentaudio "github.com/pardnchiu/agenvoy/internal/agents/audio"
	toolregister "github.com/pardnchiu/agenvoy/internal/tools/register"
	tooltypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func registAudio() {
	toolregister.Regist(toolregister.Def{
		Name:          "generate_audio",
		SystemDefault: true,
		Description: `Speaks text with the configured text-to-speech model and saves the audio file.
Use for "read this aloud", "say this", "turn this into a voice file", "make an audio clip of this".
Transcribing an existing recording → read_files.`,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{
					"type":        "string",
					"description": "The exact words to speak as they should be read aloud, not a description of them.",
				},
				"voice": map[string]any{
					"type":        "string",
					"description": "Provider voice name, e.g. alloy for openai, Kore for gemini; blank uses the provider default.",
				},
				"output_file": map[string]any{
					"type":        "string",
					"description": "Save path: blank auto-names audio-<timestamp> in the output directory; absolute is used as-is; relative joins under the working directory; the extension always follows the returned format.",
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

			workDir := ""
			if e != nil {
				workDir = e.WorkDir
			}
			return agentaudio.Speak(ctx, agentaudio.TTSRequest{
				Text:       params.Text,
				Voice:      params.Voice,
				OutputFile: params.OutputFile,
				WorkDir:    workDir,
			})
		},
	})
}
