package cloudflare

import agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"

type response struct {
	Result  agentTypes.Output `json:"result"`
	Success bool              `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"errors"`
}
