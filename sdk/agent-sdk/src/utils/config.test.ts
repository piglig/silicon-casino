import test from "node:test";
import assert from "node:assert/strict";

import { DEFAULT_API_BASE, resolveApiBase } from "./config.js";

test("resolveApiBase prefers override", () => {
  assert.equal(resolveApiBase("http://x/api"), "http://x/api");
});

test("resolveApiBase falls back to default", () => {
  const old = process.env.API_BASE;
  delete process.env.API_BASE;
  assert.equal(resolveApiBase(undefined), DEFAULT_API_BASE);
  process.env.API_BASE = old;
});
