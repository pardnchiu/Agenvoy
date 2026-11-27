package utils

import (
	"encoding/json"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var networkErrors = func() []string {
	var list []string
	if err := json.Unmarshal(configs.NetworkError, &list); err != nil {
		panic(utilsmsg.Wrap(utilsmsg.Internal, "json: Unmarshal", "network_error.json", err))
	}
	return list
}()

func IsNetworkError(output string) bool {
	output = strings.ToLower(output)
	for _, one := range networkErrors {
		if strings.Contains(output, one) {
			return true
		}
	}
	return false
}
