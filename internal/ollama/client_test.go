package ollama

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGenerateFallsBackToLlamaCPPCompletion(t *testing.T) {
	var seenCompletion bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/generate":
			http.NotFound(w, r)
		case "/completion":
			seenCompletion = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if body["prompt"] != "explain this" {
				t.Fatalf("prompt = %v", body["prompt"])
			}
			if body["model"] != "loaded-model" {
				t.Fatalf("model = %v", body["model"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"content": "llama output"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	got, err := client.Generate(t.Context(), "loaded-model", "explain this")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got != "llama output" {
		t.Fatalf("Generate() = %q", got)
	}
	if !seenCompletion {
		t.Fatal("llama.cpp completion endpoint was not called")
	}
}

func TestGenerateFallsBackToOpenAIChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/generate", "/completion":
			http.NotFound(w, r)
		case "/v1/chat/completions":
			var body struct {
				Model    string `json:"model"`
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if body.Model != "chat-model" {
				t.Fatalf("model = %q", body.Model)
			}
			if len(body.Messages) != 1 || body.Messages[0].Content != "draft it" {
				t.Fatalf("messages = %#v", body.Messages)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]string{"content": "chat output"}},
				},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	got, err := client.Generate(t.Context(), "chat-model", "draft it")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got != "chat output" {
		t.Fatalf("Generate() = %q", got)
	}
}

func TestEmbedFallsBackToLlamaCPPEmbedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/embed":
			http.NotFound(w, r)
		case "/embedding":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if body["content"] != "hello" {
				t.Fatalf("content = %v", body["content"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"embedding": []float32{1, 2, 3}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	got, err := client.Embed(t.Context(), "embedding-model", "hello")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("Embed() = %#v", got)
	}
}

func TestEmbedFallsBackToOpenAIEmbeddings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/embed", "/embedding":
			http.NotFound(w, r)
		case "/v1/embeddings":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if body["model"] != "embedding-model" {
				t.Fatalf("model = %v", body["model"])
			}
			if body["input"] != "hello" {
				t.Fatalf("input = %v", body["input"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"embedding": []float32{4, 5, 6}},
				},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	got, err := client.Embed(t.Context(), "embedding-model", "hello")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}
	if len(got) != 3 || got[0] != 4 || got[1] != 5 || got[2] != 6 {
		t.Fatalf("Embed() = %#v", got)
	}
}
