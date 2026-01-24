#!/usr/bin/env python3
import curses
import random
import time

# Stars by depth: (chars, drift_speed, density_factor)
LAYERS = [
    (["·", "."], 0.10, 0.40),   # far
    (["*", "·"], 0.22, 0.28),   # mid
    (["✦", "*"], 0.45, 0.18),   # near (if ✦ looks bad, change to ["+", "*"])
]

def safe_addstr(stdscr, y, x, s, attr=0):
    try:
        stdscr.addstr(y, x, s, attr)
    except curses.error:
        pass

def main(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    # Colors (best-effort)
    # 1 dim gray, 2 white, 3 bright white
    try:
        curses.init_pair(1, 8, -1)
        curses.init_pair(2, 7, -1)
        curses.init_pair(3, 15, -1)
    except curses.error:
        curses.init_pair(1, 0, -1)
        curses.init_pair(2, 0, -1)
        curses.init_pair(3, 0, -1)

    stdscr.nodelay(True)
    stdscr.timeout(60)

    stars = []  # each: [xf, y, layer_idx, glyph, twinkle_phase, twinkle_rate, base_bright]
    shooting = None  # dict or None
    last_spawn = 0.0
    last_shoot = 0.0

    # Tuning knobs (cozy defaults)
    BASE_SPAWN_RATE = 0.030      # probability per tick to spawn per layer (scaled by width)
    MAX_STARS_PER_COL = 1.25     # approximate cap: width * this
    TWINKLE_PROB = 0.10          # fraction of stars that twinkle
    SHOOTING_MIN_SEC = 18        # minimum seconds between shooting stars
    SHOOTING_CHANCE = 0.10       # chance to start a shooting star after min interval
    CLEAR_TRAILS = True          # keep background clean (recommended)

    def spawn_star(w, sky_h, layer_idx):
        glyphs, _, density = LAYERS[layer_idx]
        xf = random.random() * max(1, w - 1)
        y = random.randint(0, max(0, sky_h - 1))
        glyph = random.choice(glyphs)

        # Twinkle parameters
        tw = random.random() < TWINKLE_PROB
        tw_phase = random.random() * 6.28 if tw else 0.0
        tw_rate = random.uniform(0.03, 0.08) if tw else 0.0

        # Base brightness by depth (far dimmer)
        base = 1 if layer_idx == 0 else (2 if layer_idx == 1 else 2)
        return [xf, y, layer_idx, glyph, tw_phase, tw_rate, base]

    while True:
        h, w = stdscr.getmaxyx()
        top = 2
        sky_h = max(1, h - top)

        # Header
        now = time.strftime("%H:%M:%S")
        date = time.strftime("%a %b %d")
        safe_addstr(stdscr, 0, 0, ("  tuiwall  " + now).ljust(w)[:w], curses.A_BOLD)
        safe_addstr(stdscr, 1, 0, ("  " + date).ljust(w)[:w])

        t = time.time()

        # Soft cap on star count
        max_stars = int(w * MAX_STARS_PER_COL)

        # Spawn stars (scaled by width and per-layer density)
        if len(stars) < max_stars and (t - last_spawn) > 0.02:
            last_spawn = t
            for li, (_, _, density) in enumerate(LAYERS):
                # spawn probability scales with width and layer density
                p = BASE_SPAWN_RATE * (w / 80.0) * density
                if random.random() < p:
                    stars.append(spawn_star(w, sky_h, li))

        # Rare shooting star
        if shooting is None and (t - last_shoot) > SHOOTING_MIN_SEC:
            if random.random() < SHOOTING_CHANCE:
                last_shoot = t
                # Start near top-left-ish, moving down-right
                shooting = {
                    "x": random.randint(0, max(0, w // 3)),
                    "y": random.randint(0, max(0, sky_h // 3)),
                    "vx": random.uniform(1.2, 2.0),
                    "vy": random.uniform(0.6, 1.1),
                    "life": random.randint(10, 18),
                    "trail": []
                }

        # Clear the sky area each frame (keeps it clean, prevents smear in tmux)
        if CLEAR_TRAILS:
            for yy in range(sky_h):
                safe_addstr(stdscr, top + yy, 0, " " * w)

        # Update + draw stars
        new_stars = []
        for s in stars:
            xf, y, li, glyph, tw_phase, tw_rate, base = s
            _, speed, _ = LAYERS[li]

            # Drift left -> right (wrap around)
            xf += speed
            if xf >= w:
                xf -= w
            x = int(xf)

            # Twinkle by switching brightness level occasionally
            attr = curses.color_pair(base)
            if tw_rate > 0.0:
                tw_phase += tw_rate
                # small chance to brighten when phase wraps
                if (tw_phase % 6.28) < 0.10 and random.random() < 0.35:
                    attr = curses.color_pair(3) | curses.A_BOLD

            # Draw
            if 0 <= y < sky_h and 0 <= x < w:
                safe_addstr(stdscr, top + y, x, glyph, attr)

            # Keep star
            new_stars.append([xf, y, li, glyph, tw_phase, tw_rate, base])

        stars = new_stars[:max_stars]

        # Update + draw shooting star
        if shooting is not None:
            # draw trail first (dim)
            for tx, ty in shooting["trail"][-10:]:
                if 0 <= ty < sky_h and 0 <= tx < w:
                    safe_addstr(stdscr, top + ty, tx, "·", curses.color_pair(1))

            # move head
            shooting["x"] += shooting["vx"]
            shooting["y"] += shooting["vy"]
            hx = int(shooting["x"])
            hy = int(shooting["y"])
            shooting["trail"].append((hx, hy))

            # draw head (bright)
            if 0 <= hy < sky_h and 0 <= hx < w:
                safe_addstr(stdscr, top + hy, hx, "✦", curses.color_pair(3) | curses.A_BOLD)

            shooting["life"] -= 1
            if shooting["life"] <= 0 or hx >= w or hy >= sky_h:
                shooting = None

        stdscr.refresh()

        # Exit on keypress
        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    curses.wrapper(main)

