package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	updateCheckInterval = 24 * time.Hour
	updateDocsURL       = "https://github.com/epismoai/cli#updating"
	githubLatestURL     = "https://api.github.com/repos/epismoai/cli/releases/latest"
	githubReleaseURL    = "https://github.com/epismoai/cli/releases/latest"

	// updateCheckBlockingBudget bounds how long a command will wait on the
	// background update-check goroutine before exiting anyway. A fast
	// network finishes well within this and the cache gets refreshed (and
	// the warning printed) as part of this run; a slow one just means the
	// command exits promptly and the check is retried on the next stale run,
	// rather than adding the full fetch timeout to every command's exit time.
	updateCheckBlockingBudget = 1 * time.Second
)

type updateCache struct {
	LastChecked   int64  `json:"lastChecked"`
	LatestVersion string `json:"latestVersion"`
}

type releaseVersion struct {
	core       [3]int
	prerelease string
}

var releaseVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$`)

func updateCachePath() string { return filepath.Join(configDir(), "cache", "update.json") }

func updateCommand() *command {
	return &command{Path: "update", Summary: "check for an update and show the correct installation command", Run: func(a *app, _ invocation) (any, error) {
		return a.updateCLI()
	}}
}

func (a *app) updateCLI() (any, error) {
	if !isReleaseVersion(a.version) {
		return nil, &Error{Code: "UPDATE_UNAVAILABLE", Message: "Update checks are unavailable for development builds.", Hint: "Install a released Epismo CLI before using this command.", ExitCode: 1}
	}
	latest, err := fetchLatestVersion(10 * time.Second)
	if err != nil {
		return nil, &Error{Code: "UPDATE_CHECK_FAILED", Message: "Could not determine the latest Epismo CLI version.", Retryable: true, Hint: updateDocsURL, Cause: err, ExitCode: 1}
	}
	result := map[string]any{"currentVersion": a.version, "latestVersion": latest}
	install := detectInstallation(a.version, a.distribution)
	result["installationMethod"] = installationLabel(install)
	if install.Scope != "" {
		result["installationScope"] = install.Scope
	}
	if !isNewerVersion(latest, a.version) {
		result["status"] = "up_to_date"
		return map[string]any{"update": result}, nil
	}

	result["status"] = "action_required"
	if command := updateInstruction(install); command != "" {
		result["command"] = command
	}
	result["instructions"] = updateDocsURL
	return map[string]any{"update": result}, nil
}

func installationLabel(install installation) string {
	if install.Method == "node" && install.Manager != "" && install.Manager != "unknown" {
		return install.Manager
	}
	return install.Method
}

func updateInstruction(install installation) string {
	home, _ := os.UserHomeDir()
	return updateInstructionForOS(install, runtime.GOOS, home)
}

func updateInstructionForOS(install installation, goos, home string) string {
	switch install.Method {
	case "node":
		if install.Scope != "global" {
			return ""
		}
		switch install.Manager {
		case "pnpm":
			return "pnpm add -g epismo@latest"
		case "yarn":
			if isYarnClassic(install.ManagerVersion) {
				return "yarn global add epismo@latest"
			}
			return ""
		case "bun":
			return "bun add -g epismo@latest"
		case "npm":
			return "npm install -g epismo@latest"
		default:
			return ""
		}
	case "go":
		return "go install github.com/epismoai/cli/cmd/epismo@latest"
	case "homebrew":
		return "brew upgrade epismo"
	case "scoop":
		return "scoop update epismo"
	case "curl", "powershell", "standalone":
		directory := filepath.Clean(filepath.Dir(install.Executable))
		if isDefaultInstallDirectory(directory, home, goos) {
			if goos == "windows" {
				return "irm https://epismo.ai/install.ps1 | iex"
			}
			return "curl -fsSL https://epismo.ai/install.sh | sh"
		}
		return installDirCommand(goos, directory)
	default:
		return ""
	}
}

// isDefaultInstallDirectory reports whether directory is the default
// $HOME/.local/bin location install.sh and install.ps1 use. It requires a
// known home directory: without one (os.UserHomeDir failed, e.g. no
// HOME/USERPROFILE in a minimal container) there's no way to tell, so the
// caller falls back to the explicit EPISMO_INSTALL_DIR form, which is always
// correct even though it's more verbose than strictly necessary.
func isDefaultInstallDirectory(directory, home, goos string) bool {
	if home == "" {
		return false
	}
	defaultDirectory := filepath.Clean(filepath.Join(home, ".local", "bin"))
	if goos == "windows" {
		return strings.EqualFold(directory, defaultDirectory)
	}
	return directory == defaultDirectory
}

func installDirCommand(goos, directory string) string {
	if goos == "windows" {
		directory = strings.ReplaceAll(directory, "'", "''")
		return "$env:EPISMO_INSTALL_DIR='" + directory + "'; irm https://epismo.ai/install.ps1 | iex"
	}
	directory = strings.ReplaceAll(directory, "'", "'\\''")
	return "curl -fsSL https://epismo.ai/install.sh | EPISMO_INSTALL_DIR='" + directory + "' sh"
}

func isYarnClassic(version string) bool {
	major, _, ok := strings.Cut(strings.TrimSpace(version), ".")
	if !ok {
		major = strings.TrimSpace(version)
	}
	parsed, err := strconv.Atoi(major)
	return err == nil && parsed == 1
}

func (a *app) maybeCheckForUpdate(args []string) {
	if a.options.Quiet || !a.updateTTY || strings.TrimSpace(os.Getenv("EPISMO_UPDATE_CHECK")) == "0" || !isReleaseVersion(a.version) {
		return
	}
	if len(args) > 0 && args[0] == "update" {
		return
	}
	cache, found := readUpdateCache()
	now := time.Now()
	displayed := found && isNewerVersion(cache.LatestVersion, a.version)
	if displayed {
		a.event("warning", "UPDATE_AVAILABLE", fmt.Sprintf("Update available: %s → %s. Run: epismo update", a.version, cache.LatestVersion), nil)
	}
	if found && now.Sub(time.UnixMilli(cache.LastChecked)) <= updateCheckInterval {
		return
	}
	refreshUpdateCache(a, now, displayed)
}

// refreshUpdateCache fetches the latest version and refreshes the cache on a
// background goroutine, waiting only up to updateCheckBlockingBudget rather
// than the full fetch timeout. This keeps the (roughly once-per-24h)
// interactive update check from adding its full network latency to every
// command's exit time.
func refreshUpdateCache(a *app, now time.Time, displayed bool) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		latest, err := fetchLatestVersion(2 * time.Second)
		if err != nil {
			return
		}
		_ = writeJSONAtomic(updateCachePath(), updateCache{LastChecked: now.UnixMilli(), LatestVersion: latest})
		if !displayed && isNewerVersion(latest, a.version) {
			a.event("warning", "UPDATE_AVAILABLE", fmt.Sprintf("Update available: %s → %s. Run: epismo update", a.version, latest), nil)
		}
	}()
	select {
	case <-done:
	case <-time.After(updateCheckBlockingBudget):
	}
}

func readUpdateCache() (updateCache, bool) {
	var cache updateCache
	found, err := readJSONFile(updateCachePath(), &cache)
	if err != nil || !found || cache.LastChecked <= 0 || !isReleaseVersion(cache.LatestVersion) {
		return updateCache{}, false
	}
	return cache, true
}

func fetchLatestVersion(timeout time.Duration) (string, error) {
	endpoint := strings.TrimSpace(os.Getenv("EPISMO_UPDATE_API_URL"))
	if endpoint != "" {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return fetchLatestVersionFromAPI(ctx, endpoint)
	}
	apiCtx, apiCancel := context.WithTimeout(context.Background(), timeout)
	defer apiCancel()
	version, apiErr := fetchLatestVersionFromAPI(apiCtx, githubLatestURL)
	if apiErr == nil {
		return version, nil
	}
	// The redirect fallback gets its own fresh timeout budget rather than
	// inheriting whatever remains of the API call's deadline, so a slow (but
	// ultimately failing) API request can't starve the fallback of time to
	// even attempt a request.
	redirectCtx, redirectCancel := context.WithTimeout(context.Background(), timeout)
	defer redirectCancel()
	version, redirectErr := fetchLatestVersionFromRedirect(redirectCtx, githubReleaseURL)
	if redirectErr == nil {
		return version, nil
	}
	return "", fmt.Errorf("GitHub API: %v; release redirect: %w", apiErr, redirectErr)
}

func fetchLatestVersionFromAPI(ctx context.Context, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "epismo-update-check")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("latest release returned HTTP %d", response.StatusCode)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return "", err
	}
	version := strings.TrimPrefix(strings.TrimSpace(payload.TagName), "v")
	if !isReleaseVersion(version) {
		return "", errors.New("latest release has an invalid version")
	}
	return version, nil
}

func fetchLatestVersionFromRedirect(ctx context.Context, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "epismo-update-check")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	const marker = "/releases/tag/"
	path := response.Request.URL.Path
	index := strings.LastIndex(path, marker)
	if index < 0 {
		return "", errors.New("latest release redirect did not contain a version tag")
	}
	version := strings.TrimPrefix(strings.TrimSpace(path[index+len(marker):]), "v")
	if !isReleaseVersion(version) {
		return "", errors.New("latest release redirect has an invalid version")
	}
	return version, nil
}

func isReleaseVersion(version string) bool {
	_, ok := parseReleaseVersion(version)
	return ok
}

func isNewerVersion(latest, current string) bool {
	l, lok := parseReleaseVersion(latest)
	c, cok := parseReleaseVersion(current)
	if !lok || !cok {
		return false
	}
	for index := range 3 {
		if l.core[index] != c.core[index] {
			return l.core[index] > c.core[index]
		}
	}
	if l.prerelease == c.prerelease {
		return false
	}
	if l.prerelease == "" {
		return true
	}
	if c.prerelease == "" {
		return false
	}
	return comparePrerelease(l.prerelease, c.prerelease) > 0
}

func comparePrerelease(left, right string) int {
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	for index := 0; index < len(leftParts) && index < len(rightParts); index++ {
		if leftParts[index] == rightParts[index] {
			continue
		}
		leftNumber, leftErr := strconv.Atoi(leftParts[index])
		rightNumber, rightErr := strconv.Atoi(rightParts[index])
		switch {
		case leftErr == nil && rightErr == nil:
			if leftNumber > rightNumber {
				return 1
			}
			return -1
		case leftErr == nil:
			return -1
		case rightErr == nil:
			return 1
		case leftParts[index] > rightParts[index]:
			return 1
		default:
			return -1
		}
	}
	if len(leftParts) > len(rightParts) {
		return 1
	}
	if len(leftParts) < len(rightParts) {
		return -1
	}
	return 0
}

func parseReleaseVersion(version string) (releaseVersion, bool) {
	var result releaseVersion
	value := strings.TrimPrefix(strings.TrimSpace(version), "v")
	match := releaseVersionPattern.FindStringSubmatch(value)
	if match == nil {
		return result, false
	}
	for index, part := range match[1:4] {
		value, err := strconv.Atoi(part)
		if err != nil {
			return result, false
		}
		result.core[index] = value
	}
	result.prerelease = match[4]
	return result, true
}
