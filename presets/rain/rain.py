#!/usr/bin/env python3
import curses, random, time

def main(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    # Colors (may vary by terminal; still fine without color support)
    curses.init_pair(1, 12, -1)  # blue-ish
    curses.init_pair(2, 8,  -1)  # dim gray
    curses.init_pair(3, 15, -1)  # white highlight

    stdscr.nodelay(True)
    stdscr.timeout(60)  # overall tick rate

    drops = []
    last_spawn = 0.0

    # Tuning knobs
    SPAWN_INTERVAL = 0.04   # lower => more rain
    SPAWN_BURST    = (1, 4)
    MAX_DROPS      = 1200

    # Wind: slowly changes over time (left/right drift)
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

        # Wind updates (smooth, slow)
        if t - last_wind_change > 2.5:
            last_wind_change = t
            wind_target = random.uniform(-0.9, 0.9)
        wind += (wind_target - wind) * 0.03  # ease

        # Spawn new drops
        if t - last_spawn > SPAWN_INTERVAL and len(drops) < MAX_DROPS:
            last_spawn = t
            burst = random.randint(SPAWN_BURST[0], SPAWN_BURST[1])
            for _ in range(burst):
                x = random.randint(0, max(0, w - 1))
                y = -random.randint(0, sky_h // 2)
                speed = random.choice([1, 1, 1, 2])  # mostly fast
                # length of streak (how many cells drawn vertically each tick)
                length = random.choice([1, 1, 2, 2, 3])
                color = random.choice([1, 1, 2, 3])
                # store: x,y,speed,tick,length,color,prev_cells
                drops.append([x, y, speed, 0, length, color, []])

        new_drops = []
        for d in drops:
            x, y, speed, tick, length, color, prev = d

            # erase previous cells (NO TRAIL)
            for (px, py) in prev:
                if 0 <= py < sky_h and 0 <= px < w:
                    try:
                        stdscr.addstr(top + py, px, " ")
                    except curses.error:
                        pass
            prev = []

            tick += 1
            if tick >= speed:
                tick = 0
                # fall
                y += 1
                # drift with wind a bit
                if random.random() < 0.6:
                    x += int(round(wind))
                x = max(0, min(w - 1, x))

            # Draw new streak (a vertical line of length)
            if y < sky_h:
                for k in range(length):
                    yy = y - k
                    if 0 <= yy < sky_h:
                        ch = "|" if length <= 2 else "│"
                        # occasional sparkle
                        attr = curses.color_pair(color)
                        if k == 0 and random.random() < 0.08:
                            ch = "╷"
                            attr = curses.color_pair(3) | curses.A_BOLD
                        try:
                            stdscr.addstr(top + yy, x, ch, attr)
                            prev.append((x, yy))
                        except curses.error:
                            pass

                # tiny splash at bottom
                if y == sky_h - 1 and random.random() < 0.15:
                    for dx, ch in [(-1, "."), (0, "."), (1, ".")]:
                        sx = x + dx
                        sy = y
                        if 0 <= sx < w and 0 <= sy < sky_h:
                            try:
                                stdscr.addstr(top + sy, sx, ch, curses.color_pair(2))
                            except curses.error:
                                pass

                new_drops.append([x, y, speed, tick, length, color, prev])

        drops = new_drops
        stdscr.refresh()

        # Exit on keypress (nice for manual testing in the header pane)
        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    curses.wrapper(main)

