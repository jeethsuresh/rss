import { expect, test } from "bun:test";
import { stableKey, cacheable } from "./offline";

test("RPC cache keys are stable across parameter order and separate accounts", () => {
  expect(
    stableKey({ query: { feedId: "feed", unreadOnly: true }, cursor: null }),
  ).toBe(
    stableKey({ cursor: null, query: { unreadOnly: true, feedId: "feed" } }),
  );
  expect(stableKey(["alice", "articles.get", { id: "one" }])).not.toBe(
    stableKey(["bob", "articles.get", { id: "one" }]),
  );
});
test("only explicitly supported reads enter the offline cache", () => {
  for (const method of [
    "feeds.list",
    "articles.get",
    "articles.list",
    "articles.fetchLive",
    "stories.get",
    "readLater.list",
    "sports.schedule.daily",
    "sports.f1.races.list",
    "settings.get",
  ])
    expect(cacheable(method)).toBe(true);
  for (const method of [
    "feeds.add",
    "feeds.remove",
    "articles.toggleStar",
    "articles.markRead",
    "stories.split",
    "settings.update",
    "sports.f1.race.watch",
    "auth.session",
    "ai.logs",
  ])
    expect(cacheable(method)).toBe(false);
});
