package main

import "os/signal"
import "syscall"
import "strconv"
import "archive/zip"

import (
	"embed"
	"fmt"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"github.com/Mug-Costanza/tuiwall/internal/tmux"
)

var embeddedPresets embed.FS

func checkPlatformSupport() {
	if getTermiosReq == 0 || setTermiosReq == 0 {
		// This means the build tags didn't fire or the OS isn't supported
		fatal(fmt.Errorf("unsupported operating system for PTY operations"))
	}
}

func clearWindowStyles() {
	// -g unsets the global/default style
	_ = exec.Command("tmux", "set-window-option", "-g", "window-style", "default").Run()
	_ = exec.Command("tmux", "set-window-option", "-g", "window-active-style", "default").Run()

	// Also target the current window specifically to be safe
	_ = exec.Command("tmux", "set-window-option", "window-style", "default").Run()
	_ = exec.Command("tmux", "set-window-option", "window-active-style", "default").Run()
}

func ensureExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Add the executable bit (0111) to the existing permissions
	mode := info.Mode() | 0111
	return os.Chmod(path, mode)
}

// Add this to your main.go
func setupSignalHandler() {
	sigChan := make(chan os.Signal, 1)
	// Listen for Ctrl+C (Interrupt) and Kill signals
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		// This runs even if the program is interrupted
		fmt.Println("\nCleaning up UI styles...")
		_ = exec.Command("tmux", "set-window-option", "-g", "window-style", "default").Run()
		_ = exec.Command("tmux", "set-window-option", "-g", "window-active-style", "default").Run()
		os.Exit(0)
	}()
}

func installEmbeddedTemplate() error {
	home, err := presetHomeDir()
	if err != nil {
		return err
	}

	dstDir := filepath.Join(home, "template")
	if _, err := os.Stat(dstDir); err == nil {
		return fmt.Errorf("template already installed")
	}

	_ = os.MkdirAll(dstDir, 0755)

	// Read from the embedded filesystem
	content, err := embeddedPresets.ReadFile("internal/presets/template/template.py")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dstDir, "template.py"), content, 0644)
}

func getPythonCmd() string {
	// Check for python3 first (modern standard)
	if _, err := exec.LookPath("python3"); err == nil {
		return "python3"
	}
	// Fallback to python (older systems/Windows)
	if _, err := exec.LookPath("python"); err == nil {
		return "python"
	}
	// Default to python3 and let the error surface during execution
	return "python3"
}

type PresetMetadata struct {
	Name        string
	Description string
	Category    string
	Author      string
}

func runRecord(name string) {
	if _, err := exec.LookPath("vhs"); err != nil {
		fatal(fmt.Errorf("vhs not found. Please install it to use the record feature.\n" +
			"Visit: https://github.com/charmbracelet/vhs"))
	}

	scriptPath, err := presetScriptPathStrict(name)
	if err != nil {
		fatal(err)
	}

	dir := filepath.Dir(scriptPath)
	tapePath := filepath.Join(dir, "demo.tape")

	// We use a specific export for ZSH/Bash prompts to strip the username
	content := fmt.Sprintf(`Output %s.gif
Set FontSize 20
Set Width 1200
Set Height 600
Set TypingSpeed 0ms

# Start the recording in 'Hide' mode so setup isn't visible
Hide
Type "export PS1='$ '" Enter
Type "export PROMPT='$ '" Enter
Type "export USER=user" Enter
Type "export HOME=/home/user" Enter
Type "clear" Enter

# Start tmux and immediately hide the green status bar
Type "tmux new-session -s vhs_session" Enter
Sleep 1s
Type "tmux set-option status off" Enter
Type "tuiwall set %s" Enter
Type "tuiwall enable" Enter
Type "clear" Enter
# Only show the recording once the environment is clean
Show

# Capture 10 seconds of animation
Sleep 10s

Hide
# Clean up so the GIF ends cleanly without a hanging shell
Type "tmux kill-session" Enter
Show
`, name, name)

	if err := os.WriteFile(tapePath, []byte(content), 0644); err != nil {
		fatal(fmt.Errorf("failed to write tape file: %w", err))
	}

	fmt.Printf("Running vhs to generate %s.gif...\n", name)
	cmd := exec.Command("vhs", tapePath)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fatal(fmt.Errorf("vhs failed: %w", err))
	}

	fmt.Printf("Successfully generated %s.gif in %s\n", name, dir)
}

func printWrapped(text string, indent int, maxWidth int) {
	if maxWidth <= indent {
		maxWidth = 80 // Fallback
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return
	}

	currentLineLength := indent
	for _, word := range words {
		// If the word + a space would exceed the width, start a new line
		if currentLineLength+len(word)+1 > maxWidth {
			fmt.Print("\n" + strings.Repeat(" ", indent))
			currentLineLength = indent
		}

		fmt.Print(word + " ")
		currentLineLength += len(word) + 1
	}
}

