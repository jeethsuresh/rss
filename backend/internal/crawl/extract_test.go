package crawl

import (
	"strings"
	"testing"
)

var noisyArticleHTML = `<!doctype html><html><body>
<nav>Home Sports Weather</nav>
<article>
  <h1>Rural broadband funding</h1>
  <p>By Jane Doe</p>
  <p>` + strings.Repeat("The province announced new funding for rural broadband so towns can finally get reliable service. ", 8) + `</p>
  <p>` + strings.Repeat("Officials said construction starts next spring and households can apply for subsidies. ", 8) + `</p>
</article>
<aside>Subscribe to our newsletter</aside>
</body></html>`

func TestExtractHTMLReturnsArticleText(t *testing.T) {
	html, err := ExtractHTML(noisyArticleHTML)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(html), "rural broadband") && !strings.Contains(strings.ToLower(stripTags(html)), "broadband") {
		t.Fatalf("expected article text, got %q", html)
	}
	if strings.Contains(html, "Subscribe to our newsletter") {
		t.Fatalf("should drop newsletter chrome, got %q", html)
	}
}

func TestExtractHTMLRejectsEmpty(t *testing.T) {
	if _, err := ExtractHTML("   "); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ExtractHTML("<html><body></body></html>"); err == nil {
		t.Fatal("expected error for empty article")
	}
}
