#!/usr/bin/env python3
import curses, random, time, math

LEAF_CHARS = ["*", "w", "~", "o"]
# If emoji look bad in your terminal, change to: LEAF_CHARS = ["*", "v", "w", "~", "o"]

def main(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    # Colors (these depend on terminal; still fine if unsupported)
    # We'll try some autumn-ish palette: yellow/orange/red/brown-ish (best-effort)
    curses.init_pair(1, 220, -1)  # yellow-ish (often works in 256-color terms; may map)
    curses.init_pair(2, 208, -1)  # orange-ish
    curses.init_pair(3, 196, -1)  # red-ish
    curses.init_pair(4, 94,  -1)  # brown-ish (may vary)
    curses.init_pair(5, 15,  -1)  # white highlight

    stdscr.nodelay(True)
    stdscr.timeout(70)  # calm

    leaves = []
    last_spawn = 0.0

    # Tuning knobs
    SPAWN_INTERVAL = 0.05   # lower => more leaves
    SPAWN_CHANCE = 0.85
    MAX_LEAVES = 140

    # Wind state
    wind = 0.0
    wind_target = 0.0
    last_wind_change = 0.0

    while True:
        h, w = stdscr.getmaxyx()
        top = 2
        sky_h = max(1, h - top)

        # Header
        now = time.strftime("%H:%M:%S")
        date = time.strftime("%a %b %d")
        stdscr.addstr(0, 0, ("  tuiwall  " + now).ljust(w)[:w], curses.A_BOLD)
        stdscr.addstr(1, 0, ("  " + date).ljust(w)[:w])

        t = time.time()

        # Change wind every few seconds
        if t - last_wind_change > random.uniform(2.5, 5.5):
            last_wind_change = t
            # occasional gust
            if random.random() < 0.25:
                wind_target = random.uniform(-2.2, 2.2)
            else:
                wind_target = random.uniform(-1.0, 1.0)

        wind += (wind_target - wind) * 0.04  # smooth easing

        # Spawn leaves
        if t - last_spawn > SPAWN_INTERVAL and len(leaves) < MAX_LEAVES:
            last_spawn = t
            if random.random() < SPAWN_CHANCE:
                x = random.randint(0, max(0, w - 1))
                y = -random.randint(0, sky_h // 2)
                fall_speed = random.choice([1, 1, 2])  # tick divisor
                color = random.choice([1, 2, 2, 3, 4])
                ch = random.choice(LEAF_CHARS)
                phase = random.uniform(0, math.tau)
                wobble = random.uniform(0.3, 1.2)      # horizontal wiggle
                wobble_speed = random.uniform(0.10, 0.22)
                rot_tick = random.randint(0, 3)

                # leaf: x(float), y(int), fall_speed, tick, color, ch, phase, wobble, wobble_speed, prev(x,y)
                leaves.append([float(x), y, fall_speed, 0, color, ch, phase, wobble, wobble_speed, None, rot_tick])

        new_leaves = []
        for leaf in leaves:
            xf, y, fall_speed, tick, color, ch, phase, wobble, wobble_speed, prev, rot_tick = leaf

            # erase old position (NO TRAIL)
            if prev is not None:
                px, py = prev
                if 0 <= py < sky_h and 0 <= px < w:
                    try:
                        stdscr.addstr(top + py, px, " ")
                    except curses.error:
                        pass

            tick += 1
            if tick >= fall_speed:
                tick = 0
                y += 1

                # horizontal drift: wind + sinusoidal wobble
                phase += wobble_speed
                xf += wind * 0.25 + math.sin(phase) * wobble * 0.25

                # clamp
                if xf < 0: xf = 0
                if xf > w - 1: xf = w - 1

            # Optional "spin" for ASCII-only leaves
            if ch in ["*", "v", "w", "~", "o", "❦"]:
                rot_tick = (rot_tick + 1) % 6
                if rot_tick == 0:
                    ch = random.choice(["v", "w", "*", "❦"])

            x = int(round(xf))

            # draw new position
            if 0 <= y < sky_h:
                try:
                    attr = curses.color_pair(color)
                    # occasional highlight glint
                    if random.random() < 0.02:
                        attr = curses.color_pair(5) | curses.A_BOLD
                    stdscr.addstr(top + y, x, ch, attr)
                except curses.error:
                    pass
                prev = (x, y)

            # keep leaf if still visible
            if y < sky_h:
                new_leaves.append([xf, y, fall_speed, tick, color, ch, phase, wobble, wobble_speed, prev, rot_tick])

        leaves = new_leaves
        stdscr.refresh()

        # exit on keypress
        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    curses.wrapper(main)

