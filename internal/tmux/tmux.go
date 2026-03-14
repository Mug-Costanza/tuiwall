package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func ResizePaneHeight(paneID string, height int) error {
	_, err := run("resize-pane", "-t", paneID, "-y", fmt.Sprint(height))
	return err
}

func MustExecutablePath() string {
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return exe
}

func run(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if s == "" {
			s = err.Error()
		}
		return "", errors.New("tmux " + strings.Join(args, " ") + ": " + s)
	}
	return s, nil
}

func SetGlobalOption(key, value string) error {
	_, err := run("set-option", "-gq", key, value)
	return err
}

func GetGlobalOption(key string) (string, error) {
	out, err := run("show-option", "-gqv", key)
	if err != nil {
		return "", err
	}
	return out, nil
}

func UnsetGlobalOption(key string) error {
	_, err := run("set-option", "-gu", key)
	return err
}

func SplitTopPane(height int, command string) (string, error) {
	out, err := run("split-window", "-vb", "-l", fmt.Sprint(height), "-P", "-F", "#{pane_id}", command)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func SplitTopPaneInWindow(windowID string, height int, command string) (string, error) {
	out, err := run("split-window", "-t", windowID, "-d", "-vb", "-l", fmt.Sprint(height), "-P", "-F", "#{pane_id}", command)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func KillPane(paneID string) error {
	_, err := run("kill-pane", "-t", paneID)
	return err
}

func SelectPaneDown() error {
	_, err := run("select-pane", "-D")
	return err
}

func CurrentPaneID() (string, error) {
	out, err := run("display-message", "-p", "#{pane_id}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func CurrentWindowID() (string, error) {
	out, err := run("display-message", "-p", "#{window_id}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func ListAllWindowIDs() ([]string, error) {
	out, err := run("list-windows", "-a", "-F", "#{window_id}")
	if err != nil {
		return nil, err
	}
	lines := []string{}
	for _, s := range strings.Split(out, "\n") {
		s = strings.TrimSpace(s)
		if s != "" {
			lines = append(lines, s)
		}
	}
	return lines, nil
}

func KillPaneAsync(paneID string) error {
	cmd := fmt.Sprintf("tmux kill-pane -t %q >/dev/null 2>&1 || true", paneID)
	_, err := run("run-shell", "-b", cmd)
	return err
}

func PaneExists(paneID string) bool {
	out, err := run("list-panes", "-a", "-F", "#{pane_id}")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == paneID {
			return true
		}
	}
	return false
}

func SetHookGlobal(name string, command string) error {
	_, err := run("set-hook", "-g", name, command)
	return err
}

func UnsetHookGlobal(name string) error {
	_, err := run("set-hook", "-gu", name)
	return err
}

func ListGlobalOptionsWithPrefix(prefix string) (map[string]string, error) {
	out, err := run("show-options", "-gq")
	if err != nil {
		return nil, err
	}
	res := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.Contains(line, prefix) {
			continue
		}

		fields := strings.Fields(line)
		for _, f := range fields {
			if strings.HasPrefix(f, prefix) {
				key := strings.Trim(f, `"`)
				v, err := GetGlobalOption(key)
				if err == nil {
					res[key] = strings.TrimSpace(v)
				}
				break
			}
		}
	}
	return res, nil
}

func SetPaneOption(paneID, key, value string) error {
	_, err := run("set-option", "-pt", paneID, key, value)
	return err
}

func GetPaneOption(paneID, key string) (string, error) {
	out, err := run("show-option", "-pt", paneID, "-qv", key)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

type Pane struct {
	ID     string
	Tagged bool
}

func ListPanesInWindow(windowID string) ([]Pane, error) {
	out, err := run("list-panes", "-t", windowID, "-F", "#{pane_id} #{@tuiwall_header}")
	if err != nil {
		return nil, err
	}

	var panes []Pane
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		p := Pane{ID: fields[0]}
		if len(fields) > 1 && strings.TrimSpace(fields[1]) != "" {
			p.Tagged = true
		}
		panes = append(panes, p)
	}
	return panes, nil
}

func FindHeaderPaneInWindow(windowID string) (string, bool) {
	panes, err := ListPanesInWindow(windowID)
	if err != nil {
		return "", false
	}
	for _, p := range panes {
		if p.Tagged {
			return p.ID, true
		}
	}
	return "", false
}

func ListSessionWindowIDs() ([]string, error) {
	out, err := run("list-windows", "-F", "#{window_id}")
	if err != nil {
		return nil, err
	}
	lines := []string{}
	for _, s := range strings.Split(out, "\n") {
		s = strings.TrimSpace(s)
		if s != "" {
			lines = append(lines, s)
		}
	}
	return lines, nil
}

func CurrentClientSize() (w int, h int, err error) {
	out, err := run("display-message", "-p", "#{client_width} #{client_height}")
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("unexpected client size output: %q", out)
	}
	// parse ints
	var wi, hi int
	_, err = fmt.Sscanf(fields[0], "%d", &wi)
	if err != nil {
		return 0, 0, err
	}
	_, err = fmt.Sscanf(fields[1], "%d", &hi)
	if err != nil {
		return 0, 0, err
	}
	return wi, hi, nil
}

// MaxPaneSize returns the size (w,h) of the largest pane currently known to the tmux server.
// "Largest" = greatest width; ties broken by height.
func MaxPaneSize() (w int, h int, err error) {
	out, err := run("list-panes", "-a", "-F", "#{pane_width} #{pane_height}")
	if err != nil {
		return 0, 0, err
	}

	bestW, bestH := 0, 0
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var pw, ph int
		if _, e := fmt.Sscanf(line, "%d %d", &pw, &ph); e != nil {
			continue
		}
		if pw > bestW || (pw == bestW && ph > bestH) {
			bestW, bestH = pw, ph
		}
	}

	if bestW <= 0 || bestH <= 0 {
		return 0, 0, fmt.Errorf("could not determine pane sizes")
	}
	return bestW, bestH, nil
}
