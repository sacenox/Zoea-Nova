# Shell Commands and Completions in Go

This document covers how to execute shell commands and implement shell completions in pure Go without requiring CGO.

## Executing Shell Commands

Go's standard library provides `os/exec` for running external commands. This is pure Go and requires no CGO.

### Basic Command Execution

```go
import "os/exec"

// Simple command with arguments
cmd := exec.Command("ls", "-la", "/tmp")
output, err := cmd.Output()
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(output))
```

### Shell Pipelines

```go
// Run command through shell for pipelines
cmd := exec.Command("bash", "-c", "echo hello | grep h")
output, err := cmd.CombinedOutput()
```

### Interactive Commands

```go
// Connect stdin/stdout/stderr
cmd := exec.Command("/bin/bash")
cmd.Stdin = os.Stdin
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
err := cmd.Run()
```

### With Timeout

```go
import "context"

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

cmd := exec.CommandContext(ctx, "long-running-command")
output, err := cmd.Output()
if ctx.Err() == context.DeadlineExceeded {
    log.Println("Command timed out")
}
```

### Capturing Output Separately

```go
var stdout, stderr bytes.Buffer
cmd := exec.Command("some-command")
cmd.Stdout = &stdout
cmd.Stderr = &stderr

err := cmd.Run()
fmt.Println("stdout:", stdout.String())
fmt.Println("stderr:", stderr.String())
```

## MCP Tool Example

If Zoea Nova needed a shell execution tool for Myses:

```go
// internal/mcp/tools.go

func (r *Registry) registerShellTool() {
    r.AddTool(Tool{
        Name:        "run_command",
        Description: "Execute a shell command",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "command": map[string]interface{}{
                    "type":        "string",
                    "description": "Shell command to execute",
                },
                "timeout": map[string]interface{}{
                    "type":        "number",
                    "description": "Timeout in seconds (default 30)",
                },
            },
            "required": []string{"command"},
        },
        Handler: r.executeCommand,
    })
}

func (r *Registry) executeCommand(args map[string]interface{}) (interface{}, error) {
    command := args["command"].(string)
    timeout := 30
    if t, ok := args["timeout"].(float64); ok {
        timeout = int(t)
    }

    // Security: validate/sanitize command
    // Consider allowlist of safe commands or restricted shell

    ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, "bash", "-c", command)
    output, err := cmd.CombinedOutput()

    result := map[string]interface{}{
        "output":   string(output),
        "exitCode": 0,
    }

    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            result["exitCode"] = exitErr.ExitCode()
        } else {
            return nil, err
        }
    }

    return result, nil
}
```

**Security Note**: Shell execution tools should be carefully designed with security in mind. Consider:
- Command allowlists
- Restricted shell environments
- Input sanitization
- Resource limits (timeout, memory, CPU)

## Shell Completions

Shell completions help users by auto-completing commands, flags, and arguments. All approaches below are pure Go.

### Option 1: Cobra Framework (Recommended)

[Cobra](https://github.com/spf13/cobra) is the standard for Go CLI applications and includes built-in completion generation.

```go
import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
    Use:   "zoea",
    Short: "Zoea Nova - AI swarm controller",
}

func init() {
    // Add flags
    rootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug logging")
    rootCmd.PersistentFlags().StringP("config", "c", "", "Config file path")

    // Add completion command
    rootCmd.AddCommand(&cobra.Command{
        Use:   "completion [bash|zsh|fish|powershell]",
        Short: "Generate completion script",
        Long: `Generate shell completion script.

Example usage:
  # Bash
  zoea completion bash > /etc/bash_completion.d/zoea

  # Zsh
  zoea completion zsh > "${fpath[1]}/_zoea"

  # Fish
  zoea completion fish > ~/.config/fish/completions/zoea.fish
`,
        Args: cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            switch args[0] {
            case "bash":
                rootCmd.GenBashCompletion(os.Stdout)
            case "zsh":
                rootCmd.GenZshCompletion(os.Stdout)
            case "fish":
                rootCmd.GenFishCompletion(os.Stdout, true)
            case "powershell":
                rootCmd.GenPowerShellCompletion(os.Stdout)
            default:
                fmt.Fprintf(os.Stderr, "Unsupported shell: %s\n", args[0])
                os.Exit(1)
            }
        },
    })
}
```

**Installation:**
```bash
# Bash
zoea completion bash | sudo tee /etc/bash_completion.d/zoea

# Zsh
zoea completion zsh > "${fpath[1]}/_zoea"

# Fish
zoea completion fish > ~/.config/fish/completions/zoea.fish
```

### Option 2: posener/complete (Lightweight)

[posener/complete](https://github.com/posener/complete) is a lightweight pure Go completion library.

```go
import "github.com/posener/complete"

func main() {
    zoea := complete.Command{
        Flags: complete.Flags{
            "-debug":  complete.PredictNothing,
            "-config": complete.PredictFiles("*.toml"),
        },
        Sub: complete.Commands{
            "start": complete.Command{},
            "stop":  complete.Command{},
            "list":  complete.Command{},
        },
    }

    cmp := complete.New("zoea", zoea)
    cmp.Run()
}
```

### Option 3: Manual Generation

For full control, generate completion scripts manually:

```go
const bashCompletion = `
_zoea_completion() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="-debug -config -help"

    case "${prev}" in
        -config)
            COMPREPLY=( $(compgen -f -X '!*.toml' -- ${cur}) )
            return 0
            ;;
        *)
            ;;
    esac

    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
}
complete -F _zoea_completion zoea
`

func generateCompletion(shell string) {
    switch shell {
    case "bash":
        fmt.Println(bashCompletion)
    case "zsh":
        fmt.Println(zshCompletion)
    case "fish":
        fmt.Println(fishCompletion)
    default:
        fmt.Fprintf(os.Stderr, "Unsupported shell: %s\n", shell)
        os.Exit(1)
    }
}
```

## Zoea Nova Context

Zoea Nova is currently a TUI application, not a CLI with subcommands. Shell completions would be most useful if:

1. **CLI commands are added** - e.g., `zoea list`, `zoea create`, `zoea delete`
2. **Flag completion** - Complete `-debug`, `-config`, etc.
3. **Config file paths** - Complete `*.toml` files for `-config` flag

**Recommendation**: If CLI commands are added in the future, use **Cobra** for automatic completion generation. For now, basic flag completion can wait until needed.

## Testing Completions

```bash
# Test bash completion manually
source <(zoea completion bash)
zoea <TAB><TAB>

# Test with specific shell
bash -c "source <(zoea completion bash) && complete -p zoea"
```

## References

- [os/exec Package](https://pkg.go.dev/os/exec)
- [Cobra CLI Framework](https://github.com/spf13/cobra)
- [posener/complete](https://github.com/posener/complete)
- [Bash Completion Guide](https://www.gnu.org/software/bash/manual/html_node/Programmable-Completion.html)
