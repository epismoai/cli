import assert from "node:assert/strict";
import { mkdir, mkdtemp, readFile, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { binaryFor, detectPackageManager, installationScope, targetFor } from "./launcher.js";

const npmDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(npmDir, "..");

async function readPlatformManifest() {
  const contents = await readFile(join(npmDir, "platforms.txt"), "utf8");
  return contents
    .trim()
    .split("\n")
    .map((line) => line.trim().split(/\s+/))
    .map(([target, , binaryName]) => ({ target, binaryName }));
}

test("maps every supported Node platform to its bundled binary", () => {
  const cases = [
    ["darwin", "x64", "darwin-amd64", "epismo"],
    ["darwin", "arm64", "darwin-arm64", "epismo"],
    ["linux", "x64", "linux-amd64", "epismo"],
    ["linux", "arm64", "linux-arm64", "epismo"],
    ["win32", "x64", "windows-amd64", "epismo.exe"],
    ["win32", "arm64", "windows-arm64", "epismo.exe"]
  ];
  for (const [platform, architecture, target, name] of cases) {
    assert.equal(targetFor(platform, architecture), target);
    assert.equal(binaryFor(platform, architecture, "/package"), join("/package", "npm", "vendor", target, name));
  }
  assert.equal(targetFor("freebsd", "x64"), null);
});

test("detects Bun and Yarn layouts and otherwise defaults to npm", () => {
  assert.equal(detectPackageManager({ root: "/home/me/.bun/install/global/node_modules/epismo" }).name, "bun");
  assert.equal(detectPackageManager({ root: "/home/me/.config/yarn/global/node_modules/epismo" }).name, "yarn");
  assert.deepEqual(detectPackageManager({ root: "/usr/local/lib/node_modules/epismo", userAgent: "npm/11.0.0 node/v24" }), { name: "npm", version: "11.0.0" });
});

test("detects a pnpm-owned package from its node_modules metadata", async (t) => {
  const temporary = await mkdtemp(join(tmpdir(), "epismo-launcher-test-"));
  const packageRoot = join(temporary, "store", "epismo");
  const nodeModules = join(temporary, "node_modules");
  await mkdir(packageRoot, { recursive: true });
  await mkdir(nodeModules, { recursive: true });
  await writeFile(join(nodeModules, ".modules.yaml"), "virtualStoreDir: .pnpm\n");
  await symlink(packageRoot, join(nodeModules, "epismo"), "dir");
  t.after(() => rm(temporary, { recursive: true, force: true }));
  assert.equal(detectPackageManager({ root: packageRoot, entrypoint: join(temporary, "bin", "epismo") }).name, "pnpm");
});

test("marks npx-style executions as ephemeral", () => {
  assert.equal(installationScope({ npm_command: "exec" }), "ephemeral");
  assert.equal(installationScope({}), "global");
});

test("shared platform manifest, launcher targets, and package.json files stay in sync", async () => {
  const manifest = await readPlatformManifest();
  const packageJson = JSON.parse(await readFile(join(repoRoot, "package.json"), "utf8"));
  const vendorFiles = packageJson.files.filter((entry) => entry.startsWith("npm/vendor/"));

  assert.equal(vendorFiles.length, manifest.length, "package.json files should list exactly the manifest's vendor binaries");
  for (const { target, binaryName } of manifest) {
    const expectedPath = `npm/vendor/${target}/${binaryName}`;
    assert.ok(vendorFiles.includes(expectedPath), `package.json files is missing ${expectedPath} from npm/platforms.txt`);
  }

  const platformArchPairs = [
    ["darwin", "x64"],
    ["darwin", "arm64"],
    ["linux", "x64"],
    ["linux", "arm64"],
    ["win32", "x64"],
    ["win32", "arm64"]
  ];
  const derivedTargets = platformArchPairs.map(([platform, architecture]) => targetFor(platform, architecture)).sort();
  const manifestTargets = manifest.map(({ target }) => target).sort();
  assert.deepEqual(derivedTargets, manifestTargets, "launcher.js's supported targets should match npm/platforms.txt");
});
