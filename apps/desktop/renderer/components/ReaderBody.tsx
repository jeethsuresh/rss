import type { Article } from "@rss-reader/shared";
import { readerPaneModel } from "../lib/readerMode";

type Props = {
  article: Article;
  contentBusy: boolean;
  onRecrawl: () => void;
  onNavigate?: (url: string) => void;
};

export function ReaderBody({ article, contentBusy, onRecrawl, onNavigate }: Props) {
  const model = readerPaneModel(article);
  switch (model.kind) {
    case "status":
      return (
        <div className="reader-body">
          <p className="muted">{model.message}</p>
          {model.recrawl ? (
            <button className="btn" disabled={contentBusy} onClick={onRecrawl}>
              {contentBusy ? "Retrying…" : "Retry crawl"}
            </button>
          ) : null}
        </div>
      );
    case "article":
      return (
        <div
          className="reader-body reader-mode-body"
          onClick={(event) => {
            const anchor = (event.target as Element).closest("a[href]") as HTMLAnchorElement | null;
            if (!anchor || !onNavigate) return;
            const url = new URL(anchor.getAttribute("href") || "", article.url).href;
            if (!/^https?:\/\//i.test(url)) return;
            event.preventDefault();
            onNavigate(url);
          }}
        >
          {model.byline ? <p className="reader-byline">{model.byline}</p> : null}
          <div dangerouslySetInnerHTML={{ __html: model.contentHtml }} />
        </div>
      );
    default: {
      const _exhaustive: never = model;
      return _exhaustive;
    }
  }
}
