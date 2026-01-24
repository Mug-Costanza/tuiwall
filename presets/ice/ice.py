#!/usr/bin/env python3
import curses
import random
import time

# 8-direction movement (dx, dy)
DIRS = [
    (0, -1),  # N
    (1, -1),  # NE
    (1, 0),   # E
    (1, 1),   # SE
    (0, 1),   # S
    (-1, 1),  # SW
    (-1, 0),  # W
    (-1, -1), # NW
]

def safe_add(stdscr, y, x, s, attr=0):
    try:
        stdscr.addstr(y, x, s, attr)
    except curses.error:
        pass

def in_bounds(x, y, w, h):
    return 0 <= x < w and 0 <= y < h

def glyph_for_move(dx, dy):
    # pick a character based on direction
    if dx == 0 and dy != 0:
        return "│"
    if dy == 0 and dx != 0:
        return "─"
    if dx == dy:
        return "╲"  # SE / NW
    return "╱"      # NE / SW

def combine_glyph(existing, new):
    # When branches cross, upgrade to junction glyphs (simple, but looks good)
    if existing == " ":
        return new
    if existing in ["│", "─", "╱", "╲"] and new in ["│", "─", "╱", "╲"]:
        # crossing -> plus-ish
        if existing != new:
            return "┼"
        return existing
    if existing == "┼":
        return "┼"
    # keep strongest-looking
    return existing

