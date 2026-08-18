package cli

import (
	"io"
	"os"
	"strings"
)

type app struct {
	version      string
	distribution string
	stdin        io.Reader
	stdout       io.Writer
	stderr       io.Writer
	client       *apiClient
	isTTY        bool
	updateTTY    bool
}

type authContext struct {
	UserID      string
	AccessToken string
	ExpiresAt   string
	AccountID   string
}

type executionContext struct {
	Auth        authContext
	WorkspaceID string
}

func Main(args []string, version string, stdin io.Reader, stdout, stderr io.Writer) int {
	distribution := "development"
	if isReleaseVersion(version) {
		distribution = "release"
	}
	return MainWithDistribution(args, version, distribution, stdin, stdout, stderr)
}

func MainWithDistribution(args []string, version, distribution string, stdin io.Reader, stdout, stderr io.Writer) int {
	a := &app{version: version, distribution: distribution, stdin: stdin, stdout: stdout, stderr: stderr, client: newAPIClient(version), isTTY: terminalReader(stdin) && terminalWriter(stderr), updateTTY: terminalWriter(stdout) && terminalWriter(stderr)}
	defer a.maybeCheckForUpdate(args)
	commands := buildCommands()
	if len(args) == 0 {
		printGroupHelp(stdout, "", commands)
		return 0
	}
	if args[0] == "--version" || args[0] == "-V" {
		_, _ = io.WriteString(stdout, version+"\n")
		return 0
	}
	if args[0] == "--help" || args[0] == "-h" {
		printGroupHelp(stdout, "", commands)
		return 0
	}

	cmd, consumed := resolveCommand(commands, args)
	if cmd == nil {
		prefix := strings.Join(args[:groupPrefixLength(commands, args)], " ")
		if prefix == strings.Join(args, " ") {
			printGroupHelp(stdout, prefix, commands)
			return 0
		}
		if len(args) > 0 && (args[len(args)-1] == "--help" || args[len(args)-1] == "-h") {
			printGroupHelp(stdout, prefix, commands)
			return 0
		}
		return printError(stderr, &Error{Code: "UNKNOWN_COMMAND", Message: "unknown command: " + strings.Join(args, " "), Hint: "Run `epismo --help` for available commands.", ExitCode: 1})
	}
	rest := args[consumed:]
	if containsHelpFlag(cmd, rest) {
		printCommandHelp(stdout, cmd)
		return 0
	}
	invocation, err := parseInvocation(cmd, rest, stdin)
	if err != nil {
		return printError(stderr, err)
	}
	result, err := cmd.Run(a, invocation)
	if err != nil {
		return printError(stderr, err)
	}
	if result != nil {
		if err := printJSON(stdout, result); err != nil {
			return printError(stderr, err)
		}
	}
	return 0
}

// containsHelpFlag reports whether rest requests help, without mistaking a
// value passed to a preceding option (e.g. --title -h) for the help flag.
func containsHelpFlag(cmd *command, rest []string) bool {
	byName := map[string]optionSpec{}
	for _, option := range baseOptions(cmd) {
		byName[option.Name] = option
	}
	for index := 0; index < len(rest); index++ {
		argument := rest[index]
		if argument == "--" {
			return false
		}
		if argument == "--help" || argument == "-h" {
			return true
		}
		if strings.HasPrefix(argument, "--") {
			name, _, hasEquals := strings.Cut(argument, "=")
			if _, ok := byName[name]; ok && !hasEquals {
				index++
			}
		}
	}
	return false
}

func terminalReader(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func terminalWriter(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func resolveCommand(commands []*command, args []string) (*command, int) {
	var found *command
	consumed := 0
	for _, cmd := range commands {
		parts := strings.Fields(cmd.Path)
		if len(parts) > len(args) {
			continue
		}
		matches := true
		for index := range parts {
			if parts[index] != args[index] {
				matches = false
				break
			}
		}
		if matches && len(parts) > consumed {
			found, consumed = cmd, len(parts)
		}
	}
	return found, consumed
}

func groupPrefixLength(commands []*command, args []string) int {
	length := 0
	for index := 1; index <= len(args); index++ {
		prefix := strings.Join(args[:index], " ")
		for _, cmd := range commands {
			if strings.HasPrefix(cmd.Path, prefix+" ") {
				length = index
				break
			}
		}
	}
	return length
}

func (a *app) context() (executionContext, error) {
	envToken := strings.TrimSpace(os.Getenv("EPISMO_TOKEN"))
	config, err := readConfig()
	if err != nil && envToken == "" {
		return executionContext{}, err
	}
	workspaceID := ""
	if envToken == "" && config.DefaultWorkspace != nil {
		workspaceID = config.DefaultWorkspace.ID
	}
	if envToken != "" && config.DefaultWorkspace != nil {
		printWarning(a.stderr, "EPISMO_TOKEN_WORKSPACE_IGNORED", "EPISMO_TOKEN is set — saved default workspace \""+config.DefaultWorkspace.Handle+"\" is ignored. Use epismo token create --workspace-id to issue a workspace-scoped token.")
	}
	auth, err := a.resolveAuthentication()
	if err != nil {
		return executionContext{}, err
	}
	return executionContext{Auth: auth, WorkspaceID: workspaceID}, nil
}
