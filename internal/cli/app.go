package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

type app struct {
	version            string
	distribution       string
	stdin              io.Reader
	stdout             io.Writer
	stderr             io.Writer
	client             *apiClient
	isTTY              bool
	updateTTY          bool
	options            globalOptions
	workspaceAnnounced bool
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
	parsedArgs, options, err := extractGlobalOptions(args)
	if err != nil {
		return printAppError(&app{stderr: stderr, options: options}, err)
	}
	args = parsedArgs
	a := &app{version: version, distribution: distribution, stdin: stdin, stdout: stdout, stderr: stderr, client: newAPIClient(version), isTTY: terminalReader(stdin) && terminalWriter(stderr), updateTTY: terminalWriter(stdout) && terminalWriter(stderr), options: options}
	if !options.DryRun {
		defer a.maybeCheckForUpdate(args)
	}
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
	if args[0] == "docs" {
		if options.DryRun {
			return printAppError(a, dryRunNotSupportedError("docs"))
		}
		return printDocs(a, commands, args[1:])
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
		return printAppError(a, unknownCommandError(commands, args, prefix))
	}
	rest := args[consumed:]
	rest = normalizeConvenienceArgs(cmd, rest)
	if options.DryRun && !cmd.Safety.DryRun {
		return printAppError(a, dryRunNotSupportedError(cmd.Path))
	}
	if containsHelpFlag(cmd, rest) {
		printCommandHelp(stdout, cmd)
		return 0
	}
	if options.Schema || containsSchemaFlag(rest) {
		return printCommandSchema(stdout, cmd)
	}
	invocation, err := parseInvocation(cmd, rest, stdin)
	if err != nil {
		return printAppError(a, err)
	}
	if cmd.Prepare != nil {
		invocation, err = cmd.Prepare(invocation)
		if err != nil {
			return printAppError(a, err)
		}
	}
	if options.DryRun {
		payload, payloadErr := invocation.payload(a)
		if payloadErr != nil {
			return printAppError(a, payloadErr)
		}
		preview := map[string]any{"dryRun": true, "command": cmd.Path, "arguments": invocation.Positionals}
		if len(payload) > 0 {
			preview["request"] = payload
		}
		return printResultOrError(a, preview, nil)
	}
	if cmd.Safety.Confirmation && a.isTTY && !options.Yes && invocation.text("_input") != "-" {
		if !confirm(a, cmd.Path) {
			return printAppError(a, &Error{Code: "CONFIRMATION_REQUIRED", Message: "Operation cancelled.", Hint: "Re-run with --yes to proceed without a prompt.", ExitCode: 1})
		}
	}
	result, err := cmd.Run(a, invocation)
	if err != nil {
		return printAppError(a, err)
	}
	if result != nil {
		if cmd.Output == outputRaw {
			_, err = io.WriteString(stdout, fmt.Sprint(result))
		} else {
			err = printResult(stdout, result, options)
		}
		if err != nil {
			return printAppError(a, err)
		}
	}
	return 0
}

func dryRunNotSupportedError(command string) *Error {
	return &Error{
		Code:     "DRY_RUN_NOT_SUPPORTED",
		Message:  "--dry-run is not supported for this command.",
		Hint:     "Run the command without --dry-run.",
		Details:  map[string]any{"command": command},
		ExitCode: 1,
	}
}

func printDocs(a *app, commands []*command, args []string) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		if cmd, consumed := resolveCommand(commands, []string{"docs"}); cmd != nil && consumed == 1 {
			printCommandHelp(a.stdout, cmd)
			return 0
		}
	}
	if len(args) == 0 {
		_, _ = io.WriteString(a.stdout, "https://github.com/epismoai/cli#readme\n")
		return 0
	}
	commandArgs := args
	if len(commandArgs) > 1 && (commandArgs[len(commandArgs)-1] == "--help" || commandArgs[len(commandArgs)-1] == "-h") {
		commandArgs = commandArgs[:len(commandArgs)-1]
	}
	cmd, consumed := resolveCommand(commands, commandArgs)
	if cmd != nil && consumed == len(commandArgs) {
		printCommandHelp(a.stdout, cmd)
		return 0
	}
	prefix := strings.Join(args[:groupPrefixLength(commands, args)], " ")
	if prefix != "" {
		printGroupHelp(a.stdout, prefix, commands)
		return 0
	}
	return printAppError(a, &Error{Code: "UNKNOWN_COMMAND", Message: "Unknown command: " + strings.Join(args, " ") + ".", Hint: "Run `epismo --help` for available commands.", ExitCode: 1})
}

func normalizeConvenienceArgs(cmd *command, args []string) []string {
	if cmd.Path != "playbook search" {
		return args
	}
	normalized := append([]string(nil), args...)
	for index, arg := range normalized {
		if arg == "-q" {
			normalized[index] = "--query"
		}
	}
	if contains(normalized, "--query") || containsPrefix(normalized, "--query=") {
		return normalized
	}
	byName := map[string]optionSpec{}
	for _, option := range baseOptions(cmd) {
		byName[option.Name] = option
	}
	result := make([]string, 0, len(args)+1)
	for index := 0; index < len(normalized); index++ {
		arg := normalized[index]
		if !strings.HasPrefix(arg, "-") {
			result = append(result, "--query", arg)
			result = append(result, normalized[index+1:]...)
			return result
		}
		result = append(result, arg)
		name, _, hasEquals := strings.Cut(arg, "=")
		if option, ok := byName[name]; ok && option.Kind != kindBoolean && !hasEquals && index+1 < len(normalized) {
			index++
			result = append(result, normalized[index])
		}
	}
	return result
}

func containsPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func unknownCommandError(commands []*command, args []string, prefix string) *Error {
	entered := strings.Join(args, " ")
	hint := "Run `epismo --help` for available commands."
	if prefix != "" {
		hint = "Run `epismo " + prefix + " --help` for available commands."
	}
	candidates := make([]string, 0)
	for _, cmd := range commands {
		if commandDistance(entered, cmd.Path) <= 3 {
			candidates = append(candidates, cmd.Path)
		}
	}
	if len(candidates) == 0 && len(args) > 0 {
		for _, word := range buildCommandWords() {
			if commandDistance(args[0], word) <= 3 {
				candidates = append(candidates, word)
			}
		}
	}
	if len(candidates) > 0 {
		hint = "Did you mean `epismo " + candidates[0] + "`?"
	}
	return &Error{Code: "UNKNOWN_COMMAND", Message: "Unknown command: " + entered + ".", Hint: hint, ExitCode: 1}
}

func containsSchemaFlag(rest []string) bool {
	for _, arg := range rest {
		if arg == "--schema" {
			return true
		}
	}
	return false
}

func printCommandSchema(w io.Writer, cmd *command) int {
	options := make([]any, 0, len(baseOptions(cmd)))
	for _, option := range baseOptions(cmd) {
		required := option.Required && option.RequiredAlternative == ""
		entry := map[string]any{"name": option.Name, "required": required, "kind": optionKindName(option.Kind), "choices": option.Choices, "default": option.Default, "description": option.Help}
		if option.RequiredAlternative != "" {
			entry["requiredUnless"] = option.RequiredAlternative
		}
		options = append(options, entry)
	}
	if err := printJSON(w, map[string]any{"command": cmd.Path, "arguments": cmd.Args, "options": options, "examples": cmd.Examples}); err != nil {
		return 1
	}
	return 0
}

func optionKindName(kind optionKind) string {
	switch kind {
	case kindInteger:
		return "integer"
	case kindBoolean:
		return "boolean"
	case kindObject, kindObjectSource:
		return "object"
	case kindArray:
		return "array"
	case kindCSV:
		return "string_array"
	default:
		return "string"
	}
}

func printResultOrError(a *app, result any, err error) int {
	if err != nil {
		return printAppError(a, err)
	}
	if outputErr := printResult(a.stdout, result, a.options); outputErr != nil {
		return printAppError(a, outputErr)
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
			if option, ok := byName[name]; ok && option.Kind != kindBoolean && !hasEquals {
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
	workspaceRef := strings.TrimSpace(a.options.Workspace)
	if workspaceRef == "" {
		workspaceRef = strings.TrimSpace(os.Getenv("EPISMO_WORKSPACE"))
	}
	if workspaceRef == "" && envToken == "" && config.DefaultWorkspace != nil {
		workspaceID = config.DefaultWorkspace.ID
	}
	if envToken != "" && config.DefaultWorkspace != nil {
		a.event("warning", "EPISMO_TOKEN_WORKSPACE_IGNORED", "EPISMO_TOKEN is set — saved default workspace \""+config.DefaultWorkspace.Handle+"\" is ignored. Use epismo token create --workspace-id to issue a workspace-scoped token.", nil)
	}
	auth, err := a.resolveAuthentication()
	if err != nil {
		return executionContext{}, err
	}
	if workspaceRef != "" {
		resolved, err := a.resolveWorkspaceRef(auth.AccessToken, workspaceRef)
		if err != nil {
			return executionContext{}, err
		}
		workspaceID = resolved
	}
	if a.options.DiagnosticFormat == "human" && a.isTTY && !a.workspaceAnnounced && workspaceID != "" {
		label := workspaceID
		if workspaceRef != "" {
			label = workspaceRef
		} else if config.DefaultWorkspace != nil && config.DefaultWorkspace.Handle != "" {
			label = config.DefaultWorkspace.Handle
		}
		_, _ = fmt.Fprintf(a.stderr, "Workspace: %s\n", label)
		a.workspaceAnnounced = true
	}
	return executionContext{Auth: auth, WorkspaceID: workspaceID}, nil
}

func (a *app) event(level, code, message string, details map[string]any) {
	if a.options.Quiet {
		return
	}
	if a.options.DiagnosticFormat == "human" {
		_, _ = fmt.Fprintln(a.stderr, message)
		for _, key := range sortedKeys(details) {
			_, _ = fmt.Fprintf(a.stderr, "%s: %s\n", key, scalarString(details[key]))
		}
		return
	}
	printEvent(a.stderr, level, code, message, details)
}

func (a *app) resolveWorkspaceRef(token, ref string) (string, error) {
	requested := strings.TrimSpace(ref)
	items, err := a.workspaces(token)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if stringField(item, "id") == requested || strings.EqualFold(stringField(item, "handle"), requested) {
			return stringField(item, "id"), nil
		}
	}
	hint := "Run `epismo workspace list` to see accessible workspaces."
	if suggestions := workspaceSuggestions(items, requested); len(suggestions) > 0 {
		hint = "Did you mean: " + strings.Join(suggestions, ", ") + "?"
	}
	return "", &Error{Code: "WORKSPACE_NOT_FOUND", Message: "Workspace not found or not accessible: " + requested, Hint: hint, ExitCode: 1}
}

func confirm(a *app, path string) bool {
	_, _ = fmt.Fprintf(a.stderr, "This will run `epismo %s`. Continue? [y/N] ", path)
	answer, _ := bufio.NewReader(a.stdin).ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(answer), "y") || strings.EqualFold(strings.TrimSpace(answer), "yes")
}
