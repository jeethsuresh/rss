import { describe, expect, test } from "bun:test";
import { PROTOCOL_VERSION, RPC_METHODS } from "./src/index";

describe("shared contract", () => {
  test("protocol version is stable", () => {
    expect(PROTOCOL_VERSION).toBe(1);
  });

  test("includes core methods", () => {
    expect(RPC_METHODS).toContain("system.ping");
    expect(RPC_METHODS).toContain("feeds.add");
    expect(RPC_METHODS).toContain("articles.list");
    expect(RPC_METHODS).toContain("feeds.importUrls");
    expect(RPC_METHODS).toContain("stories.list");
    expect(RPC_METHODS).toContain("stories.voteArticle");
    expect(RPC_METHODS).toContain("stories.reindex");
    expect(RPC_METHODS).toContain("stories.split");
    expect(RPC_METHODS).toContain("ai.scan");
  });
});
