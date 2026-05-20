package prompt

import "fmt"

func BuildPrompt(contextText, question string) string {
	return fmt.Sprintf(`You are a helpful assistant. Answer using only the provided context. If the question cannot be answered from the context, say "I don't have enough information to answer this question."\n\nContext:\n%s\n\nQuestion:\n%s\n`, contextText, question)
}
