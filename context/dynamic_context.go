package context

import "github.com/lace-ai/gai/ai"

type DynamicSource struct {
	store      RAGStore
	id         int
	tokenLimit int
	tokenizer  ai.Tokenizer
}

func Dynamic(store RAGStore, id, tokenLimit int, tokenizer ai.Tokenizer) DynamicSource {
	return DynamicSource{
		store:      store,
		id:         id,
		tokenLimit: tokenLimit,
		tokenizer:  tokenizer,
	}
}
