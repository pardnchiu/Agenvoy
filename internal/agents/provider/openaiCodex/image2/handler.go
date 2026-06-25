package image2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	openaicodex "github.com/pardnchiu/agenvoy/internal/agents/provider/openaiCodex"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func handler(ctx context.Context, _ *toolTypes.Executor, args json.RawMessage) (string, error) {
	var p struct {
		Prompt             string `json:"prompt"`
		Size               string `json:"size"`
		Quality            string `json:"quality"`
		ReferenceImagePath string `json:"reference_image_path,omitempty"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("json.Unmarshal: %w", err)
	}
	if strings.TrimSpace(p.Prompt) == "" {
		return "prompt is required", nil
	}
	if strings.TrimSpace(p.Size) == "" {
		return "size is required — call ask_user to confirm 1024x1024 / 1024x1792 / 1792x1024 before retrying.", nil
	}
	if strings.TrimSpace(p.Quality) == "" {
		return "quality is required — call ask_user to confirm low / medium / high before retrying.", nil
	}

	cfg, err := go_pkg_filesystem.ReadJSON[exec.AgentConfig](filesystem.ConfigPath)
	if err != nil {
		return fmt.Sprintf("read config: %s", err.Error()), nil
	}
	codexModel := ""
	for _, m := range cfg.Models {
		if strings.HasPrefix(m.Name, "codex@") {
			codexModel = m.Name
			break
		}
	}
	if codexModel == "" {
		return "no codex@ model registered; run `agen model add` to authenticate Codex first.", nil
	}

	agent, err := openaicodex.New(codexModel)
	if err != nil {
		return fmt.Sprintf("codex agent init failed: %s", err.Error()), nil
	}

	opts := openaicodex.ImageOptions{
		Size:    p.Size,
		Quality: p.Quality,
	}

	if p.ReferenceImagePath != "" {
		// * os.ReadFile retained: reference image is small binary (<10 MB) and go-pkg/filesystem only exposes ReadText/ReadJSON
		raw, err := os.ReadFile(p.ReferenceImagePath)
		if err != nil {
			return fmt.Sprintf("read reference image: %s", err.Error()), nil
		}
		opts.RefImageB64 = base64.StdEncoding.EncodeToString(raw)
		switch strings.ToLower(filepath.Ext(p.ReferenceImagePath)) {
		case ".jpg", ".jpeg":
			opts.RefMime = "image/jpeg"
		case ".webp":
			opts.RefMime = "image/webp"
		default:
			opts.RefMime = "image/png"
		}
	}

	b64, revised, err := agent.GenerateImage(ctx, p.Prompt, opts)
	if err != nil {
		return fmt.Sprintf("generate_image failed: %s. If the error persists, codex OAuth may have revoked image_generation tool access.", err.Error()), nil
	}

	imgBytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Sprintf("base64 decode: %s", err.Error()), nil
	}

	downloadDir := filesystem.DownloadDir

	outPath := filepath.Join(downloadDir, fmt.Sprintf("agenvoy-img-%s.png", go_pkg_utils.UUID()))
	// * os.WriteFile retained: binary PNG bytes; go-pkg/filesystem.WriteFile only takes string content
	if err := os.WriteFile(outPath, imgBytes, 0644); err != nil {
		return fmt.Sprintf("write image: %s", err.Error()), nil
	}

	msg := ""
	if revised != "" {
		msg = "(revised: " + revised + ")\n\n"
	}
	msg += "[SEND_FILE:" + outPath + "]"
	return msg, nil
}
