package context

type SessionStore interface {
	GetSession(sessionID int) error
	// GetMessages returns messages in a session, ordered by created_at asc
	GetMessages(sessionID int, limit int, offset int) ([]Message, error)
	CreateSession() (int, error)
	AddMessages(sessionID int, messages []Message) ([]Message, error)
	AddMessage(sessionID int, message Message) (Message, error)
}

type Document struct {
	ID      int
	Content string
}

type RagStore interface {
	// GetRelevantDocuments returns relevant documents for a query, ordered by relevance desc
	GetRelevantDocuments(query string, limit int) ([]Document, error)
	AddDocument(content string) (int, error)
}
