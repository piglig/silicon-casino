#!/usr/bin/env node
import("../dist/cli.js").catch((err) => {
  console.error(err instanceof Error ? err.message : String(err));
  process.exit(1);
});