func confirmAction(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func presetAttachImage(name, imgPath string) error {
	scriptPath, ok := presetScriptPath(name)
	if !ok {
		return fmt.Errorf("preset %q not found", name)
	}

	dir := filepath.Dir(scriptPath)
	st, err := os.Stat(imgPath)
	if err != nil {
		return fmt.Errorf("source image not found: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(imgPath))
	validExt := false
	for _, v := range []string{".png", ".jpg", ".jpeg", ".gif"} {
		if ext == v {
			validExt = true
			break
		}
	}
	if !validExt {
		return fmt.Errorf("unsupported image format: %s (use .png, .jpg, or .gif)", ext)
	}

	// Destination now uses the source's extension
	dst := filepath.Join(dir, name+ext)

	return copyFile(imgPath, dst, st.Mode())
}

var HEADER_HEIGHT = 10

var ALLOWED_CATEGORIES = []string{"Animation", "Dashboard", "Ambiance", "System", "Productivity", "Misc"}

func parseMetadata(path string) PresetMetadata {
	f, err := os.Open(path)
	if err != nil {
		return PresetMetadata{}
	}
	defer f.Close()

	meta := PresetMetadata{Category: "Misc"} // Default to Misc immediately
	buf := make([]byte, 1024)
	n, _ := f.Read(buf)
	content := string(buf[:n])

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			continue
		}

		clean := strings.TrimPrefix(line, "#")
		parts := strings.SplitN(clean, ":", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			meta.Name = val
		case "description":
			meta.Description = val
		case "author":
			meta.Author = val
		case "category":
			isValid := false
			formattedVal := strings.Title(strings.ToLower(val)) // Normalize to Title Case
			for _, allowed := range ALLOWED_CATEGORIES {
				if formattedVal == allowed {
					meta.Category = allowed
					isValid = true
					break
				}
			}
			if !isValid {
				meta.Category = "Misc"
			}
		}
	}
	return meta
}

func setMasterPTYSizeFromMaxPaneLocked() {
	if currentPty == nil {
		return
	}

	mw, _, err := tmux.MaxPaneSize()
	if err != nil || mw <= 0 {
		cw, _, _ := tmux.CurrentClientSize()
		if cw <= 0 {
			mw = 80
		} else {
			mw = cw
		}
	}

	_ = pty.Setsize(currentPty, &pty.Winsize{
		Rows: uint16(HEADER_HEIGHT),
		Cols: uint16(mw),
	})

	if currentCmd != nil && currentCmd.Process != nil {
		_ = currentCmd.Process.Signal(syscall.SIGWINCH)
	}
}

func main() {
	setupSignalHandler()

	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "tuiwall panic: %v\n", r)
			_ = disable()
			os.Exit(1)
		}
	}()

	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "enable":
		mustInTmux()
		exe := tmux.MustExecutablePath()
		if err := enable(exe); err != nil {
			fatal(err)
		}
		fmt.Println("tuiwall enabled")
	case "disable":
		mustInTmux()
		if err := disable(); err != nil {
			fatal(err)
		}
		fmt.Println("tuiwall disabled")

	case "set":
		mustInTmux()

		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: tuiwall set <preset-name>"))
		}

		p := strings.TrimSpace(os.Args[2])
		if p == "" {
			fatal(fmt.Errorf("usage: tuiwall set <preset-name>"))
		}

		if err := tmux.SetGlobalOption("@tuiwall_preset", p); err != nil {
			fatal(err)
		}

		fmt.Println("tuiwall preset set to:", p)

	case "status":
		mustInTmux()
		enabled, _ := tmux.GetGlobalOption("@tuiwall_enabled")
		preset, _ := tmux.GetGlobalOption("@tuiwall_preset")
		if strings.TrimSpace(preset) == "" {
			preset = "clock"
		}

		heightStr, err := tmux.GetGlobalOption("@tuiwall_height")

		if strings.TrimSpace(heightStr) == "" || err != nil {
			heightStr = "10" // Default fallback
		}

		fmt.Printf("enabled=%s preset=%s\n", strings.TrimSpace(enabled), strings.TrimSpace(preset))

	case "render":
		if len(os.Args) < 3 {
			os.Exit(2)
		}
	case "_header":
		runHeader()
	case "_master":
		runMaster()
	case "_mirror":
		runMirror()
	case "list":
		presets, err := listPresets()
		if err != nil {
			fatal(err)
		}
		if len(presets) == 0 {
			fmt.Println("no presets found")
			return
		}

		termWidth := 80 // Default fallback
		if w, _, err := tmux.CurrentClientSize(); err == nil && w > 0 {
			termWidth = w
		}

		// Name (15), Category (12), Spacing/Pipes (5)
		nameWidth := 15
		catWidth := 12
		descLimit := termWidth - nameWidth - catWidth - 5
		if descLimit < 10 {
			descLimit = 10 // Sanity floor
		}

		var sb strings.Builder
		header := fmt.Sprintf("%-15s %-12s %s\n", "PRESET", "CATEGORY", "DESCRIPTION")
		sb.WriteString(header)
		sb.WriteString(strings.Repeat("-", len(header)) + "\n")

		for _, p := range presets {
			cat := p.Category
			if cat == "" {
				cat = "Misc"
			}

			// Truncate name if it somehow exceeds 15 chars
			displayName := p.Name
			if len(displayName) > nameWidth {
				displayName = displayName[:nameWidth-3] + "..."
			}

			// Smart truncate description based on terminal width
			desc := p.Description
			if len(desc) > descLimit {
				desc = desc[:descLimit-3] + "..."
			}

			sb.WriteString(fmt.Sprintf("%-15s %-12s %s\n", displayName, cat, desc))
		}

		// -F: quit if content fits on one screen
		// -R: allow ANSI colors (if you add them later)
		// -X: don't clear screen on exit
		cmd := exec.Command("less", "-F", "-R", "-X")
		cmd.Stdin = strings.NewReader(sb.String())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			// Fallback: just print if less fails
			fmt.Print(sb.String())
		}

	case "search":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: tuiwall search <keyword>"))
		}
		query := strings.ToLower(os.Args[2])
		presets, _ := listPresets()

		fmt.Printf("Results for '%s':\n", query)
		for _, p := range presets {
			if strings.Contains(strings.ToLower(p.Name), query) ||
				strings.Contains(strings.ToLower(p.Category), query) {
				fmt.Printf("%-15s [%s] %s\n", p.Name, p.Category, p.Description)
			}
		}
	case "record":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: tuiwall record <preset-name>"))
		}
		runRecord(os.Args[2])
	case "help":
		usage()
	case "version":
		fmt.Println("tuiwall v0.1.0")
		return

	case "reset":
		// disable
		mustInTmux()
		exe := tmux.MustExecutablePath()
		if err := disable(); err != nil {
			fatal(err)
		}

		// reenable
		if err := enable(exe); err != nil {
			fatal(err)
		}
		fmt.Println("tuiwall reset")

	/*
		case "height":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: tuiwall height <2-10>"))
		}

		newHeight, e := strconv.Atoi(strings.TrimSpace(os.Args[2]))

		if e != nil {
			fatal(e)
		}

		if newHeight < 2 || newHeight > 10 {
			fatal(fmt.Errorf("usage: tuiwall height <2-10>"))
		}

		if err := tmux.SetGlobalOption("@tuiwall_height", strings.TrimSpace(os.Args[2])); err != nil  {
			fatal(err)
		}

		fmt.Println("set height to", newHeight)
		HEADER_HEIGHT = newHeight

	enabled, _ := tmux.GetGlobalOption("@tuiwall_enabled")
		if strings.TrimSpace(enabled) == "1" {
			// disable
			mustInTmux()
			exe := tmux.MustExecutablePath()
			if err := disable(); err != nil {
				fatal(err)
			}

			// reenable
			if err:= enable(exe); err != nil {
				fatal(err)
			}
		}
	*/

	case "_reset_on_resize":
		// disable
		mustInTmux()
		exe := tmux.MustExecutablePath()
		if err := disable(); err != nil {
			fatal(err)
		}

		// reenable
		if err := enable(exe); err != nil {
			fatal(err)
		}

	case "preset":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: tuiwall preset <new|edit|path> ..."))
		}

		sub := strings.TrimSpace(os.Args[2])
		switch sub {
		case "new":
			if len(os.Args) < 4 {
				fatal(fmt.Errorf("usage: tuiwall preset new <name>"))
			}
			name := strings.TrimSpace(os.Args[3])
			if err := presetNewFromTemplate(name); err != nil {
				fatal(err)
			}
			fmt.Println("created preset:", name)

		case "edit":
			if len(os.Args) < 4 {
				fatal(fmt.Errorf("usage: tuiwall preset edit <name>"))
			}
			name := strings.TrimSpace(os.Args[3])
			p, err := presetScriptPathStrict(name)
			if err != nil {
				fatal(err)
			}
			if err := openInEditor(p); err != nil {
				fatal(err)
			}

		case "path":
			if len(os.Args) < 4 {
				fatal(fmt.Errorf("usage: tuiwall preset path <name>"))
			}
			name := strings.TrimSpace(os.Args[3])
			p, err := presetScriptPathStrict(name)
			if err != nil {
				fatal(err)
			}
			fmt.Println(p)

		case "info":
			if len(os.Args) < 4 {
				fatal(fmt.Errorf("usage: tuiwall preset info <preset-name>"))
			}
			name := os.Args[3]
			path, err := presetScriptPathStrict(name)
			if err != nil {
				fatal(err)
			}
			meta := parseMetadata(path)

			// Using a consistent width for labels (e.g., 14 characters)
			fmt.Printf("%-14s %s\n", "Name:", meta.Name)
			fmt.Printf("%-14s %s\n", "Author:", meta.Author)
			fmt.Printf("%-14s %s\n", "Category:", meta.Category)

			// Handle the description with wrapping
			fmt.Printf("%-14s ", "Description:")
			printWrapped(meta.Description, 15, 80) // Indent subsequent lines by 15
			fmt.Println()

		case "image":
			if len(os.Args) < 5 {
				fatal(fmt.Errorf("usage: tuiwall preset image <preset-name> <image-path>"))
			}
			name := strings.TrimSpace(os.Args[3])
			img := strings.TrimSpace(os.Args[4])
			if err := presetAttachImage(name, img); err != nil {
				fatal(err)
			}

		case "copy":
			if len(os.Args) < 5 {
				fatal(fmt.Errorf("usage: tuiwall preset copy <existing preses> <name>"))
			}

			copyName := strings.TrimSpace(os.Args[3])
			name := strings.TrimSpace(os.Args[4])

			if _, err := presetScriptPathStrict(copyName); err != nil {
				fatal(err)
			}

			if err := presetNewFromCopy(copyName, name); err != nil {
				fatal(err)
			}

			/*
				if err := o {
					fatal(err)
				}
			*/

		default:
			fatal(fmt.Errorf("usage: tuiwall preset <new|edit|path> ..."))
		}

	case "install":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: tuiwall install <path|git-url|name>"))
		}

		src := strings.TrimSpace(os.Args[2])

		fmt.Println("SECURITY WARNING: Presets contain Python scripts that run on your system.")
		fmt.Printf("   Only install presets from authors you trust.\n\n")

		if !confirmAction(fmt.Sprintf("Do you want to install '%s'?", src)) {
			fmt.Println("Installation aborted.")
			return
		}

		if err := presetInstall(src); err != nil {
			fatal(err)
		}
	case "uninstall":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: tuiwall uninstall <name>"))
		}
		name := strings.TrimSpace(os.Args[2])
		if name == "template" {
			fmt.Println("You cannot uninstall the template preset")
			return
		}
		if err := presetUninstall(name); err != nil {
			fatal(err)
		}
		fmt.Println("uninstalled preset:", name)

	case "upload":
		if len(os.Args) < 3 {
			fatal(fmt.Errorf("usage: tuiwall upload <preset-name> [git-remote-url|--zip]"))
		}
		name := strings.TrimSpace(os.Args[2])

		fmt.Println("PRIVACY WARNING: You are about to share your local preset code.")
		fmt.Println("   Ensure your script doesn't contain hardcoded API keys, tokens, or personal paths.")

		if !confirmAction(fmt.Sprintf("Are you sure you want to upload/zip '%s'?", name)) {
			fmt.Println("Upload aborted.")
			return
		}

		if len(os.Args) >= 4 && os.Args[3] == "--zip" {
			out, err := presetZip(name)
			if err != nil {
				fatal(err)
			}
			fmt.Println(out)
			return
		}

		if len(os.Args) >= 4 && looksLikeGitRemote(os.Args[3]) {
			remote := strings.TrimSpace(os.Args[3])
			if err := presetUploadToGit(name, remote); err != nil {
				fatal(err)
			}
			return
		}

		fmt.Printf("Preparing community repo contribution for '%s'...\n", name)
		if err := communityRepoPR(name); err != nil {
			fatal(err)
		}

	case "_update-master-size":
		_ = exec.Command("pkill", "-SIGWINCH", "-f", "tuiwall _master").Run()
		return
	case "_ensure-header":
		mustInTmux()

		if os.Getenv("TUIWALL_HEADER") == "1" {
			// fmt.Println("Header already exists - TUIWALL_HEADER")
			return
		}

		if p, err := tmux.CurrentPaneID(); err == nil {
			if tag, _ := tmux.GetPaneOption(strings.TrimSpace(p), "@tuiwall_header"); strings.TrimSpace(tag) == "1" {
				// fmt.Println("Header already exists - @tuiwall_header")
				return
			}
		}

		// only act if enabled
		enabled, _ := tmux.GetGlobalOption("@tuiwall_enabled")
		if strings.TrimSpace(enabled) != "1" {
			return
		}

		exe := tmux.MustExecutablePath()

		win, err := tmux.CurrentWindowID()
		if err != nil || strings.TrimSpace(win) == "" {
			return
		}
		win = strings.TrimSpace(win)

		// If a header creation is in-flight for this window, do nothing.
		lk := lockKeyForWindow(win)
		if v, _ := tmux.GetGlobalOption(lk); strings.TrimSpace(v) != "" {
			// optional: clear stale locks (> 3s)
			if ms, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
				if time.Now().UnixMilli()-ms < 3000 {
					return
				}
			}
			_ = tmux.UnsetGlobalOption(lk)
		}

		// If the ACTIVE pane is already a tuiwall header, do nothing.
		if p, err := tmux.CurrentPaneID(); err == nil && strings.TrimSpace(p) != "" {
			if tag, _ := tmux.GetPaneOption(strings.TrimSpace(p), "@tuiwall_header"); strings.TrimSpace(tag) == "1" {
				return
			}
		}

		ensureHeaderForWindow(exe, win)

	default:
		usage()
	}
}

