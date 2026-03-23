package apiAdapter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (t *Translator) JSONRequest(doc *APIDocumentData, params map[string]any) (*http.Request, error) {
	apiPath := replaceParams(doc, params)

	var reader io.Reader
	if doc.Endpoint.Method != "GET" && len(params) > 0 {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("json.Marshal: %w", err)
		}
		reader = strings.NewReader(string(data))
	}

	if doc.Endpoint.Method == "GET" {
		query := url.Values{}
		for k, v := range doc.Endpoint.Query {
			query.Set(k, v)
		}
		for k, v := range params {
			query.Set(k, fmt.Sprintf("%v", v))
		}
		if len(query) > 0 {
			apiPath = apiPath + "?" + query.Encode()
		}
	}

	req, err := http.NewRequest(doc.Endpoint.Method, apiPath, reader)
	if err != nil {
		return nil, err
	}
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (t *Translator) FormDataRequest(doc *APIDocumentData, params map[string]any) (*http.Request, error) {
	apiPath := replaceParams(doc, params)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, fmt.Sprintf("%v", v))
	}

	req, err := http.NewRequest(doc.Endpoint.Method, apiPath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

// * repalce {key} with real value
func replaceParams(doc *APIDocumentData, params map[string]any) string {
	apiPath := doc.Endpoint.URL

	for k, v := range params {
		placeholder := "{" + k + "}"
		if strings.Contains(apiPath, placeholder) {
			val := fmt.Sprintf("%v", v)
			if val == "" {
				delete(params, k)
				continue
			}
			apiPath = strings.ReplaceAll(apiPath, placeholder, url.PathEscape(val))
			delete(params, k)
		}
	}

	return trimUnused(apiPath)
}

// * remove {key} unused
func trimUnused(path string) string {
	for strings.Contains(path, "{") {
		start := strings.Index(path, "{")
		end := strings.Index(path, "}")
		if end < start {
			break
		}

		newStart := start
		if newStart > 0 && path[newStart-1] == '/' {
			newStart--
		}
		path = path[:newStart] + path[end+1:]
	}
	return path
}
