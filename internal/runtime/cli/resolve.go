package cli

import (
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/session"
)

func ResolveSession() (string, error) {
	sessions := listSessions()
	switch len(sessions) {
	case 0:
		id, err := session.CreateSession("cli-")
		if err != nil {
			return "", fmt.Errorf("session.CreateSession: %w", err)
		}
		return id, nil
	case 1:
		return sessions[0].id, nil
	default:
		sid, ok := pickSession("Select session")
		if !ok || sid == "" {
			return "", fmt.Errorf("no session selected")
		}
		return sid, nil
	}
}
