package context

import "context"

type SessionStore interface {
	GetSession(ctx context.Context, sessionID int) error
	// GetMessages returns messages in a session, ordered by created_at asc
	GetMessages(ctx context.Context, sessionID int, limit int, offset int) ([]Message, error)
	CreateSession(ctx context.Context) (int, error)
	AddMessages(ctx context.Context, sessionID int, messages []Message) ([]Message, error)
	AddMessage(ctx context.Context, sessionID int, message Message) (Message, error)
}
