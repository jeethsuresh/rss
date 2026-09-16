export type ArticleListFilter = "all" | "unread";

// The unread view is a reading queue snapshot. Background state changes may
// update rows in place, but must not rebuild the list and make read items jump
// away while the user is moving through it.
export function shouldReloadArticleListInBackground(filter: ArticleListFilter): boolean {
  return filter !== "unread";
}
