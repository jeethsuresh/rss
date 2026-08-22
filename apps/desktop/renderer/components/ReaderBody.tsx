import type { Article } from "@rss-reader/shared";
import { readerPaneModel } from "../lib/readerMode";

type Props = {
  article: Article;
  contentBusy: boolean;
  onRecrawl: () => void;
};

export function ReaderBody({ article, contentBusy, onRecrawl }: Props) {
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
        <div className="reader-body reader-mode-body">
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
