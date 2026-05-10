package context

import (
	"strings"
	"testing"
)

type stubConversation struct {
	messages []Message
}

func (c stubConversation) Messages() []Message {
	return c.messages
}

type stubSessionStore struct {
	messages []Message
	limits   []int
	offsets  []int
}

func (s *stubSessionStore) GetSession(sessionID int) error {
	return nil
}

func (s *stubSessionStore) GetMessages(sessionID int, limit int, offset int) ([]Message, error) {
	s.limits = append(s.limits, limit)
	s.offsets = append(s.offsets, offset)
	if offset >= len(s.messages) {
		return nil, nil
	}
	end := offset + limit
	if end > len(s.messages) {
		end = len(s.messages)
	}
	return append([]Message(nil), s.messages[offset:end]...), nil
}

func (s *stubSessionStore) CreateSession() (int, error) {
	return 1, nil
}

func (s *stubSessionStore) AddMessages(sessionID int, messages []Message) ([]Message, error) {
	return messages, nil
}

func (s *stubSessionStore) AddMessage(sessionID int, message Message) (Message, error) {
	return message, nil
}

type runeCounter struct{}

func (runeCounter) CountTokens(text string) int {
	return len([]rune(text))
}

func TestSessionManagerBuildContextLegacyKeepsFixedWindow(t *testing.T) {
	store := &stubSessionStore{
		messages: []Message{
			{Role: RoleUser, Content: NewTextContent("one")},
			{Role: RoleAssistant, Content: NewTextContent("two")},
			{Role: RoleUser, Content: NewTextContent("three")},
			{Role: RoleAssistant, Content: NewTextContent("four")},
			{Role: RoleUser, Content: NewTextContent("five")},
			{Role: RoleAssistant, Content: NewTextContent("six")},
		},
	}
	manager := NewSessionManager(store, 1)

	got, err := manager.BuildContext(stubConversation{
		messages: []Message{{Role: RoleAssistant, Content: NewTextContent("current")}},
	})
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	if len(store.limits) != 1 || store.limits[0] != defaultHistoryMessageLimit || store.offsets[0] != 0 {
		t.Fatalf("expected one fixed-window fetch, got limits=%v offsets=%v", store.limits, store.offsets)
	}
	if strings.Contains(got, "six") {
		t.Fatalf("expected legacy window to exclude messages after first 5 fetch result, got %q", got)
	}
	if !strings.Contains(got, "five") || !strings.Contains(got, "current") {
		t.Fatalf("expected legacy context to include fetched history and current loop, got %q", got)
	}
}

func TestSessionManagerBuildContextBudgetedKeepsNewestMessages(t *testing.T) {
	stored := []Message{
		{Role: RoleUser, Content: NewTextContent("old-a")},
		{Role: RoleAssistant, Content: NewTextContent("old-b")},
		{Role: RoleUser, Content: NewTextContent("keep-c")},
	}
	current := []Message{
		{Role: RoleAssistant, Content: NewTextContent("keep-d")},
		{Role: RoleTool, Content: NewToolResultContent("search", "keep-e", false, "")},
	}
	store := &stubSessionStore{messages: stored}
	full := renderSessionContext(stored, current)
	trimmed := renderSessionContext(stored[2:], current)
	manager := NewSessionManagerWithHistoryLimit(store, 1, runeCounter{}, len([]rune(trimmed)))

	got, err := manager.BuildContext(stubConversation{messages: current})
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	if got != trimmed {
		t.Fatalf("expected trimmed context %q, got %q", trimmed, got)
	}
	if len([]rune(got)) > len([]rune(full)) && len([]rune(got)) > len([]rune(trimmed)) {
		t.Fatalf("unexpected token growth: full=%d trimmed=%d got=%d", len([]rune(full)), len([]rune(trimmed)), len([]rune(got)))
	}
	if len(store.limits) < 1 {
		t.Fatalf("expected budgeted mode to query the store")
	}
}

