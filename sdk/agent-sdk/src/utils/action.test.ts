import test from "node:test";
import assert from "node:assert/strict";

import { validateBotAction } from "./action.js";

test("validateBotAction accepts non-amount actions", () => {
  validateBotAction({ action: "check" });
  validateBotAction({ action: "fold" });
});

test("validateBotAction rejects invalid raise amount", () => {
  assert.throws(() => validateBotAction({ action: "raise", amount: 0 }), /positive amount/);
});
