package summary_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lace-ai/gai/agent/summary"
	"github.com/lace-ai/gai/ai"
	aicontext "github.com/lace-ai/gai/context"
	"github.com/lace-ai/gai/testutil/mocks"
)

func TestSummarizerRunsSummaryAgentThroughLoop(t *testing.T) {
	t.Parallel()

	model := &recordingModel{response: "short summary"}
	summarizer := summary.New(model)

	got, err := summarizer.Summarize(context.Background(), aicontext.SummaryRequest{
		ID:        "history",
		Text:      "long input",
		MaxTokens: 7,
	})
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}
	if got != "short summary" {
		t.Fatalf("unexpected summary: %q", got)
	}
	if model.request.MaxTokens != 7 {
		t.Fatalf("expected summary max tokens on loop request, got %d", model.request.MaxTokens)
	}
	if !strings.Contains(model.request.Prompt.System, "Summarize the provided context") {
		t.Fatalf("expected embedded summary system prompt: %q", model.request.Prompt.System)
	}
	if !strings.Contains(model.request.Prompt.Prompt, "long input") {
		t.Fatalf("expected summary input in user prompt: %q", model.request.Prompt.Prompt)
	}
}

func TestDefinitionAllowsSystemPromptOverride(t *testing.T) {
	t.Parallel()

	model := &recordingModel{response: "short summary"}
	summarizer := summary.Summarizer{
		Definition: summary.Definition(model, summary.WithSystemPrompt("custom summary system")),
	}

	_, err := summarizer.Summarize(context.Background(), aicontext.SummaryRequest{Text: "input"})
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}
	if !strings.Contains(model.request.Prompt.System, "custom summary system") {
		t.Fatalf("expected custom system prompt: %q", model.request.Prompt.System)
	}
}

type recordingModel struct {
	response string
	request  ai.AIRequest
}

func (m *recordingModel) Name() string {
	return "recording"
}

func (m *recordingModel) Generate(ctx context.Context, req ai.AIRequest) (*ai.AIResponse, error) {
	m.request = req
	return &ai.AIResponse{Text: m.response}, nil
}

func (m *recordingModel) GenerateStream(ctx context.Context, req ai.AIRequest) <-chan ai.Token {
	out := make(chan ai.Token, 1)
	go func() {
		defer close(out)
		m.request = req
		out <- ai.Token{Type: ai.TokenTypeText, Text: m.response}
	}()
	return out
}

func (m *recordingModel) Close() error {
	return nil
}

func (m *recordingModel) Tokenizer() ai.Tokenizer {
	return mocks.MockTokenizer{}
}