func usage() {
	fmt.Println(`tuiwall (v0.1.0) - tmux "terminal wallpaper" header

Usage:
  tuiwall enable
  tuiwall disable
  tuiwall reset
  tuiwall set <preset>
  tuiwall list
  tuiwall search <keyword>
  tuiwall status
  tuiwall preset <new|edit|path|info> <preset>
  tuiwall record <preset>
  tuiwall preset image <preset> <image path>
  tuiwall preset copy <existing preset> <preset> 
  tuiwall install <repo url>
  tuiwall uninstall <preset>
  tuiwall upload <preset> <repo url|NULL>
`)
}

func activePaneID() string {
	out, err := exec.Command("tmux", "display-message", "-p", "#{pane_id}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func InTmux() bool {
	return os.Getenv("TMUX") != ""
}

func EnsureTmuxOrReexec(args []string) {
	if InTmux() {
		return
	}
	if os.Getenv("TUIWALL_REEXEC") == "1" {
		return
	}

	exe := tmux.MustExecutablePath()

	// Safely build: TUIWALL_REEXEC=1 '<exe>' '<arg1>' '<arg2>' ...
	parts := []string{"TUIWALL_REEXEC=1", shellEscape(exe)}
	for _, a := range args {
		parts = append(parts, shellEscape(a))
	}
	inner := strings.Join(parts, " ")

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(
		"tmux",
		"new-session",
		"-Ad",
		"-s", "tuiwall",
		shell,
	)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal(err)
	}

	// Run the actual command inside the session
	cmd = exec.Command("tmux", "run-shell", "-t", "tuiwall", inner)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal(err)
	}

	// Now attach user
	cmd = exec.Command("tmux", "attach-session", "-t", "tuiwall")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal(err)
	}

	os.Exit(0)
}

func mustInTmux() {
	if os.Getenv("TMUX") == "" {
		// EnsureTmuxOrReexec(os.Args[1:])

		fmt.Fprintln(os.Stderr, "tuiwall: not inside tmux. Run: tmux")
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "tuiwall:", err)
	os.Exit(1)
}

func enable(exePath string) error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux not found in PATH: please install tmux to use tuiwall")
	}

	if _, err := exec.LookPath("gh"); err != nil {
		fmt.Println("Warning: GitHub CLI (gh) not found. Community features will be disabled.")
	}

	// Check for Python (trying python3 then python)
	pyCmd := getPythonCmd()
	if pyCmd == "" {
		return fmt.Errorf("python not found: please install Python 3 to run tuiwall presets")
	}

	enabled, _ := tmux.GetGlobalOption("@tuiwall_enabled")
	hasSession := exec.Command("tmux", "has-session", "-t", "tuiwall-master").Run() == nil

	if strings.TrimSpace(enabled) == "1" && hasSession {
		ensureHeadersAllWindows(exePath)
		return nil
	}

	_ = tmux.SetGlobalOption("@tuiwall_enabled", "1")
	_ = tmux.SetGlobalOption("@tuiwall_python", pyCmd)

	mw, _, err := tmux.MaxPaneSize()
	if err != nil || mw <= 0 {
		// Fallback: current client size
		cw, _, _ := tmux.CurrentClientSize()
		if cw > 0 {
			mw = cw
		} else {
			mw = 80
		}
	}

	_ = exec.Command("tmux", "kill-session", "-t", "tuiwall-master").Run()

	// Force TERM=xterm-256color so the Python presets know they have color support
	masterCmd := fmt.Sprintf("TERM=xterm-256color %s _master", shellEscape(exePath))

	err = exec.Command("tmux", "new-session", "-d",
		"-x", fmt.Sprint(mw),
		"-y", fmt.Sprint(HEADER_HEIGHT),
		"-s", "tuiwall-master",
		masterCmd,
	).Run()

	if err != nil {
		return fmt.Errorf("failed to start tuiwall-master session: %w", err)
	}

	// Wait for the master socket to be ready before continuing
	_ = exec.Command("tmux", "wait-for", "tuiwall_ready").Run()

	installHooks(exePath)
	ensureHeadersAllWindows(exePath)

	return nil
}

