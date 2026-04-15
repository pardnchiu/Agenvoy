package store

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	toriidb "github.com/pardnchiu/ToriiDB/core/store"
)

type SetFlag = toriidb.SetFlag

const (
	SetDefault = toriidb.SetDefault
	SetNX      = toriidb.SetNX
	SetXX      = toriidb.SetXX
)

type Entry = toriidb.Entry

const (
	DBToolCache   = 0 // All tool cache
	DBSessionHist = 1 // Session conversation
	DBErrorMemory = 2 // Tool error
)

var (
	once     sync.Once
	initErr  error
	instance *toriidb.Store
)

func Init(path string) error {
	once.Do(func() {
		s, err := toriidb.New(path)
		if err != nil {
			initErr = fmt.Errorf("toriidb.New: %w", err)
			return
		}
		instance = s
	})
	return initErr
}

func Close() {
	if instance == nil {
		return
	}
	if err := instance.Close(); err != nil {
		slog.Warn("store.Close", slog.String("error", err.Error()))
	}
}

func DB(index int) *toriidb.Session {
	session := instance.Session()
	if err := session.Select(index); err != nil {
		slog.Error("store.DB.Select",
			slog.Int("index", index),
			slog.String("error", err.Error()))
	}
	return session
}

func TTL(seconds int64) *int64 {
	ts := time.Now().Unix() + seconds
	return &ts
}
