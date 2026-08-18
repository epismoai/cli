package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func TestDetectNodeInstallationUsesLauncherContext(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "epismo")
	if err := os.WriteFile(executable, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EPISMO_NODE_MANAGER", "pnpm")
	t.Setenv("EPISMO_NODE_SCOPE", "global")
	install := detectInstallationAt(executable, "1.2.3", "node", nil)
	if install.Method != "node" || install.Manager != "pnpm" || install.Scope != "global" || install.Receipt {
		t.Fatalf("installation = %#v", install)
	}
}

func TestDetectInstallationFallbacks(t *testing.T) {
	t.Run("go install", func(t *testing.T) {
		info := &debug.BuildInfo{Main: debug.Module{Path: "github.com/epismoai/cli", Version: "v1.2.3"}}
		install := detectInstallationAt(filepath.Join(t.TempDir(), "epismo"), "1.2.3", "go", info)
		if install.Method != "go" {
			t.Fatalf("installation = %#v", install)
		}
	})
	t.Run("legacy node package", func(t *testing.T) {
		root := t.TempDir()
		binaryDirectory := filepath.Join(root, "node_modules", "epismo", "npm", "bin")
		if err := os.MkdirAll(binaryDirectory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "node_modules", "epismo", "package.json"), []byte(`{"name":"epismo"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		install := detectInstallationAt(filepath.Join(binaryDirectory, "epismo.exe"), "1.2.3", "release", nil)
		if install.Method != "node" || install.Manager != "unknown" {
			t.Fatalf("installation = %#v", install)
		}
	})
	t.Run("release binary", func(t *testing.T) {
		info := &debug.BuildInfo{Main: debug.Module{Path: "github.com/epismoai/cli", Version: "v1.2.3"}}
		install := detectInstallationAt(filepath.Join(t.TempDir(), "epismo"), "1.2.3", "release", info)
		if install.Method != "standalone" {
			t.Fatalf("installation = %#v", install)
		}
	})
	t.Run("development binary", func(t *testing.T) {
		install := detectInstallationAt(filepath.Join(t.TempDir(), "epismo"), "dev", "development", nil)
		if install.Method != "unknown" {
			t.Fatalf("installation = %#v", install)
		}
	})
	t.Run("homebrew on macOS", func(t *testing.T) {
		executable := "/opt/homebrew/Cellar/epismo/1.2.3/bin/epismo"
		install := detectInstallationAt(executable, "1.2.3", "release", nil)
		if install.Method != "homebrew" {
			t.Fatalf("installation = %#v", install)
		}
	})
	t.Run("homebrew on Linux", func(t *testing.T) {
		executable := "/home/linuxbrew/.linuxbrew/Cellar/epismo/1.2.3/bin/epismo"
		install := detectInstallationAt(executable, "1.2.3", "release", nil)
		if install.Method != "homebrew" {
			t.Fatalf("installation = %#v", install)
		}
	})
	t.Run("scoop", func(t *testing.T) {
		// filepath.Dir/Abs are OS-native and won't split a backslash-separated
		// path on a non-Windows test runner, so use forward slashes here; the
		// detection logic itself normalizes separators before matching.
		executable := "C:/Users/me/scoop/apps/epismo/current/epismo.exe"
		install := detectInstallationAt(executable, "1.2.3", "release", nil)
		if install.Method != "scoop" {
			t.Fatalf("installation = %#v", install)
		}
	})
}

func TestUpdateInstructions(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "me")
	defaultExecutable := filepath.Join(home, ".local", "bin", "epismo")
	customExecutable := filepath.Join(home, "tools", "epismo")
	tests := []struct {
		install installation
		goos    string
		want    string
	}{
		{installation{Method: "node", Manager: "npm", Scope: "global"}, "linux", "npm install -g epismo@latest"},
		{installation{Method: "node", Manager: "pnpm", Scope: "global"}, "linux", "pnpm add -g epismo@latest"},
		{installation{Method: "node", Manager: "yarn", ManagerVersion: "1.22.22", Scope: "global"}, "linux", "yarn global add epismo@latest"},
		{installation{Method: "node", Manager: "yarn", ManagerVersion: "4.9.2", Scope: "global"}, "linux", ""},
		{installation{Method: "node", Manager: "yarn", Scope: "global"}, "linux", ""},
		{installation{Method: "node", Manager: "bun", Scope: "global"}, "linux", "bun add -g epismo@latest"},
		{installation{Method: "node", Manager: "unknown", Scope: "global"}, "linux", ""},
		{installation{Method: "node", Manager: "npm", Scope: "unknown"}, "linux", ""},
		{installation{Method: "node", Manager: "npm", Scope: "local"}, "linux", ""},
		{installation{Method: "go"}, "linux", "go install github.com/epismoai/cli/cmd/epismo@latest"},
		{installation{Method: "homebrew"}, "linux", "brew upgrade epismo"},
		{installation{Method: "scoop"}, "windows", "scoop update epismo"},
		{installation{Method: "curl", Executable: defaultExecutable}, "linux", "curl -fsSL https://epismo.ai/install.sh | sh"},
		{installation{Method: "curl", Executable: customExecutable}, "linux", "curl -fsSL https://epismo.ai/install.sh | EPISMO_INSTALL_DIR='" + filepath.Dir(customExecutable) + "' sh"},
		{installation{Method: "powershell", Executable: defaultExecutable + ".exe"}, "windows", "irm https://epismo.ai/install.ps1 | iex"},
		{installation{Method: "powershell", Executable: customExecutable + ".exe"}, "windows", "$env:EPISMO_INSTALL_DIR='" + filepath.Dir(customExecutable) + "'; irm https://epismo.ai/install.ps1 | iex"},
	}
	for _, test := range tests {
		if got := updateInstructionForOS(test.install, test.goos, home); got != test.want {
			t.Errorf("updateInstructionForOS(%#v, %q) = %q, want %q", test.install, test.goos, got, test.want)
		}
	}
}

func TestUpdateInstructionWithoutKnownHomeDirectory(t *testing.T) {
	executable := filepath.Join(string(filepath.Separator), "usr", "local", "bin", "epismo")
	got := updateInstructionForOS(installation{Method: "standalone", Executable: executable}, "linux", "")
	want := "curl -fsSL https://epismo.ai/install.sh | EPISMO_INSTALL_DIR='" + filepath.Dir(executable) + "' sh"
	if got != want {
		t.Fatalf("updateInstructionForOS with unknown home = %q, want %q", got, want)
	}
}

func TestVersionComparison(t *testing.T) {
	for _, test := range []struct {
		latest, current string
		newer           bool
	}{
		{"1.2.4", "1.2.3", true},
		{"2.0.0", "1.99.99", true},
		{"1.2.3", "1.2.3-beta.1", true},
		{"1.2.3-beta.2", "1.2.3-beta.1", true},
		{"1.2.3-beta.10", "1.2.3-beta.2", true},
		{"1.2.3", "1.2.3", false},
		{"1.2.2", "1.2.3", false},
		{"invalid", "1.2.3", false},
	} {
		if got := isNewerVersion(test.latest, test.current); got != test.newer {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", test.latest, test.current, got, test.newer)
		}
	}
}

func TestFetchLatestVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		_, _ = io.WriteString(w, `{"tag_name":"v1.7.2"}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_UPDATE_API_URL", server.URL)
	version, err := fetchLatestVersion(updateCheckInterval)
	if err != nil || version != "1.7.2" {
		t.Fatalf("fetchLatestVersion = %q, %v", version, err)
	}
}

func TestFetchLatestVersionFromRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/releases/latest" {
			http.Redirect(w, request, "/releases/tag/v1.8.0", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	version, err := fetchLatestVersionFromRedirect(context.Background(), server.URL+"/releases/latest")
	if err != nil || version != "1.8.0" {
		t.Fatalf("fetchLatestVersionFromRedirect = %q, %v", version, err)
	}
}

func TestInteractiveUpdateCheckCachesAndWarns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"tag_name":"v1.1.0"}`)
	}))
	defer server.Close()
	t.Setenv("EPISMO_UPDATE_API_URL", server.URL)
	t.Setenv("EPISMO_CONFIG_DIR", t.TempDir())
	var stderr bytes.Buffer
	a := &app{version: "1.0.0", stderr: &stderr, updateTTY: true}
	a.maybeCheckForUpdate([]string{"whoami"})
	if !strings.Contains(stderr.String(), "UPDATE_AVAILABLE") || !strings.Contains(stderr.String(), "epismo update") {
		t.Fatalf("warning = %s", stderr.String())
	}
	if _, found := readUpdateCache(); !found {
		t.Fatal("update cache was not written")
	}
}
