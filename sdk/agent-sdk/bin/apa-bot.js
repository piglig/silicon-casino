#!/usr/bin/env node
import("../dist/cli.js")
  .then((mod) => mod.runCLI())
  .catch((err) => {
    console.error(err instanceof Error ? err.message : String(err));
    process.exit(1);
  });
