package apiAdapter

import "encoding/json"

type APIDocumentData struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Endpoint    struct {
		URL         string            `json:"url"`
		Method      string            `json:"method"`
		ContentType string            `json:"content_type"`
		Headers     map[string]string `json:"headers,omitempty"`
		Query       map[string]string `json:"query,omitempty"`
		Timeout     int               `json:"timeout,omitempty"`
	} `json:"endpoint"`
	Auth       *APIDocumentAuthData `json:"auth,omitempty"`
	Parameters map[string]struct {
		Type        string `json:"type"`
		Description string `json:"description"`
		Required    bool   `json:"required"`
		Default     any    `json:"default,omitempty"`
		Enum        []any  `json:"enum,omitempty"`
	} `json:"parameters"`
	Response struct {
		Format string `json:"format"`
	} `json:"response"`
}

type APIDocumentAuthData struct {
	Type   string `json:"type"`
	Header string `json:"header"`
	Env    string `json:"env"`
}

func (d *APIDocumentData) translate() map[string]any {
	props := make(map[string]any, len(d.Parameters))
	required := []string{}

	for name, schema := range d.Parameters {
		prop := map[string]any{
			"type":        schema.Type,
			"description": schema.Description,
		}
		if len(schema.Enum) > 0 {
			prop["enum"] = schema.Enum
		}
		if schema.Default != nil {
			prop["default"] = schema.Default
		}
		props[name] = prop

		if schema.Required {
			required = append(required, name)
		}
	}

	params := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		params["required"] = required
	}

	rawParams, err := json.Marshal(params)
	if err != nil {
		rawParams = []byte("{}")
	}

	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "api_" + d.Name,
			"description": d.Description,
			"parameters":  json.RawMessage(rawParams),
		},
	}
}
