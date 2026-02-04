import type { BotAction } from "../types/bot.js";

export function validateBotAction(action: BotAction): void {
  if (action.action === "raise" || action.action === "bet") {
    if (!Number.isFinite(action.amount) || action.amount <= 0) {
      throw new Error(`${action.action} requires positive amount`);
    }
  }
}

export function nextRequestId(): string {
  return `req_${Date.now()}_${Math.floor(Math.random() * 1_000_000_000)}`;
}
