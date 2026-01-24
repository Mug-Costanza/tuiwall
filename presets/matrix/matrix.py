#!/usr/bin/env python3
import curses
import random
import time

GLYPHS = list(
    "アイウエオカキクケコサシスセソタチツテトナニヌネノ"
    "ハヒフヘホマミムメモヤユヨラリルレロワヲン"
    "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

HEADER_LINES = 2

def main(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    # Reduce “terminal decides to scroll” surprises
    stdscr.scrollok(False)
    stdscr.idlok(False)

    # Color pairs: tail, mid, head
    curses.init_pair(1, 2, -1)    # green
    curses.init_pair(2, 10, -1)   # bright green
    curses.init_pair(3, 15, -1)   # white

    stdscr.nodelay(True)
    stdscr.timeout(60)  # lower = faster

    def safe_addch(y, x, ch, attr=0):
        """Draw a single char safely, avoiding bottom-right cell scroll quirks."""
        try:
            h, w = stdscr.getmaxyx()
            if y < 0 or x < 0 or y >= h or x >= w:
                return
            # Avoid writing bottom-right cell; some terminals treat it as a scroll trigger
            if y == h - 1 and x == w - 1:
                return
            stdscr.addch(y, x, ch, attr)
        except curses.error:
            pass

    def safe_addnstr(y, x, s, n, attr=0):
        """Write a string safely, capped to n chars, never writing full-width."""
        try:
            h, w = stdscr.getmaxyx()
            if y < 0 or y >= h or x < 0 or x >= w:
                return
            # cap n so we never hit the last cell
            n = max(0, min(n, w - x - 1))
            if n <= 0:
                return
            stdscr.addnstr(y, x, s, n, attr)
        except curses.error:
            pass

    def clear_line(y):
        """Clear a line without writing a full-width string (prevents wrap/scroll)."""
        try:
            h, _ = stdscr.getmaxyx()
            if 0 <= y < h:
                stdscr.move(y, 0)
                stdscr.clrtoeol()
        except curses.error:
            pass

    def init_state(h, w):
        top = HEADER_LINES
        rain_h = max(1, h - top)

        state = []
        for _ in range(w):
            state.append({
                "active": False,
                "y": -1,
                "gap": random.randint(0, 40),
                "length": random.randint(6, 18),
                "speed": random.choice([1, 2, 2, 3]),
                "tick": 0,
            })
        return top, rain_h, state

    h, w = stdscr.getmaxyx()
    top, rain_h, state = init_state(h, w)
    stdscr.erase()

    MAX_ACTIVE_FRAC = 0.30
    TAIL = 20
    FADE_ROW_PROB = 0.05

    while True:
        nh, nw = stdscr.getmaxyx()
        if nh != h or nw != w:
            h, w = nh, nw
            top, rain_h, state = init_state(h, w)
            stdscr.erase()

        # --- Header (never write full width) ---
        now = time.strftime("%H:%M:%S")
        date = time.strftime("%a %b %d")

        # Clear header lines safely then redraw
        clear_line(0)
        clear_line(1)

        safe_addnstr(0, 0, "  tuiwall  " + now, w, curses.A_BOLD)
        safe_addnstr(1, 0, "  " + date, w, 0)

        # --- Fade old characters (only in animation area) ---
        for ay in range(rain_h):
            if random.random() < FADE_ROW_PROB:
                clear_line(top + ay)

        active_cols = sum(1 for s in state if s["active"])
        max_active = max(1, int(w * MAX_ACTIVE_FRAC))

        # --- Streams ---
        for x in range(w):
            s = state[x]

            if not s["active"]:
                if s["gap"] > 0:
                    s["gap"] -= 1
                    continue

                if active_cols < max_active and random.random() < 0.08:
                    s["active"] = True
                    s["y"] = -random.randint(0, rain_h // 2)
                    s["length"] = random.randint(6, 18)
                    s["speed"] = random.choice([1, 2, 2, 3])
                    s["tick"] = 0
                    active_cols += 1
                else:
                    s["gap"] = random.randint(6, 20)
                continue

            s["tick"] += 1
            if s["tick"] < s["speed"]:
                continue
            s["tick"] = 0
            s["y"] += 1
            y = s["y"]

            # Tail
            for k in range(TAIL, 0, -1):
                ty = y - k
                if 0 <= ty < rain_h and random.random() < 0.45:
                    safe_addch(
                        top + ty,
                        x,
                        random.choice(GLYPHS),
                        curses.color_pair(1)
                    )

            # Head
            if 0 <= y < rain_h:
                safe_addch(
                    top + y,
                    x,
                    random.choice(GLYPHS),
                    curses.color_pair(3) | curses.A_BOLD
                )

            # Mid
            if 0 <= y - 1 < rain_h and random.random() < 0.7:
                safe_addch(
                    top + y - 1,
                    x,
                    random.choice(GLYPHS),
                    curses.color_pair(2)
                )

            # End stream
            if y - s["length"] > rain_h:
                s["active"] = False
                s["gap"] = random.randint(12, 60)
                s["y"] = -1
                active_cols = max(0, active_cols - 1)

        stdscr.refresh()

        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    curses.wrapper(main)

