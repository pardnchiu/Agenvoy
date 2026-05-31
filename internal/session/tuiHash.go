package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"
)

const (
	DefaultTUIHash = "--------"
)

var (
	TUIHash atomic.Pointer[string]
)

func GetTUIHash() string {
	if p := TUIHash.Load(); p != nil {
		return *p
	}
	return DefaultTUIHash
}

func SetTUIHash() {
	newHash := tuiHash()
	if len(newHash) != 8 {
		newHash = DefaultTUIHash
	}
	TUIHash.Store(&newHash)
}

func tuiHash() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return DefaultTUIHash
	}
	return hex.EncodeToString(b[:])
}
