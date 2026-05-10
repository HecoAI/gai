package context

func fitMessagesToBudget(storedMessages []Message, currentLoopMessages []Message, counter TokenCounter, maxTokens int) ([]Message, []Message) {
	if counter == nil || maxTokens <= 0 {
		return storedMessages, currentLoopMessages
	}

	allMessages := append(append([]Message(nil), storedMessages...), currentLoopMessages...)
	currentCount := len(currentLoopMessages)
	kept := make([]Message, 0, len(allMessages))

	for i := len(allMessages) - 1; i >= 0; i-- {
		candidate := prependMessage(allMessages[i], kept)
		if counter.CountTokens(renderSelectedMessages(candidate, currentCount)) <= maxTokens {
			kept = candidate
			continue
		}

		if truncated, ok := truncateMessageToFit(allMessages[i], kept, currentCount, counter, maxTokens); ok {
			kept = prependMessage(truncated, kept)
		}
		break
	}

	if len(kept) == 0 {
		return nil, nil
	}

	currentKeptCount := min(len(kept), currentCount)
	historyEnd := len(kept) - currentKeptCount

	return kept[:historyEnd], kept[historyEnd:]
}

func prependMessage(msg Message, messages []Message) []Message {
	out := make([]Message, 0, len(messages)+1)
	out = append(out, msg)
	out = append(out, messages...)
	return out
}

func renderSelectedMessages(messages []Message, currentCount int) string {
	currentKeptCount := min(len(messages), currentCount)
	storedCount := len(messages) - currentKeptCount
	return renderSessionContext(messages[:storedCount], messages[storedCount:])
}

func truncateMessageToFit(msg Message, suffix []Message, currentCount int, counter TokenCounter, maxTokens int) (Message, bool) {
	switch content := msg.Content.(type) {
	case TextContent:
		text, ok := truncateTextToFit(content.Text, func(candidate string) bool {
			msgCopy := msg
			msgCopy.Content = NewTextContent(candidate)
			return counter.CountTokens(renderSelectedMessages(prependMessage(msgCopy, suffix), currentCount)) <= maxTokens
		})
		if !ok {
			return Message{}, false
		}
		msg.Content = NewTextContent(text)
		return msg, true
	case ToolResultContent:
		text, ok := truncateTextToFit(content.Result, func(candidate string) bool {
			msgCopy := msg
			msgCopy.Content = NewToolResultContent(content.ToolName, candidate, content.Precomputed, content.PrecomputedResult)
			return counter.CountTokens(renderSelectedMessages(prependMessage(msgCopy, suffix), currentCount)) <= maxTokens
		})
		if !ok {
			return Message{}, false
		}
		msg.Content = NewToolResultContent(content.ToolName, text, content.Precomputed, content.PrecomputedResult)
		return msg, true
	case ToolResultErrContent:
		text, ok := truncateTextToFit(content.Err, func(candidate string) bool {
			msgCopy := msg
			msgCopy.Content = NewToolResultErrContent(content.ToolName, candidate)
			return counter.CountTokens(renderSelectedMessages(prependMessage(msgCopy, suffix), currentCount)) <= maxTokens
		})
		if !ok {
			return Message{}, false
		}
		msg.Content = NewToolResultErrContent(content.ToolName, text)
		return msg, true
	default:
		return Message{}, false
	}
}

func truncateTextToFit(text string, fits func(candidate string) bool) (string, bool) {
	runes := []rune(text)
	if len(runes) == 0 {
		if fits("") {
			return "", true
		}
		return "", false
	}

	low := 0
	high := len(runes)
	best := -1

	for low <= high {
		mid := (low + high) / 2
		candidate := string(runes[mid:])
		if fits(candidate) {
			best = mid
			high = mid - 1
			continue
		}
		low = mid + 1
	}

	if best == -1 {
		return "", false
	}
	return string(runes[best:]), true
}
