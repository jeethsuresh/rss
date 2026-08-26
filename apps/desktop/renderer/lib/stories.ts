import type { Article, Story } from "@rss-reader/shared";

export type StoryListRow =
  | { kind: "story"; storyId: string }
  | { kind: "member"; storyId: string; articleId: string };

export function storyListRowKey(row: StoryListRow): string {
  switch (row.kind) {
    case "story":
      return `story:${row.storyId}`;
    case "member":
      return `member:${row.articleId}`;
    default: {
      const _exhaustive: never = row;
      return _exhaustive;
    }
  }
}

export const MIN_STORY_MEMBER_COUNT = 2;

export function isListableStory(story: Pick<Story, "memberCount">): boolean {
  return story.memberCount >= MIN_STORY_MEMBER_COUNT;
}

export function unreadStoryCount(stories: Story[]): number {
  return stories.filter((story) => isListableStory(story) && !story.isRead).length;
}

export function unreadStorySnapshotIds(stories: Story[]): string[] {
  return stories.filter((story) => isListableStory(story) && !story.isRead).map((story) => story.id);
}

export function storiesInSnapshot(stories: Story[], storyIds: readonly string[]): Story[] {
  const storiesById = new Map(stories.map((story) => [story.id, story]));
  return storyIds.flatMap((id) => {
    const story = storiesById.get(id);
    return story && isListableStory(story) ? [story] : [];
  });
}

export function storyListRows(stories: Story[], expanded: Story | null): StoryListRow[] {
  const rows: StoryListRow[] = [];
  for (const story of stories) {
    if (!isListableStory(story)) {
      continue;
    }
    rows.push({ kind: "story", storyId: story.id });
    if (expanded?.id !== story.id) {
      continue;
    }
    const newestFirst = [...(expanded.articles ?? [])].sort((a, b) => {
      const aTime = Date.parse(a.publishedAt ?? a.discoveredAt);
      const bTime = Date.parse(b.publishedAt ?? b.discoveredAt);
      return bTime - aTime;
    });
    for (const article of newestFirst) {
      if (article.isReadLater) {
        continue;
      }
      rows.push({ kind: "member", storyId: story.id, articleId: article.id });
    }
  }
  return rows;
}

export function adjacentMetaStory(
  stories: Story[],
  currentStoryId: string | null,
  delta: number,
): Story | null {
  const listable = stories.filter(isListableStory);
  if (listable.length === 0) return null;
  const index = currentStoryId ? listable.findIndex((story) => story.id === currentStoryId) : -1;
  const from = index < 0 ? (delta > 0 ? -1 : 0) : index;
  const nextIndex = Math.min(listable.length - 1, Math.max(0, from + delta));
  return listable[nextIndex] ?? null;
}

export function adjacentStoryListRow(
  rows: StoryListRow[],
  currentKey: string | null,
  delta: number,
): StoryListRow | null {
  if (rows.length === 0) {
    return null;
  }
  const idx = currentKey ? rows.findIndex((row) => storyListRowKey(row) === currentKey) : -1;
  const from = idx < 0 ? (delta > 0 ? -1 : 0) : idx;
  const to = Math.min(rows.length - 1, Math.max(0, from + delta));
  return rows[to] ?? null;
}

export function memberArticle(story: Story | null, articleId: string | null): Article | null {
  if (!story || !articleId) {
    return null;
  }
  return story.articles?.find((article) => article.id === articleId) ?? null;
}

export function hasNoOtherUnreadStoryMembers(
  story: Story | null,
  currentArticleId: string | null,
): boolean {
  if (!story || !currentArticleId) return false;
  const members = (story.articles ?? []).filter((article) => !article.isReadLater);
  if (!members.some((article) => article.id === currentArticleId)) return false;
  return members.every((article) => article.id === currentArticleId || article.isRead);
}

export function upsertStoryInPlace(stories: Story[], story: Story): Story[] {
  if (!isListableStory(story)) {
    return stories.filter((s) => s.id !== story.id);
  }
  const exists = stories.some((s) => s.id === story.id);
  if (!exists) {
    return [story, ...stories];
  }
  return stories.map((s) =>
    s.id === story.id ? { ...s, ...story, articles: s.articles, articleIds: s.articleIds } : s,
  );
}

export function nextStoryVote(current: string | undefined, clicked: "up" | "down"): "up" | "down" | "none" {
  if (current === clicked) {
    return "none";
  }
  return clicked;
}