def main(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    # Colors (best effort)
    # 1 dim ice, 2 mid ice, 3 bright ice, 4 sparkle highlight
    try:
        curses.init_pair(1, 24, -1)   # deep blue
        curses.init_pair(2, 38, -1)   # cyan-ish
        curses.init_pair(3, 51, -1)   # bright aqua
        curses.init_pair(4, 15, -1)   # white
    except curses.error:
        curses.init_pair(1, 7, -1)
        curses.init_pair(2, 7, -1)
        curses.init_pair(3, 15, -1)
        curses.init_pair(4, 15, -1)

    stdscr.nodelay(True)
    stdscr.timeout(60)

    # ---- Tuning knobs ----
    STEPS_PER_TICK = 1          # grow this many steps per animation tick
    STEP_INTERVAL = 0.05        # seconds between ticks (higher = slower)
    BRANCH_CHANCE = 0.08        # chance a tip splits
    TIP_CAP = 180               # max active tips (keeps CPU sane)
    MAX_AGE = 1200              # older tips may retire
    SPARKLE_PROB = 0.018        # sparkle on newly grown cells

    # Pause/resume cadence (feels like time passing)
    GROW_FOR = (6.0, 12.0)      # seconds of growth
    PAUSE_FOR = (1.5, 3.2)      # seconds of pause

    # Bias: crystals like to grow outward from the seed
    OUTWARD_BIAS = 0.75         # 0..1, higher = more radial growth
    DIAG_BIAS = 0.25            # slight preference for diagonals (looks crystalline)

    last_step = time.time()
    mode = "grow"
    mode_until = time.time() + random.uniform(*GROW_FOR)

    # Grid is stored as characters + intensity (for subtle depth)
    grid = None
    inten = None
    tips = []
    seed = None

    def reset(h, w):
        nonlocal grid, inten, tips, seed, mode, mode_until, last_step
        grid = [[" " for _ in range(w)] for __ in range(h)]
        inten = [[0 for _ in range(w)] for __ in range(h)]
        tips = []

        # seed near lower-middle for nice “upward” growth in shallow panes
        sx = w // 2
        sy = int(h * 0.60) if h > 6 else max(0, h // 2)
        seed = (sx, sy)

        # place seed
        grid[sy][sx] = "•"
        inten[sy][sx] = 3
        # start with a few initial directions
        for d in [0, 1, 7, 2, 6]:  # mostly upward + sideways
            tips.append({"x": sx, "y": sy, "dir": d, "age": 0})

        mode = "grow"
        mode_until = time.time() + random.uniform(*GROW_FOR)
        last_step = time.time()

    while True:
        H, W = stdscr.getmaxyx()
        top = 2
        field_h = max(1, H - top)
        field_w = max(1, W)

        if grid is None or len(grid) != field_h or len(grid[0]) != field_w:
            reset(field_h, field_w)
            stdscr.erase()

        # Header
        now = time.strftime("%H:%M:%S")
        date = time.strftime("%a %b %d")
        safe_add(stdscr, 0, 0, ("  tuiwall  " + now).ljust(field_w)[:field_w], curses.A_BOLD)
        safe_add(stdscr, 1, 0, ("  " + date).ljust(field_w)[:field_w])

        t = time.time()

        # Mode switch (grow <-> pause)
        if t >= mode_until:
            if mode == "grow":
                mode = "pause"
                mode_until = t + random.uniform(*PAUSE_FOR)
            else:
                mode = "grow"
                mode_until = t + random.uniform(*GROW_FOR)

        # Step timing
        if mode == "grow" and (t - last_step) >= STEP_INTERVAL:
            last_step = t

            sx, sy = seed

            for _ in range(STEPS_PER_TICK):
                if not tips:
                    break

                # pick a random tip (creates organic spread)
                i = random.randrange(len(tips))
                tip = tips[i]
                x, y, d = tip["x"], tip["y"], tip["dir"]
                tip["age"] += 1

                # retire very old tips sometimes
                if tip["age"] > MAX_AGE and random.random() < 0.15:
                    tips.pop(i)
                    continue

                # Choose a next direction near current direction, biased outward
                # outward = direction that increases distance from seed
                candidates = [(d + k) % 8 for k in (-2, -1, 0, 1, 2)]
                best = None
                best_score = -1e9
                for cd in candidates:
                    dx, dy = DIRS[cd]
                    nx, ny = x + dx, y + dy
                    if not in_bounds(nx, ny, field_w, field_h):
                        continue

                    # prefer empty-ish spots (but allow crossing)
                    occupied_pen = 0.0 if grid[ny][nx] == " " else 0.65

                    # radial outward score
                    r0 = (x - sx) * (x - sx) + (y - sy) * (y - sy)
                    r1 = (nx - sx) * (nx - sx) + (ny - sy) * (ny - sy)
                    outward = 1.0 if r1 >= r0 else -0.4

                    # diagonal bonus
                    diag = 1.0 if (dx != 0 and dy != 0) else 0.0

                    score = (OUTWARD_BIAS * outward) + (DIAG_BIAS * diag) - occupied_pen
                    # tiny noise to avoid uniformity
                    score += (random.random() - 0.5) * 0.15

                    if score > best_score:
                        best_score = score
                        best = cd

                if best is None:
                    # stuck tip
                    tips.pop(i)
                    continue

                dx, dy = DIRS[best]
                nx, ny = x + dx, y + dy

                # Draw segment at next cell
                g = glyph_for_move(dx, dy)
                grid[ny][nx] = combine_glyph(grid[ny][nx], g)

                # Intensity: brighter near tips and younger growth
                base_int = 2 if tip["age"] < 120 else 1
                inten[ny][nx] = max(inten[ny][nx], base_int)

                # Move tip forward
                tip["x"], tip["y"], tip["dir"] = nx, ny, best

                # Occasional branching
                if len(tips) < TIP_CAP and random.random() < BRANCH_CHANCE:
                    # spawn a new tip that diverges a bit
                    nd = (best + random.choice([-2, -1, 1, 2])) % 8
                    tips.append({"x": nx, "y": ny, "dir": nd, "age": tip["age"] // 2})

        # Render field (persistent)
        for y in range(field_h):
            row = grid[y]
            for x in range(field_w):
                ch = row[x]
                if ch == " ":
                    continue

                level = inten[y][x]
                attr = curses.color_pair(1 if level <= 1 else (2 if level == 2 else 3))

                # rare sparkle on bright points (never flashing hard)
                if level >= 2 and random.random() < SPARKLE_PROB:
                    attr = curses.color_pair(4) | curses.A_BOLD
                    # little sparkle glyph sometimes
                    if ch in ["│", "─", "╱", "╲", "┼"] and random.random() < 0.35:
                        ch = "✦"

                safe_add(stdscr, top + y, x, ch, attr)

        # Subtle mode hint (very low-key)
        hint = "growing" if mode == "grow" else "resting"
        safe_add(stdscr, 0, max(0, field_w - len(hint) - 2), f" {hint} ", curses.color_pair(1))

        stdscr.refresh()

        # Exit on keypress
        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    curses.wrapper(main)