func TestSessionManagerBuildContextBudgetedStopsAfterNeededTailBatch(t *testing.T) {
	stored := make([]Message, 0, 250)
	for i := 0; i < 250; i++ {
		text := "drop"
		if i >= 200 {
			text = "keep"
		}
		stored = append(stored, Message{Role: RoleUser, Content: NewTextContent(text)})
	}
	current := []Message{{Role: RoleAssistant, Content: NewTextContent("current")}}
	store := &stubSessionStore{messages: stored}
	want := renderSessionContext(stored[200:], current)
	manager := NewSessionManagerWithHistoryLimit(store, 1, runeCounter{}, len([]rune(want)))

	got, err := manager.BuildContext(stubConversation{messages: current})
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}
	if got != want {
		t.Fatalf("expected newest tail batch to fit, got %q", got)
	}

	for i, offset := range store.offsets {
		if offset == 0 && store.limits[i] > 1 {
			t.Fatalf("expected no bulk fetch from the start, limits=%v offsets=%v", store.limits, store.offsets)
		}
	}
}

func TestSessionManagerBuildContextBudgetIncludesCurrentLoop(t *testing.T) {
	stored := []Message{
		{Role: RoleUser, Content: NewTextContent("stored-one")},
	}
	current := []Message{
		{Role: RoleAssistant, Content: NewTextContent("current-two")},
		{Role: RoleTool, Content: NewToolResultContent("calc", "current-three", false, "")},
	}
	store := &stubSessionStore{messages: stored}
	want := renderSessionContext(nil, current)
	manager := NewSessionManagerWithHistoryLimit(store, 1, runeCounter{}, len([]rune(want)))

	got, err := manager.BuildContext(stubConversation{messages: current})
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	if got != want {
		t.Fatalf("expected current-loop-only context %q, got %q", want, got)
	}
}

func TestSessionManagerBuildContextTrimsOversizedNewestTextMessage(t *testing.T) {
	current := []Message{
		{Role: RoleAssistant, Content: NewTextContent("abcdefghij")},
	}
	store := &stubSessionStore{}
	full := renderSessionContext(nil, current)
	manager := NewSessionManagerWithHistoryLimit(store, 1, runeCounter{}, len([]rune(full))-4)

	got, err := manager.BuildContext(stubConversation{messages: current})
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	if got == full {
		t.Fatalf("expected oversized newest text message to be trimmed")
	}
	if !strings.Contains(got, "efghij") {
		t.Fatalf("expected trimmed message suffix to remain, got %q", got)
	}
	if len([]rune(got)) > len([]rune(full))-4 {
		t.Fatalf("expected context to fit budget, got len=%d budget=%d", len([]rune(got)), len([]rune(full))-4)
	}
}

func TestSessionManagerBuildContextDoesNotPartiallyTrimToolCalls(t *testing.T) {
	current := []Message{
		{Role: RoleAssistant, Content: NewToolCallContent("search", "{\"query\":\"long query\"}")},
	}
	store := &stubSessionStore{}
	full := renderSessionContext(nil, current)
	manager := NewSessionManagerWithHistoryLimit(store, 1, runeCounter{}, len([]rune(full))-5)

	got, err := manager.BuildContext(stubConversation{messages: current})
	if err != nil {
		t.Fatalf("BuildContext error: %v", err)
	}

	if got != currentLoopLabel {
		t.Fatalf("expected oversized tool call to be dropped entirely, got %q", got)
	}
}

func TestSessionManagerBuildContextCanTrimToolResultAndError(t *testing.T) {
	store := &stubSessionStore{}

	resultCurrent := []Message{
		{Role: RoleTool, Content: NewToolResultContent("search", "abcdefghij", false, "")},
	}
	resultBudget := len([]rune(renderSessionContext(nil, resultCurrent))) - 3
	resultManager := NewSessionManagerWithHistoryLimit(store, 1, runeCounter{}, resultBudget)

	resultGot, err := resultManager.BuildContext(stubConversation{messages: resultCurrent})
	if err != nil {
		t.Fatalf("BuildContext result error: %v", err)
	}
	if !strings.Contains(resultGot, "defghij") {
		t.Fatalf("expected trimmed tool result suffix, got %q", resultGot)
	}

	errCurrent := []Message{
		{Role: RoleTool, Content: NewToolResultErrContent("search", "klmnopqrst")},
	}
	errBudget := len([]rune(renderSessionContext(nil, errCurrent))) - 3
	errManager := NewSessionManagerWithHistoryLimit(store, 1, runeCounter{}, errBudget)

	errGot, err := errManager.BuildContext(stubConversation{messages: errCurrent})
	if err != nil {
		t.Fatalf("BuildContext tool error: %v", err)
	}
	if !strings.Contains(errGot, "nopqrst") {
		t.Fatalf("expected trimmed tool error suffix, got %q", errGot)
	}
}
