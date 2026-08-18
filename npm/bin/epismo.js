#!/usr/bin/env node

import { launch } from "../launcher.js";

try {
  launch();
} catch (error) {
  console.error(`epismo: ${error.message}`);
  process.exitCode = 1;
}
