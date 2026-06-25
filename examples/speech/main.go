package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "speech example failed: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printRootUsage(stdout)
		return nil
	}

	command := strings.ToLower(strings.TrimSpace(args[0]))
	if command == "help" || command == "-h" || command == "--help" {
		printRootUsage(stdout)
		return nil
	}

	if strings.HasPrefix(command, "-") {
		return runHTTPCommand(args, stdout, stderr)
	}

	subArgs := args[1:]
	switch command {
	case "http":
		return runHTTPCommand(subArgs, stdout, stderr)
	case "stream":
		return runStreamCommand(subArgs, stdout, stderr)
	case "websocket", "ws":
		return runWebSocketCommand(subArgs, stdout, stderr)
	case "async":
		return runAsyncCommand(subArgs, stdout, stderr)
	case "task":
		return runTaskAliasCommand(subArgs, stdout, stderr)
	default:
		printRootUsage(stderr)
		return fmt.Errorf("unknown speech command %q", command)
	}
}

func printRootUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  go run ./examples/speech <command> [flags]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Commands:")
	fmt.Fprintln(out, "  async   submit async task or query existing task_id")
	fmt.Fprintln(out, "  stream  synthesize by SSE stream and write merged audio")
	fmt.Fprintln(out, "  websocket synthesize by official WebSocket protocol")
	fmt.Fprintln(out, "  task    alias of async task query mode (deprecated)")
	fmt.Fprintln(out, "  http    synthesize via synchronous HTTP and write audio")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Examples:")
	fmt.Fprintln(out, "  go run ./examples/speech async -text \"hello\" -voice-id \"male-qn-qingse\"")
	fmt.Fprintln(out, "  go run ./examples/speech stream -text \"hello\" -voice-id \"male-qn-qingse\"")
	fmt.Fprintln(out, "  go run ./examples/speech websocket -text \"hello\" -voice-id \"male-qn-qingse\"")
	fmt.Fprintln(out, "  go run ./examples/speech async -task-id 123456789 -wait")
	fmt.Fprintln(out, "  go run ./examples/speech http -text \"hello\" -voice-id \"male-qn-qingse\"")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Backward compatibility:")
	fmt.Fprintln(out, "  go run ./examples/speech -text \"hello\" ...  # same as http")
}
