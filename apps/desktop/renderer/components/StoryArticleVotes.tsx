import type { Article, StoryVote } from "@rss-reader/shared";

type Props = { article: Article; vote?: StoryVote; onVote: (vote: StoryVote) => void };

export function StoryArticleVotes({ article, vote, onVote }: Props) {
  return (
    <div className="story-votes" role="group" aria-label={`Votes for ${article.title || "article"}`}>
      <button type="button" className={`story-vote ${vote === "up" ? "active" : ""}`}
        aria-label="Thumbs up" aria-pressed={vote === "up"} onClick={() => onVote("up")}>👍</button>
      <button type="button" className={`story-vote ${vote === "down" ? "active" : ""}`}
        aria-label="Thumbs down" aria-pressed={vote === "down"} onClick={() => onVote("down")}>👎</button>
    </div>
  );
}
