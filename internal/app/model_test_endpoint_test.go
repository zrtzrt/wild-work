package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"wild-work/internal/config"
	"wild-work/internal/provider"
)

type dummyHandler struct {
	fn func(w http.ResponseWriter, r *http.Request)
}

func (d *dummyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.fn(w, r)
}

func TestModelTestAPI(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Default()
	cfg.StateFile = tmpDir + "/state.json"
	cfgPath := tmpDir + "/config.json"
	_ = config.Save(cfg, cfgPath)

	a, err := New(Options{
		ConfigPath: cfgPath,
		Config:     cfg,
		Runtimes:   map[provider.Kind]*Runtime{},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer a.Close()

	// 1. 无 handler 时测试
	resNoH := a.TestModel(t.Context(), "test/m1")
	if resNoH.OK || resNoH.Error != "服务未就绪" {
		t.Errorf("expected error '服务未就绪', got %+v", resNoH)
	}

	// 2. 注入 mock handler
	a.SetRootHandler(&dummyHandler{
		fn: func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/chat/completions" {
				http.NotFound(w, r)
				return
			}
			var body struct {
				Model string `json:"model"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Model == "good/model" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Hello from model"}}]}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream timeout"}}`))
		},
	})

	mux := http.NewServeMux()
	a.HandleAPI(mux)

	// 3. 单模型测试 - 成功分支
	{
		body, _ := json.Marshal(map[string]any{"model": "good/model"})
		req := httptest.NewRequest("POST", "/api/models/test", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, body: %s", rec.Code, rec.Body.String())
		}
		var out ModelTestResult
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !out.OK || out.Status != 200 || out.Reply != "Hello from model" {
			t.Errorf("unexpected output: %+v", out)
		}
	}

	// 4. 单模型测试 - 失败分支
	{
		body, _ := json.Marshal(map[string]any{"model": "bad/model"})
		req := httptest.NewRequest("POST", "/api/models/test", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, body: %s", rec.Code, rec.Body.String())
		}
		var out ModelTestResult
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if out.OK || out.Status != 502 || out.Error != "upstream timeout" {
			t.Errorf("unexpected output: %+v", out)
		}
	}

	// 5. 批量测试
	{
		body, _ := json.Marshal(map[string]any{"models": []string{"good/model", "bad/model"}})
		req := httptest.NewRequest("POST", "/api/models/test", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, body: %s", rec.Code, rec.Body.String())
		}
		var out struct {
			OK      bool              `json:"ok"`
			Results []ModelTestResult `json:"results"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !out.OK || len(out.Results) != 2 || !out.Results[0].OK || out.Results[1].OK {
			t.Errorf("unexpected batch output: %+v", out)
		}
	}

	// 6. 空参数报错
	{
		body, _ := json.Marshal(map[string]any{})
		req := httptest.NewRequest("POST", "/api/models/test", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty request, got %d", rec.Code)
		}
	}
}
