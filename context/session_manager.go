package context

import (
	stdcontext "context"
	"strings"
)

const defaultHistoryLimit = 5

type SessionManager struct {
	store SessionStore
	id    int
	limit int
}

func NewSessionManager(store SessionStore, id int) *SessionManager {
	return &SessionManager{
		store: store,
		id:    id,
		limit: defaultHistoryLimit,
	}
}

func History(store SessionStore, id int, limit int) Source {
	return &HistorySource{
		store: store,
		id:    id,
		limit: limit,
	}
}

type HistorySource struct {
	store SessionStore
	id    int
	limit int
}

func (s *SessionManager) BuildParts(ctx stdcontext.Context, conv Conversation) ([]Part, error) {
	if s == nil {
		return nil, ErrSessionStoreNotFound
	}
	return (&HistorySource{
		store: s.store,
		id:    s.id,
		limit: s.limit,
	}).BuildParts(ctx, conv)
}

func (s *HistorySource) BuildParts(ctx stdcontext.Context, conv Conversation) ([]Part, error) {
	if s == nil || s.store == nil {
		return nil, ErrSessionStoreNotFound
	}

	limit := s.limit
	if limit <= 0 {
		limit = defaultHistoryLimit
	}

	messages, err := s.store.GetMessages(s.id, limit, 0)
	if err != nil {
		return nil, err
	}

	parts := []Part{
		StaticPart("history", renderMessages(messages)),
	}
	if conv != nil {
		parts = append(parts, StaticPart("current-loop", renderMessages(conv.Messages())))
	}

	return parts, nil
}

func renderMessages(messages []Message) string {
	var builder strings.Builder
	RenderMessages(messages, &builder)
	return builder.String()
}
