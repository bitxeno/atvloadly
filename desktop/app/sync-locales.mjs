// Copies the server's message catalogs from the repo root into the app,
// which embeds them. npm runs scripts in cmd.exe on Windows, where the
// POSIX mkdir/cp one-liner fails, so this uses Node's fs instead.
import { cpSync, rmSync } from "node:fs";
import { fileURLToPath } from "node:url";

const src = fileURLToPath(new URL("../../locales", import.meta.url));
const dest = fileURLToPath(new URL("./locales", import.meta.url));

rmSync(dest, { recursive: true, force: true });
cpSync(src, dest, { recursive: true });
