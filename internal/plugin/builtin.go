package plugin

import (
	"log/slog"
	"regexp"
	"strings"
)

// WordCountPlugin is a built-in plugin that demonstrates both action and
// filter hooks. It logs word counts on article lifecycle events and
// normalizes content whitespace via a filter.
type WordCountPlugin struct {
	config map[string]interface{}
}

// NewWordCountPlugin creates the built-in word-count plugin.
func NewWordCountPlugin() *WordCountPlugin { return &WordCountPlugin{} }

func (p *WordCountPlugin) Name() string    { return "word-count" }
func (p *WordCountPlugin) Version() string { return "1.0.0" }
func (p *WordCountPlugin) Description() string {
	return "Logs article word counts and normalizes content whitespace"
}
func (p *WordCountPlugin) Author() string { return "ContentX" }

func (p *WordCountPlugin) Init(config map[string]interface{}) error {
	p.config = config
	return nil
}

func (p *WordCountPlugin) Hooks() []HookRegistration {
	return []HookRegistration{
		{
			Name:     "article.afterCreate",
			Type:     HookAction,
			Priority: 0,
			Fn:       p.afterCreate,
		},
		{
			Name:     "article.afterDelete",
			Type:     HookAction,
			Priority: 0,
			Fn:       p.afterDelete,
		},
		{
			Name:     "article.filterContent",
			Type:     HookFilter,
			Priority: 0,
			Fn:       p.filterContent,
		},
	}
}

// afterCreate logs the word count of a newly created article.
func (p *WordCountPlugin) afterCreate(args map[string]interface{}) (interface{}, error) {
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)
	words := countWords(content)
	slog.Info("word-count: article created",
		"title", title, "word_count", words)
	return nil, nil
}

// afterDelete logs article deletion.
func (p *WordCountPlugin) afterDelete(args map[string]interface{}) (interface{}, error) {
	id, _ := args["article_id"].(uint)
	slog.Info("word-count: article deleted", "article_id", id)
	return nil, nil
}

// filterContent collapses multiple whitespace characters into single spaces
// and trims leading/trailing whitespace from the article content.
func (p *WordCountPlugin) filterContent(args map[string]interface{}) (interface{}, error) {
	content, ok := args["value"].(string)
	if !ok {
		return nil, nil
	}
	return normalizeWhitespace(content), nil
}

var whitespaceRe = regexp.MustCompile(`[ \t\r\n]+`)

func normalizeWhitespace(s string) string {
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(s, " "))
}

func countWords(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return len(strings.Fields(s))
}
