package telegram

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	go_pkg_filesystem "github.com/pardnchiu/go-pkg/filesystem"
	go_pkg_filesystem_reader "github.com/pardnchiu/go-pkg/filesystem/reader"
)

type pendingAuth struct {
	code        string
	promptMsgID int
}

var (
	pendingMu   sync.Mutex
	chatPending = make(map[int64]*pendingAuth)
)

func authFilePath() string {
	return filepath.Join(filesystem.AgenvoyDir, ".telegram")
}

func isAuthorized(chatID int64) bool {
	path := authFilePath()
	if !go_pkg_filesystem_reader.Exists(path) {
		return false
	}
	text, err := go_pkg_filesystem.ReadText(path)
	if err != nil {
		return false
	}
	target := strconv.FormatInt(chatID, 10)
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == target {
			return true
		}
	}
	return false
}

func authorizeChat(chatID int64) error {
	path := authFilePath()
	if err := go_pkg_filesystem.CheckDir(filepath.Dir(path), true); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem CheckDir: %w", err)
	}
	if err := go_pkg_filesystem.AppendText(path, strconv.FormatInt(chatID, 10)+"\n"); err != nil {
		return fmt.Errorf("github.com/pardnchiu/go-pkg/filesystem AppendText: %w", err)
	}
	return nil
}

func generateCode() (string, error) {
	num, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("rand.Reader: %w", err)
	}
	return fmt.Sprintf("%06d", num.Int64()), nil
}

func setPending(chatID int64, code string, promptMsgID int) {
	pendingMu.Lock()
	chatPending[chatID] = &pendingAuth{code: code, promptMsgID: promptMsgID}
	pendingMu.Unlock()
}

func getPending(chatID int64) (*pendingAuth, bool) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	p, ok := chatPending[chatID]
	return p, ok
}

func clearPending(chatID int64) {
	pendingMu.Lock()
	delete(chatPending, chatID)
	pendingMu.Unlock()
}
