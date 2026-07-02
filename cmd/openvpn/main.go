// Command openvpn is the functional CLI for the openvpn3-go engine. C/C++
// dependencies are vendored in the module; `bootstrap` is a dev helper that
// writes machine-local OpenSSL cgo flags. On a CGO_ENABLED=1 Windows build,
// `connect` runs a tunnel in-process, streaming OpenVPN3 lifecycle events +
// log messages.
//
// Daemonless: the engine runs IN this process, so `connect` is a FOREGROUND
// session — it blocks streaming events until Ctrl-C, then disconnects. There is
// no separate `status`/`disconnect` (a second process cannot see the in-process
// tunnel); the live event stream IS the status, Ctrl-C IS the disconnect.
//
//	openvpn connect <profile.ovpn> [--user U] [--pass-stdin] [--kill-switch] [--verbose] [--json]
//	openvpn bootstrap [--root DIR]
//	openvpn version
//
// The tunnel requires elevation (TUN + routes): run `connect` from an elevated
// shell. Credentials never travel argv — password is read from a hidden TTY
// prompt, or from stdin with --pass-stdin.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	openvpn3 "github.com/inovacc/openvpn3-go"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "bootstrap":
		os.Exit(cmdBootstrap(os.Args[2:]))
	case "connect":
		os.Exit(cmdConnect(os.Args[2:]))
	case "version", "--version", "-v":
		cmdVersion()
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "openvpn: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `openvpn — openvpn3-go engine CLI

Usage:
  openvpn connect <profile.ovpn> [flags]   connect a tunnel (foreground; streams events)
  openvpn bootstrap [--root DIR]           dev helper: write machine-local OpenSSL cgo flags
  openvpn version                          print version + engine availability

connect flags:
  --user U         VPN username (password read from a hidden prompt, or --pass-stdin)
  --pass-stdin     read the password from stdin instead of prompting
  --kill-switch    request the WFP kill-switch (v0: logged, not yet enforced)
  --verbose        also stream raw log lines
  --json           emit one JSON object per event/log line
`)
}

func cmdBootstrap(args []string) int {
	fs := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	root := fs.String("root", ".", "module checkout root (dir containing go.mod)")
	_ = fs.Parse(args)

	mod, err := os.ReadFile(filepath.Join(*root, "go.mod"))
	if err != nil || !strings.Contains(string(mod), "module github.com/inovacc/openvpn3-go") {
		fmt.Fprintf(os.Stderr, "openvpn bootstrap: %s is not the openvpn3-go checkout root (run from a source checkout, or pass --root)\n", *root)
		return 1
	}
	if err := setupOpenSSL(*root); err != nil {
		fmt.Fprintln(os.Stderr, "openvpn bootstrap: error:", err)
		return 1
	}
	fmt.Fprintln(os.Stderr, "openvpn bootstrap: done")
	return 0
}

func cmdVersion() {
	avail := "no (build CGO_ENABLED=1 -tags cgo on Windows)"
	if openvpn3.Available() {
		avail = "yes"
	}
	fmt.Printf("openvpn3-go %s (%s/%s)\n", openvpn3.Version, runtime.GOOS, runtime.GOARCH)
	fmt.Printf("cgo engine available: %s\n", avail)
}

func cmdConnect(args []string) int {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	user := fs.String("user", "", "VPN username")
	passStdin := fs.Bool("pass-stdin", false, "read password from stdin")
	killSwitch := fs.Bool("kill-switch", false, "request the WFP kill-switch")
	verbose := fs.Bool("verbose", false, "stream raw log lines")
	jsonOut := fs.Bool("json", false, "emit JSON event/log lines")
	_ = fs.Parse(args)

	profilePath := fs.Arg(0)
	if profilePath == "" {
		fmt.Fprintln(os.Stderr, "openvpn connect: a <profile.ovpn> path is required")
		return 2
	}
	content, err := os.ReadFile(profilePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "openvpn connect: read profile:", err)
		return 2
	}

	password, err := readPassword(*user, *passStdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "openvpn connect:", err)
		return 2
	}

	// Stream events/logs. The handlers run on a cgo thread; they only print +
	// signal fatalCh, never re-enter the engine.
	fatalCh := make(chan string, 1)
	openvpn3.SetEventHandler(func(e openvpn3.Event) {
		printEvent(e, *jsonOut)
		if e.Fatal {
			msg := e.Name
			if e.Info != "" {
				msg = e.Name + ": " + e.Info
			}
			select {
			case fatalCh <- msg:
			default:
			}
		}
	})
	if *verbose {
		openvpn3.SetLogHandler(func(s string) { printLog(s, *jsonOut) })
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	h, err := openvpn3.Connect(ctx, openvpn3.ConnectInput{
		ConfigContent: string(content),
		Username:      *user,
		Password:      password,
		KillSwitch:    *killSwitch,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "openvpn connect:", err)
		if errors.Is(err, openvpn3.ErrUnavailable) || errors.Is(err, openvpn3.ErrNotImplemented) {
			return 3
		}
		return 1
	}
	fmt.Fprintf(os.Stderr, "openvpn: connecting %q (Ctrl-C to disconnect)...\n", profilePath)

	exit := 0
	select {
	case <-sigCh:
		fmt.Fprintln(os.Stderr, "\nopenvpn: disconnecting...")
	case msg := <-fatalCh:
		fmt.Fprintln(os.Stderr, "openvpn: fatal:", msg)
		exit = 1
	}

	cancel()
	if err := openvpn3.Disconnect(h); err != nil {
		fmt.Fprintln(os.Stderr, "openvpn: disconnect:", err)
	}
	return exit
}

// readPassword resolves the VPN password without putting it in argv. Empty user
// => no password (autologin/cert/inline-auth profiles). --pass-stdin reads one
// line from stdin; otherwise a hidden TTY prompt is used.
func readPassword(user string, passStdin bool) (string, error) {
	if user == "" {
		return "", nil
	}
	if passStdin {
		sc := bufio.NewScanner(os.Stdin)
		if sc.Scan() {
			return strings.TrimRight(sc.Text(), "\r\n"), nil
		}
		return "", sc.Err()
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("no terminal for password prompt; use --pass-stdin")
	}
	fmt.Fprint(os.Stderr, "Password: ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(b), nil
}

func printEvent(e openvpn3.Event, asJSON bool) {
	if asJSON {
		b, _ := json.Marshal(struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Info  string `json:"info,omitempty"`
			Error bool   `json:"error,omitempty"`
			Fatal bool   `json:"fatal,omitempty"`
		}{"event", e.Name, e.Info, e.Error, e.Fatal})
		fmt.Println(string(b))
		return
	}
	line := "[evt] " + e.Name
	if e.Info != "" {
		line += " info=" + e.Info
	}
	if e.Fatal {
		line += " FATAL"
	} else if e.Error {
		line += " error"
	}
	fmt.Println(line)
}

func printLog(s string, asJSON bool) {
	if asJSON {
		b, _ := json.Marshal(struct {
			Type string `json:"type"`
			Line string `json:"line"`
		}{"log", s})
		fmt.Println(string(b))
		return
	}
	fmt.Println("[log] " + s)
}
