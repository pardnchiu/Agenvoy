// * last modified: 26/11/18
package image

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var mimeExt = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
}

type Request struct {
	Prompt      string
	AspectRatio string
	Size        string
	Quality     string
	Reference   string
	OutputFile  string
	WorkDir     string
}

func Generate(ctx context.Context, req Request) (string, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return "", utilsmsg.New(utilsmsg.Invalid, "prompt", "empty; describe the image to generate")
	}

	opts := go_llm_router.ImageOptions{
		AspectRatio: strings.TrimSpace(req.AspectRatio),
		Size:        strings.TrimSpace(req.Size),
		Quality:     strings.TrimSpace(req.Quality),
	}
	if ref := strings.TrimSpace(req.Reference); ref != "" {
		b64, mime, err := readReference(ref)
		if err != nil {
			return "", err
		}
		opts.RefImageB64, opts.RefMime = b64, mime
	}

	result, name, err := generateSend(ctx, prompt, opts)
	if err != nil {
		return "", err
	}
	if result == nil || result.B64 == "" {
		return "", utilsmsg.New(utilsmsg.Upstream, name, "returned empty image; retry once, then ask the user to switch the image generation model")
	}

	raw, err := base64.StdEncoding.DecodeString(result.B64)
	if err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Upstream, "base64.Decode", name, err)
	}

	path := outputPath(req, result.MimeType)
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(path), true); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "go_pkg_filesystem.CheckDir", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return "", utilsmsg.Wrap(utilsmsg.Internal, "os.WriteFile", path, err)
	}

	out := fmt.Sprintf("saved: %s\nmodel: %s\nbytes: %d", path, name, len(raw))
	if revised := strings.TrimSpace(result.Revised); revised != "" {
		out += "\nrevised prompt: " + revised
	}
	return out, nil
}

func readReference(path string) (string, string, error) {
	if !filepath.IsAbs(path) {
		return "", "", utilsmsg.New(utilsmsg.Invalid, "reference", "not an absolute path; pass the full path to the png/jpg/webp file")
	}
	if !go_pkg_filesystem_reader.Exists(path) {
		return "", "", utilsmsg.New(utilsmsg.NotFound, path, "reference image does not exist; check the path, or leave reference blank to generate from the prompt alone")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", utilsmsg.Wrap(utilsmsg.Internal, "os.ReadFile", path, err)
	}

	mime := "image/png"
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".webp":
		mime = "image/webp"
	}
	return base64.StdEncoding.EncodeToString(raw), mime, nil
}

func generateSend(ctx context.Context, prompt string, opts go_llm_router.ImageOptions) (*go_llm_router.ImageResult, string, error) {
	name := Model()
	if name == "" {
		return nil, "", utilsmsg.New(utilsmsg.Unavailable, "image generation model", "not configured; ask the user to select one")
	}
	if !strings.Contains(name, "@") {
		name += "@"
	}

	agent, err := new(ctx, name)
	if err != nil {
		return nil, name, err
	}

	img, ok := agent.(go_llm_router.ImageAgent)
	if !ok {
		return nil, name, utilsmsg.New(utilsmsg.Invalid, name, "configured but not an image generation model; ask the user to reconfigure it")
	}

	result, err := img.GenerateImage(ctx, prompt, opts)
	if err != nil {
		return nil, name, utilsmsg.Wrap(utilsmsg.Upstream, "GenerateImage", name, err)
	}
	return result, name, nil
}

func outputPath(req Request, mime string) string {
	ext, ok := mimeExt[strings.ToLower(strings.TrimSpace(mime))]
	if !ok {
		ext = ".png"
	}

	target := strings.TrimSpace(req.OutputFile)
	if target == "" {
		return filepath.Join(filesystem.OutputDir(), "image-"+time.Now().Format("20060102-150405")+ext)
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