func disable() error {
	removeHooks()

	// Explicitly kill the master session to trigger the new cleanup logic
	_ = exec.Command("tmux", "kill-session", "-t", "tuiwall-master").Run()

	_ = exec.Command("tmux", "set-window-option", "window-style", "default").Run()
	_ = exec.Command("tmux", "set-window-option", "window-active-style", "default").Run()

	wins, err := tmux.ListSessionWindowIDs()
	if err == nil {
		for _, w := range wins {
			// Find and kill the header pane
			if paneID, ok := tmux.FindHeaderPaneInWindow(w); ok {
				// Before killing, try to clear the pane's styling
				_ = exec.Command("tmux", "set-pane-option", "-t", paneID, "select-pane-color", "default").Run()
				_ = tmux.KillPaneAsync(paneID)
			}

			// Cleanup internal tmux variables
			key := headerKeyForWindow(w)
			_ = tmux.UnsetGlobalOption(key)
			_ = tmux.UnsetGlobalOption(lockKeyForWindow(w))
		}
	}

	_ = tmux.UnsetGlobalOption("@tuiwall_enabled")
	_ = tmux.UnsetGlobalOption("@tuiwall_mode")

	_ = exec.Command("tmux", "refresh-client", "-S").Run()

	return nil
}

func getFIFOPath() string {
	return filepath.Join(os.TempDir(), "tuiwall.fifo")
}

func ensureFIFOExists() error {
	path := getFIFOPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create a named pipe (Unix only)
		return exec.Command("mkfifo", path).Run()
	}
	return nil
}

func resolvePreset() string {
	preset := "template"
	if os.Getenv("TMUX") != "" {
		if p, err := tmux.GetGlobalOption("@tuiwall_preset"); err == nil {
			p = strings.TrimSpace(p)
			if p != "" {
				preset = p
			}
		}
	}
	return preset
}

func runHeaderWithWriter(out io.Writer) {
	const (
		DEBOUNCE         = 250 * time.Millisecond
		RETRY_BACKOFF    = 1 * time.Second
		TICK             = 250 * time.Millisecond
		FAST_EXIT_CUTOFF = 500 * time.Millisecond
	)

	paneID := strings.TrimSpace(os.Getenv("TMUX_PANE"))

	// Helper to send "clear" commands to the socket
	sendClear := func() {
		_, _ = out.Write([]byte("\x1b[2J\x1b[H"))
	}

	/*
		resolvePreset := func() string {
			preset := "template"
			if os.Getenv("TMUX") != "" {
				if p, err := tmux.GetGlobalOption("@tuiwall_preset"); err == nil {
					p = strings.TrimSpace(p)
					if p != "" {
						preset = p
					}
				}
			}
			return preset
		}*/

	var (
		curPreset      string
		pendingPreset  string
		pendingSince   time.Time
		cmd            *exec.Cmd
		nextStartAfter time.Time
	)

	killChild := func() {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
		cmd = nil
	}

	startPreset := func(preset string) {
		killChild()
		sendClear()

		script, ok := presetScriptPath(preset)
		if !ok {
			_, _ = fmt.Fprintf(out, "tuiwall: preset %q not found\n", preset)
			curPreset = preset
			return
		}

		c := exec.Command(getPythonCmd(), script)
		c.Stdin = os.Stdin
		c.Stdout = out
		c.Stderr = os.Stderr

		if err := c.Start(); err != nil {
			_, _ = fmt.Fprintf(out, "tuiwall: python start failed: %v\n", err)
			nextStartAfter = time.Now().Add(RETRY_BACKOFF)
			return
		}

		cmd = c
		curPreset = preset

		if paneID != "" {
			_ = tmux.ResizePaneHeight(paneID, HEADER_HEIGHT)
		}

		startedAt := time.Now()
		go func(local *exec.Cmd, started time.Time) {
			_ = local.Wait()
			if cmd == local {
				cmd = nil
			}
			if time.Since(started) < FAST_EXIT_CUTOFF {
				nextStartAfter = time.Now().Add(RETRY_BACKOFF)
			}
		}(c, startedAt)
	}

	// Initial start
	curPreset = resolvePreset()
	startPreset(curPreset)

	ticker := time.NewTicker(TICK)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		desired := resolvePreset()

		if desired != curPreset {
			if desired != pendingPreset {
				pendingPreset = desired
				pendingSince = now
			}

			if now.Sub(pendingSince) >= DEBOUNCE && now.After(nextStartAfter) {
				startPreset(pendingPreset)
				pendingPreset = ""
			}
			continue
		}

		if cmd == nil && curPreset != "" && now.After(nextStartAfter) {
			if _, ok := presetScriptPath(curPreset); ok {
				startPreset(curPreset)
			}
		}
	}
}

var currentCmd *exec.Cmd
var cmdMu sync.Mutex
var currentPty *os.File

func startPresetWithPTY(preset string, clients *[]net.Conn, mu *sync.Mutex) {
	mu.Lock()
	clearCmd := []byte("\x1b[2J\x1b[H")
	for _, conn := range *clients {
		_, _ = conn.Write(clearCmd)
	}
	mu.Unlock()

	script, _ := presetScriptPath(preset)
	c := exec.Command(getPythonCmd(), "-u", script)

	f, err := pty.Start(c)
	if err != nil {
		return
	}
	defer f.Close()

	fd := int(f.Fd())
	if term, err := unix.IoctlGetTermios(fd, getTermiosReq); err == nil {
		term.Oflag |= unix.OPOST // Enable output processing
		term.Oflag |= unix.ONLCR // Automatically map NL (\n) to CR-NL (\r\n)
		_ = unix.IoctlSetTermios(fd, setTermiosReq, term)
	}

	mw, _, err := tmux.MaxPaneSize()
	if err != nil || mw <= 0 {
		cw, _, _ := tmux.CurrentClientSize()
		if cw <= 0 {
			mw = 80
		} else {
			mw = cw
		}
	}

	_ = pty.Setsize(f, &pty.Winsize{
		Rows: uint16(HEADER_HEIGHT),
		Cols: uint16(mw),
	})

	cmdMu.Lock()
	currentCmd = c
	currentPty = f
	cmdMu.Unlock()

	buf := make([]byte, 32768)
	for {
		n, err := f.Read(buf)
		if err != nil {
			break
		}
		payload := buf[:n]

		mu.Lock()
		activeClients := make([]net.Conn, 0, len(*clients))
		for _, conn := range *clients {
			if _, err := conn.Write(payload); err == nil {
				activeClients = append(activeClients, conn)
			} else {
				conn.Close()
			}
		}
		*clients = activeClients
		mu.Unlock()
	}
}

func getSocketPath() string {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, fmt.Sprintf("tuiwall-%d.sock", os.Getuid()))
}

