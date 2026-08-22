import type { ReaderBackend } from "@rss-reader/shared";
import { extractReaderArticle } from "./readerMode";

export async function runFrontendExtract(backend: ReaderBackend, articleId: string): Promise<void> {
  const article = await backend.articles.get(articleId);
  if (article.extractStatus !== "js") {
    return;
  }
  const extracted = extractReaderArticle(article.crawledContent || "", article.url);
  await backend.articles.setExtract(articleId, extracted?.contentHtml ?? "");
}

export async function drainPendingExtracts(backend: ReaderBackend): Promise<void> {
  const { articleIds } = await backend.articles.pendingExtract();
  for (const id of articleIds ?? []) {
    try {
      await runFrontendExtract(backend, id);
    } catch {
      // One failure must not block the rest of the JS extract queue.
    }
  }
}
