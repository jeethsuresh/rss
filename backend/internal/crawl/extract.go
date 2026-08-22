package crawl

import (
	"fmt"
	"strings"

	"github.com/mackee/go-readability"
)

func ExtractHTML(pageHTML string) (string, error) {
	if strings.TrimSpace(pageHTML) == "" {
		return "", fmt.Errorf("empty html")
	}
	opts := readability.DefaultOptions()
	opts.CharThreshold = 140
	article, err := readability.Extract(pageHTML, opts)
	if err != nil {
		return "", err
	}
	if article.Root == nil {
		return "", fmt.Errorf("no article")
	}
	html := strings.TrimSpace(readability.ToHTML(article.Root))
	if stripTags(html) == "" {
		return "", fmt.Errorf("empty article")
	}
	return html, nil
}
