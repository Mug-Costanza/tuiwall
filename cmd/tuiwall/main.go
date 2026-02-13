package main

import "os/signal"
import "syscall"
import "strconv"
import "archive/zip"

import (
	"io"
	"fmt"
	"os"
	"strings"
	"time"
	"os/exec"
	"path/filepath"
	"sort"
	"net"
	"sync"
	"github.com/creack/pty"
	"tuiwall/internal/tmux"
)

func main() {

	if len(os.Args) < 2 {
		usage()
		return
	}

	switch os.Args[1] {
	case "enable":
	mustInTmux()
    	// once we're in tmux, continue normally:
    	exe := tmux.MustExecutablePath()
    	if err := enable(exe); err != nil {
        	fatal(err)
    	}
    	fmt.Println("tuiwall enabled")		
	case "disable":
		mustInTmux()
		if err:= disable(); err != nil {
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
    for _, p := range presets {
        fmt.Println(p)
    }
	case "help":
		usage()

	case "reset":
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
		fmt.Println("tuiwall reset")

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

    default: // Default for tuiwall preset command
        fatal(fmt.Errorf("usage: tuiwall preset <new|edit|path> ..."))
    }

	case "install":
   		if len(os.Args) < 3 {
     		  fatal(fmt.Errorf("usage: tuiwall install <path|git-url>"))
   		}

    		src := strings.TrimSpace(os.Args[2])
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
        fatal(fmt.Errorf("usage: tuiwall upload <preset-name> <git-remote-url> OR tuiwall upload <preset-name> --zip"))
    }
    name := strings.TrimSpace(os.Args[2])
    if len(os.Args) >= 4 && os.Args[3] == "--zip" {
        out, err := presetZip(name)
        if err != nil { fatal(err) }
        fmt.Println(out)
        return
    }
    if len(os.Args) < 4 {
        fatal(fmt.Errorf("usage: tuiwall upload <preset-name> <git-remote-url>"))
    }
    remote := strings.TrimSpace(os.Args[3])
    if err := presetUploadToGit(name, remote); err != nil {
        fatal(err)
    }

case "_update-master-size":
    // Get current max dimensions from tmux
    mw, mh, _ := tmux.MaxPaneSize() 
    if mw > 0 && mh > 0 {
        // Update the master session size
        _ = exec.Command("tmux", "resize-window", "-t", "tuiwall-master:0", "-x", fmt.Sprint(mw), "-y", "10").Run()
        
        // Signal the Master process to update PTY size
        // We can do this via the socket or a simple signal if we track the PID
        // For now, sending SIGWINCH to the Master process is the cleanest way:
        _ = exec.Command("pkill", "-SIGWINCH", "-f", "tuiwall _master").Run()
    }
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
	fmt.Println(`tuiwall (v0.1) - tmux "terminal wallpaper" header

Usage:
  tuiwall enable
  tuiwall disable
  tuiwall reset
  tuiwall set <preset>
  tuiwall list
  tuiwall status
  tuiwall preset <new|edit|path> <preset>
  tuiwall install <repo url>
  tuiwall uninstall <preset>
  tuiwall upload <preset> <repo url>
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

	// 1) Ensure a session exists (detached)
	// 2) Then attach to it
	// 3) Kick off tuiwall enable inside it using run-shell (safe + explicit)
	//
	// If session already exists, new-session -Ad is fine.
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
    enabled, _ := tmux.GetGlobalOption("@tuiwall_enabled")
    if strings.TrimSpace(enabled) == "1" {
        ensureHeadersAllWindows(exePath)
        return nil
    }

    _ = tmux.SetGlobalOption("@tuiwall_enabled", "1")
    
    // Create an empty output file for the mirrors to follow
    _ = exec.Command("touch", "/tmp/tuiwall.out").Run()

cw, ch, _ := tmux.CurrentClientSize()
if cw <= 0 { cw = 80 }
if ch <= 0 { ch = 24 }

// Pick the largest existing pane as the default render size
mw, mh, err := tmux.MaxPaneSize()
if err != nil || mw <= 0 || mh <= 0 {
	fmt.Println("mw & mh failed")
	// Fallback: current client size
	cw, ch, _ = tmux.CurrentClientSize()
	if cw > 0 { mw = cw } else { mw = 80 }
	if ch > 0 { mh = ch } else { mh = 24 }
}

masterCmd := fmt.Sprintf("%s _master", shellEscape(exePath))

    _ = exec.Command("tmux", "kill-session", "-t", "tuiwall-master").Run()

err = exec.Command("tmux", "new-session", "-d",
    "-x", fmt.Sprint(mw),
    "-y", fmt.Sprint(mh),
    "-s", "tuiwall-master",
    masterCmd,
).Run()

_ = exec.Command("tmux", "wait-for", "tuiwall_ready").Run()
    
if err != nil {
        return err
    }

    installHooks(exePath)
    ensureHeadersAllWindows(exePath)
    return nil
}

func disable() error {
    removeHooks()

    wins, err := tmux.ListSessionWindowIDs()
    if err == nil {

	// Completely kill all tuiwall instances
        for _, w := range wins {
            // 1) kill by tag (authoritative)
            if paneID, ok := tmux.FindHeaderPaneInWindow(w); ok && tmux.PaneExists(paneID) {
                _ = tmux.KillPaneAsync(paneID)
            }

            // 2) also kill by stored pointer (best effort)
            key := headerKeyForWindow(w)
            paneID, _ := tmux.GetGlobalOption(key)
            paneID = strings.TrimSpace(paneID)
            if paneID != "" && tmux.PaneExists(paneID) {
                _ = tmux.KillPaneAsync(paneID)
            }

            // 3) clear flags/pointers/locks
            _ = tmux.UnsetGlobalOption(key)
            _ = tmux.UnsetGlobalOption(lockKeyForWindow(w))
        }
    }

    _ = exec.Command("tmux", "set-window-option", "-t", "@", "-u", "window-style").Run()
    _ = exec.Command("tmux", "set-window-option", "-t", "@", "-u", "window-active-style").Run()

    _ = tmux.UnsetGlobalOption("@tuiwall_enabled")
    _ = tmux.UnsetGlobalOption("@tuiwall_mode")
    _ = tmux.UnsetGlobalOption("@tuiwall_exe")
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

func resolvePreset () string {
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
		HEADER_HEIGHT    = 10
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
		curPreset     string
		pendingPreset string
		pendingSince  time.Time
		cmd           *exec.Cmd
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

		c := exec.Command("python3", script)
		c.Stdin = os.Stdin
		c.Stdout = out // REDIRECTED TO PIPE
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


// We need to track the current command so the connection loop can signal it
var currentCmd *exec.Cmd
var cmdMu sync.Mutex

func startPresetWithPTY(preset string, clients *[]net.Conn, mu *sync.Mutex) {
    // \x1b[2J: Clear entire screen
    // \x1b[H: Move cursor to (1,1)
    // \x1b[?25l: Hide cursor
    // \x1b[39;49m: Reset colors to default
    clearSeq := []byte("\x1b[2J\x1b[H\x1b[?25l\x1b[39;49m")
    
    mu.Lock()
    for _, conn := range *clients {
        conn.Write(clearSeq)
    }
    mu.Unlock()

    script, _ := presetScriptPath(preset)
    c := exec.Command("python3", "-u", script) // Use -u for unbuffered output

    f, err := pty.Start(c)
    if err != nil {
        return
    }
    defer f.Close()

    cmdMu.Lock()
    currentCmd = c
    cmdMu.Unlock()

    // Sync PTY size immediately
    cw, _, _ := tmux.CurrentClientSize()
    if cw <= 0 { cw = 80 }
    _ = pty.Setsize(f, &pty.Winsize{Rows: 10, Cols: uint16(cw)})

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
            _, err := conn.Write(payload)
            if err == nil {
                activeClients = append(activeClients, conn)
            } else {
                conn.Close()
            }
        }
        *clients = activeClients
        mu.Unlock()
    }
}


func runMaster() {
    socketPath := "/tmp/tuiwall.sock"
    _ = os.Remove(socketPath)

    l, err := net.Listen("unix", socketPath)
    if err != nil {
        fatal(err)
    }
    defer l.Close()

    _ = exec.Command("tmux", "wait-for", "-S", "tuiwall_ready").Run()
    var clients []net.Conn
    var mu sync.Mutex

    // Signal channel to catch window resizes
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGWINCH)

    go func() {
        for {
            conn, err := l.Accept()
            if err == nil {
                mu.Lock()
                clients = append(clients, conn)
                mu.Unlock()

                // Trigger a resize on new connection to sync initial state
                sigChan <- syscall.SIGWINCH
            }
        }
    }()

    // Monitor for preset changes or resize signals
    go func() {
        lastPreset := resolvePreset()
        for {
            select {
            case <-sigChan:
                // When tmux resizes, update the PTY size
                cmdMu.Lock()
                if currentCmd != nil {
                    cw, _, _ := tmux.CurrentClientSize()
                    if cw > 0 {
                        // Update PTY size so Python knows the new width
                        _ = pty.Setsize(nil, &pty.Winsize{Rows: 10, Cols: uint16(cw)}) 
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
		HEADER_HEIGHT  = 10
		DEBOUNCE       = 250 * time.Millisecond
		RETRY_BACKOFF  = 1 * time.Second
		TICK           = 250 * time.Millisecond
		FAST_EXIT_CUTOFF = 500 * time.Millisecond
	)

	paneID := strings.TrimSpace(os.Getenv("TMUX_PANE"))

	forceRedraw := func() {
   		 // Clear entire pane and redraw background
   		 fmt.Print("\x1b[2J\x1b[H")
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
		curPreset      string
		pendingPreset  string
		pendingSince   time.Time
		switchBlanked  bool

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

		c := exec.Command("python3", script)
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
				nextStartAfter = time.Now().Add(RETRY_BACKOFF)
			}
		}(c, startedAt)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGWINCH)

	go func() {
		for s := range sig {
			switch s {
			case syscall.SIGWINCH:
				// Don't restart the renderer on resize — just enforce height.
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
			// start / update pending
			if desired != pendingPreset {
				pendingPreset = desired
				pendingSince = now
				switchBlanked = false
			}

			// During waiting/backoff, blank ONCE (no jitter).
			if !switchBlanked {
				blankScreen()
				switchBlanked = true
			}

			// If stable long enough AND not in backoff, start it.
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

		// If python died (or never started) and we're past backoff, restart.
		if cmd == nil && curPreset != "" && now.After(nextStartAfter) {
   			 if _, ok := presetScriptPath(curPreset); ok {
       			 	startPreset(curPreset)
			 }
		}
	}
}


func renderFallbackHeader(preset string) {
	const H = 10
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

func listPresets() ([]string, error) {
	seen := map[string]bool{}
	out := []string{}

	for _, dir := range presetDirs() {
		scanPresetDir(dir, seen, &out)
	}

	sort.Strings(out)
	return out, nil
}

func scanPresetDir(dir string, seen map[string]bool, out *[]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		name := e.Name()

		if e.IsDir() {
			// directory preset: <name>/<name>.py (we don't verify here; keep it fast)
			if name != "" && !seen[name] {
				seen[name] = true
				*out = append(*out, name)
			}
			continue
		}

		// flat preset: <name>.py
		if strings.HasSuffix(name, ".py") {
			base := strings.TrimSuffix(name, ".py")
			if validPresetName(base) && base != "template" && !seen[base] {
				seen[base] = true
				*out = append(*out, base)
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

	// 0) explicit override
	if d := strings.TrimSpace(os.Getenv("TUIWALL_PRESET_DIR")); d != "" {
		dirs = append(dirs, d)
	}

	// 1) user config (~/.config/tuiwall/presets on many systems)
	if cfg, err := os.UserConfigDir(); err == nil && cfg != "" {
		dirs = append(dirs, filepath.Join(cfg, "tuiwall", "presets"))
	}

	// 2) standard shared install locations (good for brew / system installs)
	dirs = append(dirs,
		"/usr/local/share/tuiwall/presets",
		"/opt/homebrew/share/tuiwall/presets", // Apple Silicon Homebrew
	)

	// 3) presets next to the binary (works for “single folder app” installs)
	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)
		dirs = append(dirs, filepath.Join(base, "presets"))
	}

	// 4) dev workflow: presets in repo relative to cwd
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(cwd, "presets"))
	}

	return dirs
}

func headerKeyForWindow(windowID string) string {
	return "@tuiwall_header_" + windowID
}

// Ensure header exists for one window
func ensureHeaderForWindow(exePath, windowID string) {
    const HEADER_HEIGHT = 10

    // 1. IMMEDIATE CHECK: If a header already exists, BAIL OUT.
    // This is the most important line to stop the infinite loop.
    if existingID, ok := tmux.FindHeaderPaneInWindow(windowID); ok {
        _ = tmux.ResizePaneHeight(existingID, HEADER_HEIGHT)
        return 
    }

    // 2. CONCURRENCY LOCK: Prevent hooks from overlapping
    lockKey := fmt.Sprintf("@tuiwall_lock_%s", windowID)
    lock, _ := tmux.GetGlobalOption(lockKey)
    if strings.TrimSpace(lock) == "1" {
        return
    }
    _ = tmux.SetGlobalOption(lockKey, "1")
    // Ensure we unlock when we are done
    defer tmux.UnsetGlobalOption(lockKey)

    // 3. THE COMMAND: Passive mirror (reading from a shared file is the most stable)
    // We use tail -f because multiple panes can read the same file without conflict.
    mirrorCmd := shellEscape(exePath) + " _mirror"

    // 4. SPLIT: This will trigger the hook again, but the check in Step 1 will catch it.
    newPane, err := tmux.SplitTopPaneInWindow(windowID, HEADER_HEIGHT, mirrorCmd)
    if err != nil {
        return
    }

    // 5. TAG: Mark this pane as a header so FindHeaderPaneInWindow sees it next time.
    _ = tmux.SetPaneOption(newPane, "@tuiwall_header", "1")
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
	const HEADER_HEIGHT = 10

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
			"-k",           // kill existing process
			"-t", paneID,
			cmd,
		).Run()

		_ = tmux.ResizePaneHeight(paneID, HEADER_HEIGHT)
	}
}

// Ensure all tmux windows have hooks
// Synchronization
func installHooks(exePath string) {
	// Store exe path globally so hooks keep working even if PATH differs
	_ = tmux.SetGlobalOption("@tuiwall_exe", exePath)

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
        return fmt.Errorf("missing template preset: %w", err)
    }
    templateDir := filepath.Dir(templateScript)

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
        // Enforce convention: must end up with <name>.py
        if _, err2 := os.Stat(newPy); err2 != nil {
            return fmt.Errorf("template preset must include template.py (so it can be renamed)")
        }
    }

    _ = replaceInFile(newPy, map[string]string{
        "template": name,
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
    if src == "" {
        return fmt.Errorf("usage: tuiwall install <path|git-url>")
    }

    // Heuristic: treat as URL/remote if it looks like one
    if looksLikeGitRemote(src) {
        return presetInstallFromGit(src)
    }

    // Otherwise local path
    return presetInstallFromPath(src)
}

func looksLikeGitRemote(s string) bool {
    s = strings.TrimSpace(s)
    return strings.HasPrefix(s, "http://") ||
        strings.HasPrefix(s, "https://") ||
        strings.HasPrefix(s, "git@") ||
        strings.HasSuffix(s, ".git")
}

// Install a preset from a local directory path.
// Contract:
//   - directory name is preset name (last path segment)
//   - must contain <name>.py OR a single .py at root (handled elsewhere)
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

func presetInstallFromGit(remote string) error {
    if _, err := exec.LookPath("git"); err != nil {
        return fmt.Errorf("git not found in PATH (required for installing from URLs)")
    }

    tmp, err := os.MkdirTemp("", "tuiwall-install-*")
    if err != nil {
        return err
    }
    defer os.RemoveAll(tmp)

    clone := exec.Command("git", "clone", "--depth", "1", remote, tmp)
    clone.Stdin = os.Stdin
    clone.Stdout = os.Stdout
    clone.Stderr = os.Stderr
    if err := clone.Run(); err != nil {
        return fmt.Errorf("git clone failed: %w", err)
    }

    // Find installable preset directory inside repo.
    presetDir, presetName, err := findPresetDirInRepo(tmp, remote)
    if err != nil {
        return err
    }

    // Copy into user preset home
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
    // 1) derive a default name from repo URL (tuiwall-preset-bar -> bar, or repo -> repo)
    derived := derivePresetNameFromRemote(remote)
    if derived != "" && validPresetName(derived) && derived != "template" {
        // Layout: <derived>/<derived>.py
        d := filepath.Join(repoPath, derived)
        s := filepath.Join(d, derived+".py")
        if st, e := os.Stat(s); e == nil && !st.IsDir() {
            return d, derived, nil
        }
        // Layout: root/<derived>.py
        s2 := filepath.Join(repoPath, derived+".py")
        if st, e := os.Stat(s2); e == nil && !st.IsDir() {
            return repoPath, derived, nil
        }
    }

    // 2) search presets/*/*/*.py pattern: presets/<x>/<x>.py
    candidates := []struct {
        dir  string
        name string
    }{}

    presetsDir := filepath.Join(repoPath, "presets")
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
            candidates = append(candidates, struct {
                dir  string
                name string
            }{dir: d, name: n})
        }
    }

    if len(candidates) == 1 {
        return candidates[0].dir, candidates[0].name, nil
    }
    if len(candidates) > 1 {
        // If derived matches one, prefer it.
        if derived != "" {
            for _, c := range candidates {
                if c.name == derived {
                    return c.dir, c.name, nil
                }
            }
        }
        names := []string{}
        for _, c := range candidates {
            names = append(names, c.name)
        }
        sort.Strings(names)
        return "", "", fmt.Errorf("repo contains multiple presets (%s); clone manually and run: tuiwall install <path-to-one>", strings.Join(names, ", "))
    }

// 2.5) If repo root contains exactly one directory that looks like a preset, use it.
rootEntries, _ := os.ReadDir(repoPath)
dirCandidates := []string{}
for _, e := range rootEntries {
    if e.IsDir() {
        n := e.Name()
        if validPresetName(n) && n != "template" {
            // check <n>/<n>.py
            s := filepath.Join(repoPath, n, n+".py")
            if st, e2 := os.Stat(s); e2 == nil && !st.IsDir() {
                dirCandidates = append(dirCandidates, n)
            }
        }
    }
}
if len(dirCandidates) == 1 {
    n := dirCandidates[0]
    return filepath.Join(repoPath, n), n, nil
}


    // 3) last resort: if repo root contains exactly one *.py, install that as derived name (or file base)
    rootEntries, _ = os.ReadDir(repoPath)
    pyFiles := []string{}
    for _, e := range rootEntries {
        if e.IsDir() {
            continue
        }
        if strings.HasSuffix(e.Name(), ".py") {
            pyFiles = append(pyFiles, e.Name())
        }
    }
    if len(pyFiles) == 1 {
        base := strings.TrimSuffix(pyFiles[0], ".py")
        if validPresetName(base) && base != "template" {
            return repoPath, base, nil
        }
        if derived != "" && validPresetName(derived) && derived != "template" {
            // We'll install as derived, but we should ensure <derived>.py exists (it doesn't),
            return "", "", fmt.Errorf("found single python file %q but it doesn't match a safe preset name", pyFiles[0])
        }
    }

    return "", "", fmt.Errorf("could not find an installable preset in repo. expected one of:\n- <name>/<name>.py\n- <name>.py at repo root\n- presets/<name>/<name>.py")
}

func derivePresetNameFromRemote(remote string) string {
    r := strings.TrimSpace(remote)

    // Strip trailing .git
    r = strings.TrimSuffix(r, ".git")

    // Take last path segment after / or :
    // https://github.com/foo/tuiwall-preset-bar -> tuiwall-preset-bar
    // git@github.com:foo/tuiwall-preset-bar -> tuiwall-preset-bar
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

    // Locate installed preset (user-owned)
    srcDir, err := presetInstalledDirStrict(name)
    if err != nil {
        return err
    }

    // Build a repo layout that your installer will detect:
    // repo/
    //   <name>/
    //     <name>.py
    //     assets...
    tmp, err := os.MkdirTemp("", "tuiwall-upload-*")
    if err != nil {
        return err
    }
    defer os.RemoveAll(tmp)

    repoRoot := tmp
    stagedPresetDir := filepath.Join(repoRoot, name)
    if err := copyDir(srcDir, stagedPresetDir); err != nil {
        return err
    }

    // git init, commit, push
    if err := runGit(repoRoot, "init"); err != nil { return err }

    _ = runGit(repoRoot, "checkout", "-b", "main")

    if err := runGit(repoRoot, "add", "."); err != nil { return err }
    if err := runGit(repoRoot, "commit", "-m", "Add tuiwall preset "+name); err != nil {
        return fmt.Errorf("git commit failed (do you have user.name/user.email set?): %w", err)
    }

    _ = runGit(repoRoot, "remote", "remove", "origin") 
    if err := runGit(repoRoot, "remote", "add", "origin", remote); err != nil { return err }

    if err := runGit(repoRoot, "push", "-u", "origin", "main"); err != nil {
        return fmt.Errorf("git push failed (auth/remote?): %w", err)
    }

    fmt.Println("uploaded preset:", name)
    fmt.Println("remote:", remote)
    fmt.Println("install with: tuiwall install", remote)
    return nil
}

func runGit(dir string, args ...string) error {
    cmd := exec.Command("git", args...)
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
        if err != nil { return err }
        if info.IsDir() { return nil }

        rel, err := filepath.Rel(root, path)
        if err != nil { return err }

        // Ensure it zips as <name>/...
        if !strings.HasPrefix(rel, base+string(os.PathSeparator)) {
            return nil
        }

        w, err := zw.Create(rel)
        if err != nil { return err }

        in, err := os.Open(path)
        if err != nil { return err }
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
    
    for i := 0; i < 15; i++ {
        conn, err = net.Dial("unix", "/tmp/tuiwall.sock")
        if err == nil {
            break
        }
        time.Sleep(200 * time.Millisecond)
    }

    if err != nil {
        return
    }
    defer conn.Close()    

    // \x1b[2J: Clear screen
    // \x1b[H: Move cursor to home
    // \x1b[?25l: Hide cursor
    // \x1b[7l: Disable line wrapping (prevents overflow to next line)
// Defensive Reset: 
    // 1. Reset attributes 
    // 2. Clear Screen 
    // 3. Hide Cursor 
    // 4. Disable Wrap
    fmt.Print("\x1b[0m\x1b[2J\x1b[H\x1b[?25l\x1b[7l")

    _, _ = io.Copy(os.Stdout, conn)
}

