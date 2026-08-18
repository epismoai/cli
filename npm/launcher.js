import { spawn } from "node:child_process";
import { existsSync, realpathSync } from "node:fs";
import { dirname, join, parse, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const packageRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");

export function targetFor(platform = process.platform, architecture = process.arch) {
  const os = { darwin: "darwin", linux: "linux", win32: "windows" }[platform];
  const arch = { x64: "amd64", arm64: "arm64" }[architecture];
  return os && arch ? `${os}-${arch}` : null;
}

export function binaryFor(platform = process.platform, architecture = process.arch, root = packageRoot) {
  const target = targetFor(platform, architecture);
  if (!target) return null;
  return join(root, "npm", "vendor", target, platform === "win32" ? "epismo.exe" : "epismo");
}

export function detectPackageManager({
  root = packageRoot,
  entrypoint = process.argv[1] || "",
  userAgent = process.env.npm_config_user_agent || "",
  execPath = process.env.npm_execpath || ""
} = {}) {
  if (pnpmOwnsPackage(root, entrypoint)) return { name: "pnpm", version: versionFromUserAgent(userAgent, "pnpm") };
  const normalizedRoot = root.toLowerCase().replaceAll("\\", "/");
  if (/\bbun\//i.test(userAgent) || /(?:^|[\\/])bun(?:\.exe)?$/i.test(execPath) || normalizedRoot.includes("/.bun/install/global/")) {
    return { name: "bun", version: versionFromUserAgent(userAgent, "bun") };
  }
  if (/\byarn\//i.test(userAgent) || /(?:^|[\\/])yarn(?:\.c?js|\.cmd)?$/i.test(execPath) || normalizedRoot.includes("/.config/yarn/global/")) {
    return { name: "yarn", version: versionFromUserAgent(userAgent, "yarn") || "1" };
  }
  return { name: "npm", version: versionFromUserAgent(userAgent, "npm") };
}

export function installationScope(environment = process.env) {
  return environment.npm_command === "exec" || environment.npm_lifecycle_event === "npx" ? "ephemeral" : "global";
}

export function launch() {
  const binary = binaryFor();
  if (!binary) {
    throw new Error(`Unsupported platform: ${process.platform}-${process.arch}`);
  }
  if (!existsSync(binary)) {
    throw new Error(`Epismo native binary is missing for ${process.platform}-${process.arch}. Reinstall the epismo package.`);
  }
  const manager = detectPackageManager();
  const env = {
    ...process.env,
    EPISMO_DISTRIBUTION: "node",
    EPISMO_NODE_MANAGER: manager.name,
    EPISMO_NODE_MANAGER_VERSION: manager.version || "",
    EPISMO_NODE_SCOPE: installationScope()
  };
  const child = spawn(binary, process.argv.slice(2), { stdio: "inherit", env });
  const signalHandlers = new Map();
  const removeSignalHandlers = () => {
    for (const [signal, handler] of signalHandlers) process.off(signal, handler);
  };
  child.on("error", (error) => {
    removeSignalHandlers();
    console.error(error.message);
    process.exitCode = 1;
  });
  for (const signal of ["SIGINT", "SIGTERM", "SIGHUP"]) {
    const handler = () => {
      if (!child.killed) child.kill(signal);
    };
    signalHandlers.set(signal, handler);
    process.on(signal, handler);
  }
  child.on("exit", (code, signal) => {
    removeSignalHandlers();
    if (signal && process.platform !== "win32") {
      process.kill(process.pid, signal);
      return;
    }
    process.exitCode = code ?? 1;
  });
}

function versionFromUserAgent(userAgent, manager) {
  return userAgent.match(new RegExp(`(?:^|\\s)${manager}/([^\\s]+)`, "i"))?.[1] || "";
}

function pnpmOwnsPackage(root, entrypoint) {
  for (const start of new Set([root, dirname(resolve(entrypoint || root))])) {
    const filesystemRoot = parse(start).root;
    for (let current = start; ; current = dirname(current)) {
      const nodeModules = join(current, "node_modules");
      if (existsSync(join(nodeModules, ".modules.yaml"))) {
        try {
          if (realpathSync(join(nodeModules, "epismo")) === realpathSync(root)) return true;
        } catch {
          // Keep looking for the owning pnpm node_modules directory.
        }
      }
      if (current === filesystemRoot) break;
    }
  }
  return false;
}