func runMaster() {
	socketPath := getSocketPath()
	_ = os.Remove(socketPath)

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		fatal(err)
	}

	// Create a channel to listen for termination signals (Ctrl+C, tmux kill, etc.)
	sigCleanup := make(chan os.Signal, 1)
	signal.Notify(sigCleanup, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCleanup
		fmt.Println("\nShutting down tuiwall-master...")

		cmdMu.Lock()
		if currentCmd != nil && currentCmd.Process != nil {
			_ = currentCmd.Process.Kill()
		}
		cmdMu.Unlock()

		_ = os.Remove(socketPath)

		_ = l.Close()
		os.Exit(0)
	}()

	_ = exec.Command("tmux", "wait-for", "-S", "tuiwall_ready").Run()

	var clients []net.Conn
	var mu sync.Mutex

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)

	// Listener for new mirror connections
	go func() {
		for {
			conn, err := l.Accept()
			if err == nil {
				mu.Lock()
				clients = append(clients, conn)
				mu.Unlock()
				sigChan <- syscall.SIGWINCH
			}
		}
	}()

	// Orchestrator for Resizing and Preset Switching
	go func() {
		lastPreset := resolvePreset()
		for {
			select {
			case <-sigChan:
				cmdMu.Lock()
				if currentPty != nil {
					mw, _, err := tmux.MaxPaneSize()
					if err != nil || mw <= 0 {
						cw, _, _ := tmux.CurrentClientSize()
						mw = cw
					}
					if mw <= 0 {
						mw = 80
					}

					_ = pty.Setsize(currentPty, &pty.Winsize{
						Rows: uint16(HEADER_HEIGHT),
						Cols: uint16(mw),
					})

					if currentCmd != nil && currentCmd.Process != nil {
						_ = currentCmd.Process.Signal(syscall.SIGWINCH)
					}
				}
				cmdMu.Unlock()

			case <-time.After(500 * time.Millisecond):
				current := resolvePreset()
				cmdMu.Lock()
				if current != lastPreset && currentCmd != nil && currentCmd.Process != nil {
					_ = currentCmd.Process.Kill()
					lastPreset = current
				}
				cmdMu.Unlock()
			}
		}
	}()

	// Main execution loop
	for {
		preset := resolvePreset()
		startPresetWithPTY(preset, &clients, &mu)
		time.Sleep(100 * time.Millisecond)
	}
}

func sanitizeKey(k string) string {
	// Turn status-format[0] into status_format_0
	k = strings.ReplaceAll(k, "-", "_")
	k = strings.ReplaceAll(k, "[", "_")
	k = strings.ReplaceAll(k, "]", "")
	return k
}

func shellEscape(path string) string {
	// For safety inside tmux #() which uses /bin/sh -c
	// Wrap in single quotes and escape any single quotes.
	if !strings.Contains(path, "'") {
		return "'" + path + "'"
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

// Probably not a very good solution
func blankScreen() {
	// Clear, home, hide cursor, leave alt screen
	fmt.Print("\x1b[2J\x1b[H\x1b[?25l\x1b[?1049l")
}

func runHeader() {
	const (
		DEBOUNCE         = 350 * time.Millisecond
		RETRY_BACKOFF    = 1 * time.Second
		TICK             = 250 * time.Millisecond
		FAST_EXIT_CUTOFF = 500 * time.Millisecond
	)

	paneID := strings.TrimSpace(os.Getenv("TMUX_PANE"))

	forceRedraw := func() {
		// Clear entire pane and redraw background
		// 	 fmt.Print("\x1b[2J\x1b[H")
	}

	blankScreen := func() {
		if paneID != "" {
			_ = tmux.ResizePaneHeight(paneID, HEADER_HEIGHT)
		}
	}

	showCursorRestore := func() {
		// Show cursor; leave alt screen
		fmt.Print("\x1b[?25h\x1b[?1049l")
	}

	resolvePreset := func() string {
		preset := "template"
		if os.Getenv("TMUX") != "" {
			if p, err := tmux.GetGlobalOption("@tuiwall_preset"); err == nil {
				p = strings.TrimSpace(p)
				if p != "" {
					preset = p
				}
			}
		}
		return preset
	}

	var (
		curPreset     string
		pendingPreset string
		pendingSince  time.Time
		switchBlanked bool

		cmd            *exec.Cmd
		nextStartAfter time.Time
	)

	// Kill dangling Python processes
	killChild := func() {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
		cmd = nil
	}

	startPreset := func(preset string) {
		// Show blank once right as we commit to starting
		blankScreen()
		switchBlanked = false

		killChild()

		script, ok := presetScriptPath(preset)
		if !ok {
			blankScreen()
			fmt.Printf("tuiwall: preset %q not found\n", preset)
			curPreset = preset
			return
		}

		c := exec.Command(getPythonCmd(), script)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Start(); err != nil {
			blankScreen()
			fmt.Printf("tuiwall: python start failed: %v\n", err)
			nextStartAfter = time.Now().Add(RETRY_BACKOFF)
			return
		}

		cmd = c
		curPreset = preset

		// Make sure height stays pinned after spawn
		if paneID != "" {
			_ = tmux.ResizePaneHeight(paneID, HEADER_HEIGHT)
		}

		// Watch for exit; if it dies quickly, back off to avoid rapid respawns.
		startedAt := time.Now()
		go func(local *exec.Cmd, started time.Time) {
			_ = local.Wait()

			// Only clear if we're still the active cmd
			if cmd == local {
				cmd = nil
			}

			if time.Since(started) < FAST_EXIT_CUTOFF {
				// nextStartAfter = time.Now().Add(RETRY_BACKOFl)
			}
		}(c, startedAt)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGWINCH)

	go func() {
		for s := range sig {
			switch s {
			case syscall.SIGWINCH:
				if paneID != "" {
					_ = tmux.ResizePaneHeight(paneID, HEADER_HEIGHT)
				}

				forceRedraw()
			default:
				// Exit cleanup
				killChild()
				showCursorRestore()
				os.Exit(0)
			}
		}
	}()

	// Start initial preset immediately
	curPreset = resolvePreset()
	startPreset(curPreset)

	ticker := time.NewTicker(TICK)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		desired := resolvePreset()

		// preset switch requested
		if desired != curPreset {
			if desired != pendingPreset {
				pendingPreset = desired
				pendingSince = now
				switchBlanked = false
			}

			if !switchBlanked {
				blankScreen()
				switchBlanked = true
			}

			if now.Sub(pendingSince) >= DEBOUNCE && now.After(nextStartAfter) {
				startPreset(pendingPreset)
				pendingPreset = ""
				switchBlanked = false
			}
			continue
		}

		// desired == curPreset
		pendingPreset = ""
		switchBlanked = false

		if cmd == nil && curPreset != "" && now.After(nextStartAfter) {
			if _, ok := presetScriptPath(curPreset); ok {
				startPreset(curPreset)
			}
		}
	}
}

func renderFallbackHeader(preset string) {
	var H = HEADER_HEIGHT
	prev := make([]string, H)

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		// now := time.Now()
		lines := []string{}

		switch preset {
		case "clock":
		case "stats":
		default:
		}

		for len(lines) < H {
			lines = append(lines, "")
		}
		if len(lines) > H {
			lines = lines[:H]
		}

		for i := 0; i < H; i++ {
			if lines[i] == prev[i] {
				continue
			}
			prev[i] = lines[i]
			fmt.Printf("\x1b[%d;1H", i+1)
			fmt.Print(lines[i])
			fmt.Print("\x1b[K")
		}
	}
}

func listPresets() ([]PresetMetadata, error) {
	seen := map[string]bool{}
	var out []PresetMetadata

	for _, dir := range presetDirs() {
		scanPresetDir(dir, seen, &out)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func scanPresetDir(dir string, seen map[string]bool, out *[]PresetMetadata) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		var scriptPath string
		name := e.Name()

		if e.IsDir() {
			scriptPath = filepath.Join(dir, name, name+".py")
		} else if strings.HasSuffix(name, ".py") {
			scriptPath = filepath.Join(dir, name)
			name = strings.TrimSuffix(name, ".py")
		}

		if scriptPath != "" && validPresetName(name) && name != "template" && !seen[name] {
			if st, err := os.Stat(scriptPath); err == nil && !st.IsDir() {
				meta := parseMetadata(scriptPath)
				if meta.Name == "" {
					meta.Name = name
				}
				*out = append(*out, meta)
				seen[name] = true
			}
		}
	}
}

