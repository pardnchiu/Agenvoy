package torii

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

func ModelsCacheKey(target, name string) string {
	return fmt.Sprintf("provider:models:%s:%s", target, name)
}

func DropModelsCache(ctx context.Context, name string) {
	name = strings.TrimSuffix(name, "@")
	DropKeys(ctx,
		ModelsCacheKey("list", name),
		ModelsCacheKey("image", name),
		ModelsCacheKey("decision", name),
		ModelsCacheKey("stt", name),
		ModelsCacheKey("tts", name),
	)
}

func CachedList(ctx context.Context, key string, ttl int64, fetch func() ([]string, error)) ([]string, error) {
	db := DB(DBToolCache)
	if Ready() {
		if record, ok := db.Get(ctx, key); ok {
			var list []string
			if json.Unmarshal([]byte(record.Value()), &list) == nil {
				return list, nil
			}
		}
	}

	list, err := fetch()
	if err != nil {
		return nil, err
	}

	if Ready() {
		raw, _ := json.Marshal(list)
		if err := db.Set(ctx, key, string(raw), TTL(ttl)); err != nil {
			slog.Debug("torii.CachedList", slog.String("key", key), slog.String("error", err.Error()))
		}
	}
	return list, nil
}

func DropKeys(ctx context.Context, keys ...string) {
	if !Ready() {
		return
	}
	DB(DBToolCache).Del(ctx, keys...)
}
