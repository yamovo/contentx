package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerPrompts adds AI-writing prompt templates. Prompts are pure guidance
// for the client LLM: they orchestrate the existing read/write tools but never
// touch the database themselves, so they are safe to expose on every transport.
// When write tools are absent (stdio, read-only), the templates instruct the
// agent to present the draft instead of persisting it.
func registerPrompts(s *mcpsdk.Server) {
	s.AddPrompt(&mcpsdk.Prompt{
		Name:        "write_article",
		Title:       "Write a new article",
		Description: "Research existing content, draft a new article on the given topic, and save it as a draft via create_article.",
		Arguments: []*mcpsdk.PromptArgument{
			{Name: "topic", Description: "What the article should cover", Required: true},
			{Name: "style", Description: "Writing style, e.g. tutorial, news, opinion (default: informative blog post)"},
			{Name: "length", Description: "Target length: short (~400 words), medium (~800), long (1500+) (default: medium)"},
			{Name: "locale", Description: "BCP-47 locale for the article, e.g. en or zh (default: en)"},
		},
	}, promptWriteArticle)

	s.AddPrompt(&mcpsdk.Prompt{
		Name:        "improve_article",
		Title:       "Improve an existing article",
		Description: "Fetch an article by ID, improve it with the requested focus, and save the revision via update_article.",
		Arguments: []*mcpsdk.PromptArgument{
			{Name: "article_id", Description: "The ID of the article to improve", Required: true},
			{Name: "focus", Description: "What to improve: readability, structure, seo, grammar (default: overall quality)"},
		},
	}, promptImproveArticle)

	s.AddPrompt(&mcpsdk.Prompt{
		Name:        "summarize_article",
		Title:       "Summarize an article",
		Description: "Fetch an article by ID and produce a concise summary, optionally saving it as the article's excerpt.",
		Arguments: []*mcpsdk.PromptArgument{
			{Name: "article_id", Description: "The ID of the article to summarize", Required: true},
			{Name: "max_words", Description: "Maximum summary length in words (default: 100)"},
		},
	}, promptSummarizeArticle)

	s.AddPrompt(&mcpsdk.Prompt{
		Name:        "translate_article",
		Title:       "Translate an article",
		Description: "Fetch an article by ID, translate it into the target locale, and save the translation as a new draft via create_article.",
		Arguments: []*mcpsdk.PromptArgument{
			{Name: "article_id", Description: "The ID of the source article", Required: true},
			{Name: "target_locale", Description: "BCP-47 locale to translate into, e.g. zh, en, ja", Required: true},
		},
	}, promptTranslateArticle)
}

// promptResult wraps a single user-role instruction message.
func promptResult(description, text string) *mcpsdk.GetPromptResult {
	return &mcpsdk.GetPromptResult{
		Description: description,
		Messages: []*mcpsdk.PromptMessage{
			{Role: "user", Content: &mcpsdk.TextContent{Text: text}},
		},
	}
}

// arg returns the named prompt argument or a fallback default.
func arg(req *mcpsdk.GetPromptRequest, name, fallback string) string {
	if req != nil && req.Params != nil {
		if v, ok := req.Params.Arguments[name]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return fallback
}

// requireArg returns the named argument or an error when missing.
func requireArg(req *mcpsdk.GetPromptRequest, name string) (string, error) {
	v := arg(req, name, "")
	if v == "" {
		return "", fmt.Errorf("missing required argument: %s", name)
	}
	return v, nil
}

func promptWriteArticle(_ context.Context, req *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
	topic, err := requireArg(req, "topic")
	if err != nil {
		return nil, err
	}
	style := arg(req, "style", "informative blog post")
	length := arg(req, "length", "medium (~800 words)")
	locale := arg(req, "locale", "en")

	text := fmt.Sprintf(`You are a content writer working inside the ContentX CMS. Write a new article.

Topic: %s
Style: %s
Target length: %s
Locale: %s

Follow this workflow:
1. Call search_content with keywords from the topic to check what already exists. If a very similar article is already published, say so and ask whether to continue instead of duplicating it.
2. Draft the article in Markdown: a compelling title, a short excerpt (1-2 sentences), an introduction, well-structured sections with headings, and a conclusion.
3. If the create_article tool is available, save the draft with create_article (title, content, excerpt, locale "%s"). Never publish it yourself: publishing requires an explicit, separate publish_article call by the user.
4. If create_article is not available (read-only session), present the full draft in the chat instead.
5. End by reporting the article ID (if saved) and asking the user to review the draft.`, topic, style, length, locale, locale)

	return promptResult("Write a new article about: "+topic, text), nil
}

func promptImproveArticle(_ context.Context, req *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
	id, err := requireArg(req, "article_id")
	if err != nil {
		return nil, err
	}
	focus := arg(req, "focus", "overall quality")

	text := fmt.Sprintf(`You are an editor working inside the ContentX CMS. Improve an existing article.

Article ID: %s
Improvement focus: %s

Follow this workflow:
1. Call get_article with id %s to fetch the current title, excerpt and body.
2. Analyze the article and improve it with the requested focus. Preserve the author's voice, factual claims and any code samples; do not invent facts.
3. Summarize the changes you made as a short bullet list.
4. If the update_article tool is available, save the revision with update_article (id %s). The update never changes the publication status.
5. If update_article is not available (read-only session), present the improved version in the chat instead.
6. End with the change summary so the user can review before publishing.`, id, focus, id, id)

	return promptResult("Improve article "+id, text), nil
}

func promptSummarizeArticle(_ context.Context, req *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
	id, err := requireArg(req, "article_id")
	if err != nil {
		return nil, err
	}
	maxWords := arg(req, "max_words", "100")

	text := fmt.Sprintf(`You are working inside the ContentX CMS. Summarize an article.

Article ID: %s
Maximum length: %s words

Follow this workflow:
1. Call get_article with id %s to fetch the full body.
2. Write a faithful, self-contained summary of at most %s words. Do not add information that is not in the article.
3. Present the summary in the chat.
4. Only if the user explicitly asks to store it and update_article is available, save the summary as the article's excerpt via update_article (id %s).`, id, maxWords, id, maxWords, id)

	return promptResult("Summarize article "+id, text), nil
}

func promptTranslateArticle(_ context.Context, req *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
	id, err := requireArg(req, "article_id")
	if err != nil {
		return nil, err
	}
	target, err := requireArg(req, "target_locale")
	if err != nil {
		return nil, err
	}

	text := fmt.Sprintf(`You are a translator working inside the ContentX CMS. Translate an article.

Source article ID: %s
Target locale: %s

Follow this workflow:
1. Call get_article with id %s to fetch the source title, excerpt and body.
2. Translate the title, excerpt and body into %s. Keep Markdown structure, links and code blocks intact; translate naturally rather than word-for-word.
3. If the create_article tool is available, save the translation as a new draft with create_article (locale "%s"). Never publish it yourself.
4. If create_article is not available (read-only session), present the translation in the chat instead.
5. End by reporting the new draft's article ID (if saved) alongside the source ID %s.`, id, target, id, target, target, id)

	return promptResult(fmt.Sprintf("Translate article %s into %s", id, target), text), nil
}