func presetScriptPath(preset string) (string, bool) {
	for _, dir := range presetDirs() {
		try := filepath.Join(dir, preset+"/"+preset+".py")
		if st, err := os.Stat(try); err == nil && !st.IsDir() {
			return try, true
		}

		try2 := filepath.Join(dir, preset+".py")
		if st, err := os.Stat(try2); err == nil && !st.IsDir() {
			return try2, true
		}
	}
	return "", false
}

func presetDirs() []string {
	dirs := []string{}

	if d := strings.TrimSpace(os.Getenv("TUIWALL_PRESET_DIR")); d != "" {
		dirs = append(dirs, d)
	}

	if cfg, err := os.UserConfigDir(); err == nil && cfg != "" {
		dirs = append(dirs, filepath.Join(cfg, "tuiwall", "presets"))
	}

	dirs = append(dirs,
		"/usr/local/share/tuiwall/presets",    // Manual Linux / Intel Mac
		"/usr/share/tuiwall/presets",          // Linux Distro Packages
		"/opt/homebrew/share/tuiwall/presets", // Apple Silicon Homebrew
	)

	// Useful for "portable" installs where the user just downloads a folder.
	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)
		dirs = append(dirs, filepath.Join(base, "presets"))
	}

	return dirs
}

func headerKeyForWindow(windowID string) string {
	return "@tuiwall_header_" + windowID
}

// Ensure header exists for one window
func ensureHeaderForWindow(exePath, windowID string) {
	if existingID, ok := tmux.FindHeaderPaneInWindow(windowID); ok {
		_ = tmux.ResizePaneHeight(existingID, HEADER_HEIGHT)
		return
	}

	lockKey := fmt.Sprintf("@tuiwall_lock_%s", windowID)
	lock, _ := tmux.GetGlobalOption(lockKey)
	if strings.TrimSpace(lock) == "1" {
		return
	}
	_ = tmux.SetGlobalOption(lockKey, "1")
	defer tmux.UnsetGlobalOption(lockKey)

	mirrorCmd := shellEscape(exePath) + " _mirror"

	// Command: tmux split-window -t <win> -v -b -l <height> -P -F "#{pane_id}" <cmd>
	cmd := exec.Command("tmux", "split-window",
		"-t", windowID,
		"-d",       // Detached
		"-v", "-b", // Top split
		"-l", strconv.Itoa(HEADER_HEIGHT),
		"-P", "-F", "#{pane_id}",
		mirrorCmd,
	)

	out, err := cmd.Output()
	if err != nil {
		return
	}

	newPaneID := strings.TrimSpace(string(out))
	_ = tmux.SetPaneOption(newPaneID, "@tuiwall_header", "1")
}

// Ensure headers exist for all windows across server
func ensureHeadersAllWindows(exePath string) {
	wins, err := tmux.ListSessionWindowIDs()
	if err != nil {
		return
	}
	for _, w := range wins {
		ensureHeaderForWindow(exePath, w)
	}
}

func respawnAllHeaders(exePath string) {

	// Command to run inside each header pane
	cmd := "TUIWALL_HEADER=1 " + shellEscape(exePath) + " _header"

	wins, err := tmux.ListSessionWindowIDs()
	if err != nil {
		return
	}

	for _, w := range wins {
		// Find the existing header pane in this window
		paneID, ok := tmux.FindHeaderPaneInWindow(w)
		if !ok {
			continue
		}

		// Ensure it still exists
		if !tmux.PaneExists(paneID) {
			continue
		}

		_ = exec.Command(
			"tmux",
			"respawn-pane",
			"-k",
			"-t", paneID,
			cmd,
		).Run()

		_ = tmux.ResizePaneHeight(paneID, HEADER_HEIGHT)
	}
}

// Ensure all tmux windows have hooks
func installHooks(exePath string) {
	// Store exe path globally so hooks keep working even if PATH differs
	_ = tmux.SetGlobalOption("@tuiwall_exe", exePath)

	_ = tmux.SetGlobalOption("@tuiwall_height", strconv.Itoa(HEADER_HEIGHT))

	inner := "TUIWALL_HOOK=1 " + shellEscape(exePath) + " _ensure-header"
	hookCmd := "run-shell -b " + shellEscape(inner)

	_ = tmux.SetHookGlobal("after-new-window", hookCmd)
	_ = tmux.SetHookGlobal("after-split-window", hookCmd)

	_ = tmux.SetHookGlobal("client-attached", hookCmd)

	resizeCmd := "run-shell -b " + shellEscape("tmux resize-window -t tuiwall-master:0 -x #{client_width} -y #{client_height} >/dev/null 2>&1 || true")
	_ = tmux.SetHookGlobal("client-resized", resizeCmd)
}

// Cleanup Hooks
func removeHooks() {
	_ = tmux.UnsetHookGlobal("after-new-window")
	_ = tmux.UnsetHookGlobal("after-split-window")
	_ = tmux.UnsetHookGlobal("client-attached")
	_ = tmux.UnsetHookGlobal("client-resized")
	_ = tmux.UnsetHookGlobal("tuiwall-height")
}

func lockKeyForWindow(windowID string) string {
	return "@tuiwall_lock_" + windowID
}

func presetHomeDir() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(cfg) == "" {
		return "", fmt.Errorf("could not determine user config dir")
	}
	return filepath.Join(cfg, "tuiwall", "presets"), nil
}

func validPresetName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func presetScriptPathStrict(preset string) (string, error) {
	p, ok := presetScriptPath(preset)
	if !ok || strings.TrimSpace(p) == "" {
		return "", fmt.Errorf("preset %q not found", preset)
	}
	return p, nil
}

func presetNewFromTemplate(name string) error {
	name = strings.TrimSpace(name)
	if !validPresetName(name) {
		return fmt.Errorf("invalid preset name %q (use letters/numbers/_/-)", name)
	}
	if name == "template" {
		return fmt.Errorf("preset name %q is reserved", name)
	}

	// Find the existing "template" preset in any of your presetDirs() locations.
	templateScript, err := presetScriptPathStrict("template")
	if err != nil {
		return fmt.Errorf("missing template preset: %w\n Run: tuiwall install template", err)
	}
	templateDir := filepath.Dir(templateScript)

	// Destination is always user config dir
	home, err := presetHomeDir()
	if err != nil {
		return err
	}
	dstDir := filepath.Join(home, name)

	if _, err := os.Stat(dstDir); err == nil {
		return fmt.Errorf("preset %q already exists at %s", name, dstDir)
	}

	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}

	// Copy template/ -> ~/.config/tuiwall/presets/<name>/
	if err := copyDir(templateDir, dstDir); err != nil {
		return err
	}

	// Rename template.py -> <name>.py
	oldPy := filepath.Join(dstDir, "template.py")
	newPy := filepath.Join(dstDir, name+".py")

	if _, err := os.Stat(oldPy); err == nil {
		if err := os.Rename(oldPy, newPy); err != nil {
			return err
		}
	} else {
		if _, err2 := os.Stat(newPy); err2 != nil {
			return fmt.Errorf("template preset must include template.py")
		}
	}

	// Ensure the new script is actually runnable by the OS
	if err := ensureExecutable(newPy); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	_ = replaceInFile(newPy, map[string]string{
		"template": name,
	})

	return nil
}

