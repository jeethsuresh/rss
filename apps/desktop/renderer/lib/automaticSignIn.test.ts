import { describe, expect, test } from "bun:test";
import { appReturnPath, automaticSignInURL } from "./automaticSignIn";

const current = { pathname: "/", search: "?view=workspace", hash: "#document=abc" };
const options = { enabled: true, hasSession: false, attempted: false, location: current };

describe("automatic Gravitas sign-in", () => {
  test("preserves the requested document while requesting passive authentication", () => {
    const url = new URL(automaticSignInURL(options)!, "https://app.example");
    expect(url.pathname).toBe("/auth/gravitas/start");
    expect(url.searchParams.get("prompt")).toBe("none");
    expect(url.searchParams.get("next")).toBe("/?view=workspace#document=abc");
  });
  test("does not restart after a failed passive attempt, logout or an existing session", () => {
    expect(automaticSignInURL({ ...options, enabled: false })).toBeNull();
    expect(automaticSignInURL({ ...options, hasSession: true })).toBeNull();
    expect(automaticSignInURL({ ...options, attempted: true })).toBeNull();
    expect(automaticSignInURL({ ...options, location: { ...current, pathname: "/login", search: "?auto=off&next=%2F" } })).toBeNull();
  });
  test("keeps a fragment inherited through a server login redirect", () => {
    expect(appReturnPath({ pathname: "/login", search: "?next=%2Farticles", hash: "#saved" })).toBe("/articles#saved");
    expect(appReturnPath({ pathname: "/login", search: "?next=%2F%23document%3Dxyz", hash: "#old" })).toBe("/#document=xyz");
  });
});
