package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

const installReceiptSchema = 1

type installReceipt struct {
	SchemaVersion    int    `json:"schemaVersion"`
	Method           string `json:"method"`
	InstalledVersion string `json:"installedVersion,omitempty"`
}

type installation struct {
	Method         string
	Manager        string
	ManagerVersion string
	Scope          string
	Executable     string
	Receipt        bool
}

func detectInstallation(currentVersion, distribution string) installation {
	executable, err := os.Executable()
	if err != nil {
		return installation{Method: "unknown"}
	}
	return detectInstallationAt(executable, currentVersion, distribution, currentBuildInfo())
}

func currentBuildInfo() *debug.BuildInfo {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	return info
}

func detectInstallationAt(executable, currentVersion, distribution string, info *debug.BuildInfo) installation {
	executable, _ = filepath.Abs(executable)
	candidates := []string{executable}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil && resolved != executable {
		candidates = append(candidates, resolved)
	}
	for _, candidate := range candidates {
		if receipt, ok := readInstallReceipt(candidate + ".install.json"); ok {
			return installation{Method: receipt.Method, Executable: candidate, Receipt: true}
		}
	}
	if distribution == "node" {
		return nodeInstallation(candidates[len(candidates)-1])
	}
	if distribution == "go" || (distribution == "" && isGoInstall(info)) {
		return installation{Method: "go", Executable: candidates[len(candidates)-1]}
	}
	for _, candidate := range candidates {
		if isLegacyNodeInstall(candidate) {
			return installation{Method: "node", Manager: "unknown", Scope: "unknown", Executable: candidate}
		}
	}
	for _, candidate := range candidates {
		if method, ok := packageManagerMethod(candidate); ok {
			return installation{Method: method, Executable: candidate}
		}
	}
	if isReleaseVersion(currentVersion) {
		return installation{Method: "standalone", Executable: candidates[len(candidates)-1]}
	}
	return installation{Method: "unknown", Executable: candidates[len(candidates)-1]}
}

// packageManagerMethod recognizes executables placed by a system package
// manager from well-known directory layouts, so update instructions don't
// tell a Homebrew or Scoop user to run the curl/PowerShell installer.
func packageManagerMethod(executable string) (string, bool) {
	normalized := strings.ToLower(strings.ReplaceAll(executable, `\`, "/"))
	switch {
	case strings.Contains(normalized, "/cellar/epismo/"),
		strings.Contains(normalized, "/homebrew/"),
		strings.Contains(normalized, "/linuxbrew/"):
		return "homebrew", true
	case strings.Contains(normalized, "/scoop/apps/epismo/"):
		return "scoop", true
	default:
		return "", false
	}
}

func readInstallReceipt(path string) (installReceipt, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return installReceipt{}, false
	}
	var receipt installReceipt
	if json.Unmarshal(data, &receipt) != nil || receipt.SchemaVersion != installReceiptSchema {
		return installReceipt{}, false
	}
	receipt.Method = strings.ToLower(strings.TrimSpace(receipt.Method))
	switch receipt.Method {
	case "curl", "powershell", "standalone":
		return receipt, true
	default:
		return installReceipt{}, false
	}
}

func nodeInstallation(executable string) installation {
	manager := strings.ToLower(strings.TrimSpace(os.Getenv("EPISMO_NODE_MANAGER")))
	switch manager {
	case "npm", "pnpm", "yarn", "bun":
	default:
		manager = "unknown"
	}
	scope := strings.ToLower(strings.TrimSpace(os.Getenv("EPISMO_NODE_SCOPE")))
	switch scope {
	case "global", "local", "ephemeral":
	default:
		scope = "unknown"
	}
	return installation{
		Method:         "node",
		Manager:        manager,
		ManagerVersion: strings.TrimSpace(os.Getenv("EPISMO_NODE_MANAGER_VERSION")),
		Scope:          scope,
		Executable:     executable,
	}
}

func isGoInstall(info *debug.BuildInfo) bool {
	return info != nil && info.Main.Path == "github.com/epismoai/cli" && info.Main.Version != "" && info.Main.Version != "(devel)"
}

func isLegacyNodeInstall(executable string) bool {
	directory := filepath.Dir(executable)
	for range 6 {
		data, err := os.ReadFile(filepath.Join(directory, "package.json"))
		if err == nil {
			var pkg struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(data, &pkg) == nil && pkg.Name == "epismo" {
				return true
			}
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
		directory = parent
	}
	return false
}
