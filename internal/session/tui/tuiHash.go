package tuiHash

import (
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"
)

const (
	Default = "--------"
)

var (
	hash atomic.Pointer[string]
)

func New() {
	newHash := tuiHash()
	if len(newHash) != 8 {
		newHash = Default
	}
	hash.Store(&newHash)
}

func Get() string {
	if p := hash.Load(); p != nil {
		return *p
	}
	return Default
}

func tuiHash() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return Default
	}
	return hex.EncodeToString(b[:])
}