func presetNewFromCopy(copyName string, name string) error {
	copyName = strings.TrimSpace(copyName)
	name = strings.TrimSpace(name)
	if !validPresetName(name) {
		return fmt.Errorf("invalid preset name %q (use letters/numbers/_/-)", name)
	}
	if name == copyName {
		return fmt.Errorf("preset name %q is reserved", name)
	}

	// Find the existing "template" preset in any of your presetDirs() locations.
	copyScript, err := presetScriptPathStrict(copyName)
	if err != nil {
		return fmt.Errorf("missing template preset: %w", err)
	}
	copyPresetDir := filepath.Dir(copyScript)

	// Destination is always user config dir so users have a single place to edit/share.
	home, err := presetHomeDir()
	if err != nil {
		return err
	}
	dstDir := filepath.Join(home, name)

	if _, err := os.Stat(dstDir); err == nil {
		return fmt.Errorf("preset %q already exists at %s", name, dstDir)
	}

	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}

	// Copy template/ -> ~/.config/tuiwall/presets/<name>/
	if err := copyDir(copyPresetDir, dstDir); err != nil {
		return err
	}

	// Rename template.py -> <name>.py
	oldPy := filepath.Join(dstDir, copyName+".py")
	newPy := filepath.Join(dstDir, name+".py")

	if _, err := os.Stat(oldPy); err == nil {
		if err := os.Rename(oldPy, newPy); err != nil {
			return err
		}
	} else {
		// Enforce convention: must end up with <name>.py
		if _, err2 := os.Stat(newPy); err2 != nil {
			return fmt.Errorf("template preset must include template.py (so it can be renamed)")
		}
	}

	_ = replaceInFile(newPy, map[string]string{
		copyName: name,
	})

	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, info.Mode())
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func replaceInFile(path string, replacements map[string]string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(b)
	for from, to := range replacements {
		s = strings.ReplaceAll(s, from, to)
	}
	return os.WriteFile(path, []byte(s), 0o644)
}

func openInEditor(path string) error {
	ed := strings.TrimSpace(os.Getenv("EDITOR"))
	if ed == "" {
		ed = "vi"
	}
	cmd := exec.Command(ed, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func presetUninstall(name string) error {
	name = strings.TrimSpace(name)
	if !validPresetName(name) {
		return fmt.Errorf("invalid preset name %q", name)
	}
	if name == "template" {
		return fmt.Errorf("refusing to uninstall %q (template is reserved)", name)
	}

	home, err := presetHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, name)

	// Refuse if it doesn't exist
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("preset %q is not installed in %s", name, home)
		}
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("expected %s to be a directory", dir)
	}

	cleanHome := filepath.Clean(home) + string(os.PathSeparator)
	cleanDir := filepath.Clean(dir) + string(os.PathSeparator)
	if !strings.HasPrefix(cleanDir, cleanHome) {
		return fmt.Errorf("refusing to remove path outside preset home")
	}

	return os.RemoveAll(dir)
}

func presetInstall(src string) error {
	src = strings.TrimSpace(src)
	if src == "template" {
		return installEmbeddedTemplate()
	}

	if src == "" {
		return fmt.Errorf("usage: tuiwall install <name|path|git-url>")
	}

	isShortName := !strings.Contains(src, "/") && !strings.Contains(src, ".") && !strings.Contains(src, "@")

	if isShortName {
		fmt.Printf("--> Fetching '%s' from community repository...\n", src)
		return presetInstallFromGit("https://github.com/Mug-Costanza/tuiwall-presets.git", src)
	}

	if looksLikeGitRemote(src) {
		return presetInstallFromGit(src, "")
	}

	return presetInstallFromPath(src)
}

func looksLikeGitRemote(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "git@") ||
		strings.HasSuffix(s, ".git")
}

func presetInstallFromPath(path string) error {
	path = filepath.Clean(path)

	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("preset source must be a directory: %s", path)
	}

	name := filepath.Base(path)
	if !validPresetName(name) {
		return fmt.Errorf("invalid preset name %q", name)
	}
	if name == "template" {
		return fmt.Errorf("preset name %q is reserved", name)
	}

	// Require <name>.py in that directory
	script := filepath.Join(path, name+".py")
	if st, err := os.Stat(script); err != nil || st.IsDir() {
		return fmt.Errorf("preset directory must contain %s", name+".py")
	}

	return installDirToUserHome(name, path)
}

func presetInstallFromGit(remote string, requestedFolder string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH (required for installing from URLs)")
	}

	tmp, err := os.MkdirTemp("", "tuiwall-install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	fmt.Println("--> Connecting to remote...")
	if requestedFolder != "" {
		cloneArgs := []string{"clone", "--depth", "1", "--filter=blob:none", "--sparse", remote, tmp}
		if err := exec.Command("git", cloneArgs...).Run(); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
		target := filepath.Join("presets", requestedFolder)
		_ = exec.Command("git", "-C", tmp, "sparse-checkout", "set", target).Run()
	} else {
		cloneArgs := []string{"clone", "--depth", "1", remote, tmp}
		if err := exec.Command("git", cloneArgs...).Run(); err != nil {
			return fmt.Errorf("git clone failed: %w", err)
		}
	}

	presetDir, presetName, err := findPresetDirInRepo(tmp, remote)
	if err != nil {
		return err
	}

	return installDirToUserHome(presetName, presetDir)
}

func installDirToUserHome(name, srcDir string) error {
	home, err := presetHomeDir()
	if err != nil {
		return err
	}
	dstDir := filepath.Join(home, name)

	if _, err := os.Stat(dstDir); err == nil {
		return fmt.Errorf("preset %q already installed at %s", name, dstDir)
	}

	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	if err := copyDir(srcDir, dstDir); err != nil {
		return err
	}

	fmt.Println("installed preset:", name)
	fmt.Println("to:", dstDir)
	return nil
}

func findPresetDirInRepo(repoPath string, remote string) (dir string, name string, err error) {
	presetsDir := filepath.Join(repoPath, "presets")
	if st, err := os.Stat(presetsDir); err == nil && st.IsDir() {
		entries, _ := os.ReadDir(presetsDir)
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			n := e.Name()
			if !validPresetName(n) || n == "template" {
				continue
			}

			d := filepath.Join(presetsDir, n)
			s := filepath.Join(d, n+".py")
			if st, e2 := os.Stat(s); e2 == nil && !st.IsDir() {
				return d, n, nil
			}
		}
	}

	rootEntries, _ := os.ReadDir(repoPath)
	var dirCandidates []struct {
		path string
		name string
	}

	for _, e := range rootEntries {
		if e.IsDir() {
			n := e.Name()
			// Ignore hidden dirs like .git and reserved names
			if strings.HasPrefix(n, ".") || !validPresetName(n) || n == "template" {
				continue
			}
			scriptCheck := filepath.Join(repoPath, n, n+".py")
			if st, err := os.Stat(scriptCheck); err == nil && !st.IsDir() {
				dirCandidates = append(dirCandidates, struct{ path, name string }{
					path: filepath.Join(repoPath, n),
					name: n,
				})
			}
		}
	}

	if len(dirCandidates) == 1 {
		return dirCandidates[0].path, dirCandidates[0].name, nil
	}

	derived := derivePresetNameFromRemote(remote)
	if derived != "" && validPresetName(derived) && derived != "template" {
		d := filepath.Join(repoPath, derived)
		s := filepath.Join(d, derived+".py")
		if st, e := os.Stat(s); e == nil && !st.IsDir() {
			return d, derived, nil
		}
		s2 := filepath.Join(repoPath, derived+".py")
		if st, e := os.Stat(s2); e == nil && !st.IsDir() {
			return repoPath, derived, nil
		}
	}

	var pyFiles []string
	for _, e := range rootEntries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".py") {
			pyFiles = append(pyFiles, e.Name())
		}
	}
	if len(pyFiles) == 1 {
		base := strings.TrimSuffix(pyFiles[0], ".py")
		if validPresetName(base) && base != "template" {
			return repoPath, base, nil
		}
	}

	return "", "", fmt.Errorf("could not find an installable preset in repo. expected one of:\n- <name>/<name>.py\n- <name>.py at repo root\n- presets/<name>/<name>.py")
}

