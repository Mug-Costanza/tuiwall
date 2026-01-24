#!/usr/bin/env python3
import curses
import random
import time
import math

ASH_CHARS = ["·", ".", "˙"]
EMBER_CHARS = ["*", "✦", "•"]  # if unicode is weird, use ["*", "+", "o"]

def safe_add(stdscr, y, x, s, attr=0):
    try:
        stdscr.addstr(y, x, s, attr)
    except curses.error:
        pass

def clamp(v, lo, hi):
    return lo if v < lo else hi if v > hi else v

def main(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    # Colors (best-effort)
    # 1 ash dark, 2 ash mid, 3 ash light, 4 ember orange/red, 5 ember bright
    try:
        curses.init_pair(1, 8,  -1)   # dark gray
        curses.init_pair(2, 7,  -1)   # light gray
        curses.init_pair(3, 15, -1)   # white-ish
        curses.init_pair(4, 208, -1)  # orange
        curses.init_pair(5, 196, -1)  # red
    except curses.error:
        curses.init_pair(1, 8,  -1)
        curses.init_pair(2, 7,  -1)
        curses.init_pair(3, 15, -1)
        curses.init_pair(4, 15, -1)
        curses.init_pair(5, 15, -1)

    stdscr.nodelay(True)
    stdscr.timeout(70)

    # Particles: [xf, yf, vy, vx, kind, life, ch, col, prev(x,y)]
    # kind: 0 ash, 1 ember
    parts = []
    last_spawn = 0.0
    last_ember = 0.0

    # Tuning knobs (cinematic calm)
    MAX_PARTS = 900
    ASH_SPAWN_INTERVAL = 0.03
    ASH_SPAWN_BURST = (1, 3)

    EMBER_MIN_SEC = 2.0
    EMBER_CHANCE = 0.35        # chance to spawn an ember after min interval
    EMBER_LIFE = (18, 40)      # frames
    CLEAR_BG = False           # we erase only previous particle cells

    # Slow “thermal” drift field (changes gently)
    wind = 0.0
    wind_target = 0.0
    last_wind_change = 0.0

    t0 = time.time()

    while True:
        h, w = stdscr.getmaxyx()
        top = 2
        sky_h = max(1, h - top)

        # Header
        now = time.strftime("%H:%M:%S")
        date = time.strftime("%a %b %d")
        safe_add(stdscr, 0, 0, ("  tuiwall  " + now).ljust(w)[:w], curses.A_BOLD)
        safe_add(stdscr, 1, 0, ("  " + date).ljust(w)[:w])

        t = time.time()
        dt = t - t0
        t0 = t

        # wind updates (very slow)
        if t - last_wind_change > 3.8:
            last_wind_change = t
            wind_target = random.uniform(-0.55, 0.55)
        wind += (wind_target - wind) * 0.02

        # Optional full clear (usually not needed)
        if CLEAR_BG:
            for yy in range(sky_h):
                safe_add(stdscr, top + yy, 0, " " * w)

        # Spawn ash particles at bottom
        if t - last_spawn > ASH_SPAWN_INTERVAL and len(parts) < MAX_PARTS:
            last_spawn = t
            burst = random.randint(*ASH_SPAWN_BURST)
            for _ in range(burst):
                x = random.randint(0, max(0, w - 1))
                y = sky_h + random.randint(0, max(1, sky_h // 3))  # start below
                # slow upward velocity (float), slight random variation
                vy = -random.uniform(0.18, 0.55)
                # gentle sideways meander + wind
                vx = random.uniform(-0.08, 0.08)
                ch = random.choice(ASH_CHARS)
                col = random.choice([1, 1, 1, 2, 2, 3])
                parts.append([float(x), float(y), vy, vx, 0, 0, ch, col, None])

        # Rare ember
        if (t - last_ember) > EMBER_MIN_SEC:
            if random.random() < EMBER_CHANCE and len(parts) < MAX_PARTS:
                last_ember = t
                x = random.randint(0, max(0, w - 1))
                y = sky_h - 1  # start near bottom
                vy = -random.uniform(0.35, 0.85)
                vx = random.uniform(-0.22, 0.22)
                life = random.randint(*EMBER_LIFE)
                ch = random.choice(EMBER_CHARS)
                col = 5 if random.random() < 0.35 else 4
                parts.append([float(x), float(y), vy, vx, 1, life, ch, col, None])

        # Update + draw particles
        new_parts = []
        for p in parts:
            xf, yf, vy, vx, kind, life, ch, col, prev = p

            # erase previous position (NO TRAIL)
            if prev is not None:
                px, py = prev
                if 0 <= py < sky_h and 0 <= px < w:
                    safe_add(stdscr, top + py, px, " ")

            # thermal wobble (subtle)
            wob = math.sin((yf * 0.12) + (t * 0.7) + (xf * 0.05)) * 0.06

            # update position
            xf += vx + wind * 0.18 + wob
            yf += vy

            # wrap horizontally
            if xf < 0: xf += w
            if xf >= w: xf -= w

            x = int(round(xf))
            y = int(round(yf))

            # ember fade: swap to ash as it dies
            if kind == 1:
                life -= 1
                p[5] = life
                # fade down in intensity
                if life < 10:
                    col = 4
                if life < 5:
                    ch = "·"
                    col = 2
                    kind = 0  # becomes ash
                    vy = -random.uniform(0.18, 0.45)

            # draw if visible
            if 0 <= y < sky_h:
                attr = curses.color_pair(col)
                if kind == 1 and col == 5:
                    attr |= curses.A_BOLD
                safe_add(stdscr, top + y, x, ch, attr)
                prev = (x, y)
            else:
                prev = None

            # keep if still relevant (ash above top disappears)
            if yf >= -2:
                new_parts.append([xf, yf, vy, vx, kind, life, ch, col, prev])

        parts = new_parts
        stdscr.refresh()

        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    curses.wrapper(main)

