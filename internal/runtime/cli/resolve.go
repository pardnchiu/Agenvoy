package cli

import (
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/session"
)

func ResolveSession() (string, error) {
	chosen := pickSession("Select session")
	if chosen == pickSessionNew {
		id, err := session.CreateSession("cli-")
		if err != nil {
			return "", fmt.Errorf("session.CreateSession: %w", err)
		}
		return id, nil
	}
	return chosen, nil
}