func derivePresetNameFromRemote(remote string) string {
	r := strings.TrimSpace(remote)

	// Strip trailing .git
	r = strings.TrimSuffix(r, ".git")

	// Take last path segment after / or :
	seg := r
	if i := strings.LastIndex(seg, "/"); i >= 0 {
		seg = seg[i+1:]
	}
	if i := strings.LastIndex(seg, ":"); i >= 0 {
		seg = seg[i+1:]
	}
	seg = strings.TrimSpace(seg)
	if seg == "" {
		return ""
	}

	// If repo is named tuiwall-preset-XYZ, install name XYZ
	if strings.HasPrefix(seg, "tuiwall-preset-") {
		seg = strings.TrimPrefix(seg, "tuiwall-preset-")
	}
	seg = strings.Trim(seg, " \t\n\r")
	return seg
}

func presetInstalledDirStrict(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !validPresetName(name) {
		return "", fmt.Errorf("invalid preset name %q", name)
	}
	home, err := presetHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, name)
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("preset %q is not installed in %s", name, home)
		}
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("expected preset dir %s to be a directory", dir)
	}
	script := filepath.Join(dir, name+".py")
	if st2, err := os.Stat(script); err != nil || st2.IsDir() {
		return "", fmt.Errorf("installed preset must contain %s", name+".py")
	}
	return dir, nil
}

func presetUploadToGit(name, remote string) error {
	name = strings.TrimSpace(name)
	remote = strings.TrimSpace(remote)

	if name == "template" {
		return fmt.Errorf("refusing to upload %q (template is reserved)", name)
	}
	if remote == "" {
		return fmt.Errorf("missing remote url")
	}

	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH (required for upload)")
	}

	scriptPath, ok := presetScriptPath(name)
	if !ok {
		return fmt.Errorf("preset %q not found in any known directory", name)
	}
	srcDir := filepath.Dir(scriptPath)

	tmp, err := os.MkdirTemp("", "tuiwall-upload-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	stagedPresetDir := filepath.Join(tmp, name)
	if err := copyDir(srcDir, stagedPresetDir); err != nil {
		return err
	}

	if err := runGit(tmp, "git", "init"); err != nil {
		return err
	}
	_ = runGit(tmp, "git", "checkout", "-b", "main")
	if err := runGit(tmp, "git", "add", "."); err != nil {
		return err
	}

	commitMsg := "Add tuiwall preset " + name
	if err := runGit(tmp, "git", "commit", "-m", commitMsg); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	_ = runGit(tmp, "git", "remote", "add", "origin", remote)
	if err := runGit(tmp, "git", "push", "-u", "origin", "main", "--force"); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	fmt.Println("Uploaded preset to:", remote)
	return nil
}

func communityRepoPR(name string) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("github CLI ('gh') not found. Please install it to use community features")
	}

	out, err := exec.Command("gh", "api", "user", "-q", ".login").Output()
	if err != nil {
		return fmt.Errorf("failed to verify GitHub identity. Please run: gh auth login")
	}
	currentUser := strings.TrimSpace(string(out))

	scriptPath, ok := presetScriptPath(name)
	if !ok {
		return fmt.Errorf("preset %q not found in any known directory", name)
	}
	srcDir := filepath.Dir(scriptPath)

	tmp, err := os.MkdirTemp("", "tuiwall-comm-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	repoTarget := "Mug-Costanza/tuiwall-presets"
	repoSSH := "git@github.com:Mug-Costanza/tuiwall-presets.git"

	if currentUser == "Mug-Costanza" {
		fmt.Println("--> Admin mode: Cloning directly...")
		err = runGit("", "git", "clone", "--depth=1", repoSSH, tmp)
	} else {
		fmt.Println("--> Contributor mode: Forking community repo...")
		err = runGit("", "gh", "repo", "fork", repoTarget, "--clone", "--", "--depth=1", tmp)
	}
	if err != nil {
		return fmt.Errorf("failed to prepare repository: %w", err)
	}

	dstDir := filepath.Join(tmp, "presets", name)
	_ = os.MkdirAll(dstDir, 0755)
	if err := copyDir(srcDir, dstDir); err != nil {
		return fmt.Errorf("failed to stage code: %w", err)
	}

	extensions := []string{".gif", ".png", ".jpg", ".jpeg"}
	var foundImg string
	var imgInfo os.FileInfo

	for _, ext := range extensions {
		localImg := filepath.Join(srcDir, name+ext)
		if st, err := os.Stat(localImg); err == nil {
			foundImg = localImg
			imgInfo = st
			break
		}
	}

	if foundImg != "" {
		fmt.Printf("--> Including preview image (%s) from preset folder...\n", filepath.Ext(foundImg))
		repoImgDir := filepath.Join(tmp, "images")
		_ = os.MkdirAll(repoImgDir, 0755)

		dstImgPath := filepath.Join(repoImgDir, name+filepath.Ext(foundImg))
		_ = copyFile(foundImg, dstImgPath, imgInfo.Mode())
	} else {
		fmt.Println("--> No preview image found. Tip: Use 'vhs' to generate a demo.gif!")
	}

	branchName := "add-preset-" + name
	_ = runGit(tmp, "git", "checkout", "-b", branchName)
	_ = runGit(tmp, "git", "add", ".")
	_ = runGit(tmp, "git", "commit", "-m", "New community preset: "+name)

	fmt.Println("--> Pushing to your remote...")
	if err := runGit(tmp, "git", "push", "-u", "origin", branchName); err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}

	fmt.Println("--> Creating Pull Request...")
	return runGit(tmp, "gh", "pr", "create",
		"--title", "Add "+name,
		"--body", "Submitted via tuiwall CLI",
		"--repo", repoTarget,
	)
}

func runGit(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func presetZip(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "template" {
		return "", fmt.Errorf("refusing to zip %q (template is reserved)", name)
	}
	srcDir, err := presetInstalledDirStrict(name)
	if err != nil {
		return "", err
	}

	// Write zip to current dir
	out := name + ".zip"
	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	base := filepath.Base(srcDir)
	root := filepath.Dir(srcDir)

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		// Ensure it zips as <name>/...
		if !strings.HasPrefix(rel, base+string(os.PathSeparator)) {
			return nil
		}

		w, err := zw.Create(rel)
		if err != nil {
			return err
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		_, err = io.Copy(w, in)
		return err
	})
	if err != nil {
		return "", err
	}

	return out, nil
}

func runMirror() {
	var conn net.Conn
	var err error
	socketPath := getSocketPath()

	for i := 0; i < 15; i++ {
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if err != nil {
		return
	}
	defer conn.Close()

	// Hide cursor (\x1b[?25l), disable wrap (\x1b[?7l), clear (\x1b[2J), home (\x1b[H)
	fmt.Print("\x1b[?25l\x1b[?7l\x1b[2J\x1b[H")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)
	go func() {
		for range sigChan {
			paneID := os.Getenv("TMUX_PANE")
			if paneID != "" {
				_ = tmux.ResizePaneHeight(paneID, HEADER_HEIGHT)
			}
			_ = exec.Command("tuiwall", "_update-master-size").Run()
		}
	}()

	if _, err := io.Copy(os.Stdout, conn); err != nil {
		os.Exit(0)
	}
}
