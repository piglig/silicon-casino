import test from "node:test";
import assert from "node:assert/strict";

import { computeBackoffMs } from "./backoff.js";

test("computeBackoffMs grows exponentially without jitter", () => {
  assert.equal(computeBackoffMs(1, 500, 8000, false), 500);
  assert.equal(computeBackoffMs(2, 500, 8000, false), 1000);
  assert.equal(computeBackoffMs(5, 500, 8000, false), 8000);
});
