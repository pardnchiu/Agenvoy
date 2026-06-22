package external

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
)

func TestMain(m *testing.M) {
	filesystem.NetWhiteList = append(filesystem.NetWhiteList, "127.0.0.1")
	os.Exit(m.Run())
}

func TestCheckSSRF_Schemes(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"file:///etc/passwd", true},
		{"ftp://internal.host/data", true},
		{"https://", true},
	}
	for _, tt := range tests {
		err := checkSSRF(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("checkSSRF(%q) err=%v, wantErr=%v", tt.url, err, tt.wantErr)
		}
	}
}

func TestCheckSSRF_PrivateIPs(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"https://10.0.0.1/api", true},
		{"https://192.168.1.1/api", true},
		{"https://[::1]/api", true},
	}
	for _, tt := range tests {
		err := checkSSRF(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("checkSSRF(%q) err=%v, wantErr=%v", tt.url, err, tt.wantErr)
		}
	}
}

func TestBuildMultipart_Fields(t *testing.T) {
	body := map[string]any{
		"fields": map[string]any{
			"name": "test",
			"age":  25,
		},
	}
	buf, ct, err := buildMultipart(body)
	if err != nil {
		t.Fatalf("buildMultipart: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("empty buffer")
	}
	if ct == "" {
		t.Error("empty content type")
	}
}

func TestBuildMultipart_FileUpload(t *testing.T) {
	tmp := t.TempDir()
	fpath := filepath.Join(tmp, "test.txt")
	os.WriteFile(fpath, []byte("hello"), 0644)

	body := map[string]any{
		"files": []any{
			map[string]any{
				"name": "upload",
				"path": fpath,
			},
		},
	}
	buf, ct, err := buildMultipart(body)
	if err != nil {
		t.Fatalf("buildMultipart: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("empty buffer")
	}
	if ct == "" {
		t.Error("empty content type")
	}
}

func TestBuildMultipart_MissingFile(t *testing.T) {
	body := map[string]any{
		"files": []any{
			map[string]any{
				"name": "upload",
				"path": "/nonexistent/file.txt",
			},
		},
	}
	_, _, err := buildMultipart(body)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestBuildMultipart_InvalidFileEntry(t *testing.T) {
	body := map[string]any{
		"files": []any{
			map[string]any{
				"name": "",
				"path": "",
			},
		},
	}
	_, _, err := buildMultipart(body)
	if err == nil {
		t.Error("expected error for empty name/path")
	}
}

func TestSendHTTPRequest_GET(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	result, err := sendHTTPRequest(context.Background(), srv.URL+"/test", "GET", nil, nil, "json", 5)
	if err != nil {
		t.Fatalf("sendHTTPRequest: %v", err)
	}
	var resp httpResponse
	json.Unmarshal([]byte(result), &resp)
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if resp.Body != `{"status":"ok"}` {
		t.Errorf("body = %s", resp.Body)
	}
}

func TestSendHTTPRequest_POST_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(200)
		w.Write([]byte(`"done"`))
	}))
	defer srv.Close()

	_, err := sendHTTPRequest(context.Background(), srv.URL, "POST", nil, map[string]any{"key": "val"}, "json", 5)
	if err != nil {
		t.Fatalf("sendHTTPRequest: %v", err)
	}
}

func TestSendHTTPRequest_CustomHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "value" {
			t.Errorf("X-Custom = %q", r.Header.Get("X-Custom"))
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	_, err := sendHTTPRequest(context.Background(), srv.URL, "GET", map[string]string{"X-Custom": "value"}, nil, "", 5)
	if err != nil {
		t.Fatalf("sendHTTPRequest: %v", err)
	}
}

func TestSendHTTPRequest_InvalidMethod(t *testing.T) {
	_, err := sendHTTPRequest(context.Background(), "https://example.com", "TRACE", nil, nil, "", 5)
	if err == nil {
		t.Error("expected error for TRACE method")
	}
}

func TestSendHTTPRequest_EmptyURL(t *testing.T) {
	_, err := sendHTTPRequest(context.Background(), "", "GET", nil, nil, "", 5)
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestSendHTTPRequest_TimeoutClamp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	_, err := sendHTTPRequest(context.Background(), srv.URL, "GET", nil, nil, "", -10)
	if err != nil {
		t.Fatalf("sendHTTPRequest with negative timeout: %v", err)
	}

	_, err = sendHTTPRequest(context.Background(), srv.URL, "GET", nil, nil, "", 999)
	if err != nil {
		t.Fatalf("sendHTTPRequest with large timeout: %v", err)
	}
}

func TestSendHTTPRequest_POST_Form(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("content-type = %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	_, err := sendHTTPRequest(context.Background(), srv.URL, "POST", nil, map[string]any{"a": "b"}, "form", 5)
	if err != nil {
		t.Fatalf("sendHTTPRequest form: %v", err)
	}
}
