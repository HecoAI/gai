package context

import (
	"strings"
)

const (
	defaultHistoryMessageLimit = 5
	historyFetchBatchSize      = 100
	currentLoopLabel           = "\nCurrent Loop:\n"
)

type TokenCounter interface {
	CountTokens(text string) int
}

type SessionManager struct {
	store            SessionStore
	id               int
	tokenCounter     TokenCounter
	maxHistoryTokens int
}

func NewSessionManager(store SessionStore, id int) *SessionManager {
	return &SessionManager{
		store: store,
		id:    id,
	}
}

func NewSessionManagerWithHistoryLimit(store SessionStore, id int, counter TokenCounter, maxTokens int) *SessionManager {
	return &SessionManager{
		store:            store,
		id:               id,
		tokenCounter:     counter,
		maxHistoryTokens: maxTokens,
	}
}

func (s *SessionManager) BuildContext(conv Conversation) (string, error) {
	if s.tokenCounter == nil || s.maxHistoryTokens <= 0 {
		return s.buildLegacyContext(conv)
	}

	return s.buildBudgetedContext(conv.Messages())
}

func (s *SessionManager) buildLegacyContext(conv Conversation) (string, error) {
	var builder strings.Builder

	messages, err := s.store.GetMessages(s.id, defaultHistoryMessageLimit, 0)
	if err != nil {
		return "", err
	}
	RenderMessages(messages, &builder)
	builder.WriteString(currentLoopLabel)
	RenderMessages(conv.Messages(), &builder)

	return builder.String(), nil
}

func (s *SessionManager) buildBudgetedContext(currentLoopMessages []Message) (string, error) {
	_, trimmedCurrentLoopMessages := fitMessagesToBudget(nil, currentLoopMessages, s.tokenCounter, s.maxHistoryTokens)

	historyMessages, err := s.selectHistoryWithinBudget(trimmedCurrentLoopMessages)
	if err != nil {
		return "", err
	}
	return renderSessionContext(historyMessages, trimmedCurrentLoopMessages), nil
}

func (s *SessionManager) selectHistoryWithinBudget(currentLoopMessages []Message) ([]Message, error) {
	totalMessages, err := s.findStoredMessageCount()
	if err != nil || totalMessages == 0 {
		return nil, err
	}

	keptHistory := make([]Message, 0, min(historyFetchBatchSize, totalMessages))
	for end := totalMessages; end > 0; {
		start := max(0, end-historyFetchBatchSize)
		batch, err := s.store.GetMessages(s.id, end-start, start)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}

		candidateHistory := append(append([]Message(nil), batch...), keptHistory...)
		historyMessages, trimmedCurrent := fitMessagesToBudget(candidateHistory, currentLoopMessages, s.tokenCounter, s.maxHistoryTokens)
		if len(trimmedCurrent) != len(currentLoopMessages) {
			return historyMessages, nil
		}
		if len(historyMessages) < len(candidateHistory) {
			return historyMessages, nil
		}

		keptHistory = candidateHistory
		if start == 0 {
			return keptHistory, nil
		}
		end = start
	}

	return keptHistory, nil
}

func (s *SessionManager) findStoredMessageCount() (int, error) {
	first, err := s.store.GetMessages(s.id, 1, 0)
	if err != nil {
		return 0, err
	}
	if len(first) == 0 {
		return 0, nil
	}

	lower := 0
	upper := 1

	for {
		messages, err := s.store.GetMessages(s.id, 1, upper)
		if err != nil {
			return 0, err
		}
		if len(messages) == 0 {
			break
		}
		lower = upper
		upper *= 2
	}

	for lower < upper {
		mid := lower + (upper-lower)/2
		messages, err := s.store.GetMessages(s.id, 1, mid)
		if err != nil {
			return 0, err
		}
		if len(messages) == 0 {
			upper = mid
			continue
		}
		lower = mid + 1
	}

	return lower, nil
}

func renderSessionContext(historyMessages []Message, currentLoopMessages []Message) string {
	var builder strings.Builder
	RenderMessages(historyMessages, &builder)
	builder.WriteString(currentLoopLabel)
	RenderMessages(currentLoopMessages, &builder)
	return builder.String()
}
