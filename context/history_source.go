package context

import (
	stdcontext "context"
	"strconv"

	"github.com/lace-ai/gai/ai"
)

type HistorySource struct {
	store      SessionStore
	id         int
	tokenLimit int
	tokenizer  ai.Tokenizer
}

func History(store SessionStore, id, tokenLimit int, tokenizer ai.Tokenizer) Source {
	return &HistorySource{
		store:      store,
		id:         id,
		tokenLimit: tokenLimit,
		tokenizer:  tokenizer,
	}
}

func (s *HistorySource) BuildParts(ctx stdcontext.Context, view PromptView) ([]Part, error) {
	if s == nil || s.store == nil {
		return nil, ErrSessionStoreNotFound
	}
	if s.tokenizer == nil {
		return nil, ErrTokenizerNotFound
	}

	tokens := 0

	convParts := []Part{}
	var conv Conversation
	if view != nil {
		conv = view.Conversation()
	}
	if conv != nil {
		renderedConv := renderMessages(conv.Messages())
		if renderedConv != "" {
			convTokens, err := s.tokenizer.CountTokens(ctx, renderedConv)
			if err != nil {
				return nil, err
			}
			tokens += convTokens
			convParts = append(convParts, NewPart("current-loop", renderedConv, Required(), Tokens(convTokens)))
		}
	}

	parts := []Part{}
	historyOffset := 0
	for tokens < s.tokenLimit {
		messages, err := s.store.GetMessages(ctx, s.id, 1, historyOffset)
		if err != nil {
			return nil, err
		}
		if len(messages) == 0 {
			break
		}
		historyOffset += len(messages)

		rendered := renderMessages(messages)
		messageTokens, err := s.tokenizer.CountTokens(ctx, rendered)
		if err != nil {
			return nil, err
		}
		if tokens+messageTokens > s.tokenLimit {
			break
		}

		tokens += messageTokens
		part := NewPart("history-"+strconv.Itoa(len(parts)), rendered, Required(), Tokens(messageTokens))
		parts = append(parts, part)
	}

	parts = append(parts, convParts...)
	return parts, nil
}
