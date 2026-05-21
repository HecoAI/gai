package context_test

import (
	stdcontext "context"
	"strings"
	"testing"

	aicontext "github.com/lace-ai/gai/context"
)

func TestRAGSourceBudgetsDocumentsInsideGroup(t *testing.T) {
	t.Parallel()

	store := &fakeRAGStore{
		docs: []aicontext.Document{
			{ID: 1, Content: "one two"},
			{ID: 2, Content: "three four five"},
		},
	}
	parts, err := aicontext.RAG(store, 2, func(ctx stdcontext.Context, view aicontext.PromptView) (string, error) {
		return "query", nil
	}).BuildParts(stdcontext.Background(), testPromptView{}, aicontext.SourceBudget{
		Tokenizer:             whitespaceTokenizer{},
		MaxTokens:             4,
		RemainingPromptTokens: 4,
	})
	if err != nil {
		t.Fatalf("BuildParts failed: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("expected one grouped RAG part, got %d", len(parts))
	}
	if got := len(parts[0].Children); got != 1 {
		t.Fatalf("expected one fitting document child, got %d", got)
	}
	if parts[0].Children[0].ID != "rag-doc-0-1" {
		t.Fatalf("unexpected child id: %+v", parts[0].Children[0])
	}
}

func TestXMLRendererRendersPartGroupAsSinglePart(t *testing.T) {
	t.Parallel()

	group := aicontext.NewPartGroup("rag", []aicontext.Part{
		aicontext.NewPart("doc-1", "document one"),
		aicontext.NewPart("doc-2", "document two"),
	})
	rendered := aicontext.XMLRenderer{}.Render(aicontext.SectionContext, []aicontext.Part{group})
	if strings.Count(rendered, "<part ") != 1 {
		t.Fatalf("expected one outer part, got %q", rendered)
	}
	assertContainsAll(t, rendered, "rendered group", `<item id="doc-1">`, `<item id="doc-2">`)
}

type fakeRAGStore struct {
	docs []aicontext.Document
}

func (s *fakeRAGStore) GetRelevantDocuments(query string, limit int) ([]aicontext.Document, error) {
	if limit > 0 && limit < len(s.docs) {
		return s.docs[:limit], nil
	}
	return s.docs, nil
}

func (s *fakeRAGStore) AddDocument(content string) (int, error) {
	s.docs = append(s.docs, aicontext.Document{ID: len(s.docs) + 1, Content: content})
	return len(s.docs), nil
}
