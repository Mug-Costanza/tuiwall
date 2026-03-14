Output rain.gif
Set FontSize 20
Set Width 1200
Set Height 600
Set TypingSpeed 0ms

# Start a fresh tmux session inside the recording
Type "tmux" Enter
Type "tuiwall set rain" Enter
Type "tmux new-session -d -s vhs_session && tmux attach-session -t vhs_session" Enter

# Enable tuiwall and set the specific preset
Type "tuiwall enable" Enter

# Capture 10 seconds of animation
Sleep 10s

# Clean up so the GIF ends cleanly
Ctrl+B Type ":" Type "kill-session" Enter
