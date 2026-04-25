package gemini

type Output struct {
	Candidates []struct {
		Content       Content `json:"content"`
		FinishReason  string  `json:"finishReason,omitempty"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings,omitempty"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		TotalTokenCount         int `json:"totalTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
	} `json:"usageMetadata,omitempty"`
}

type Content struct {
	Parts []Part `json:"parts"`
	Role  string `json:"role"`
}

type Part struct {
	Text             string            `json:"text,omitempty"`
	ThoughtSignature string            `json:"thoughtSignature,omitempty"`
	InlineData       *InlineData       `json:"inlineData,omitempty"`
	FunctionCall     *FunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *FunctionResponse `json:"functionResponse,omitempty"`
}

type InlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type FunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type FunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}
