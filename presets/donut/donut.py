#!/usr/bin/env python3
import curses
import math
import time

# Shading ramp (dark -> bright)
SHADE = ".,-~:;=!*#$@"

def main(stdscr):
    curses.curs_set(0)
    curses.start_color()
    curses.use_default_colors()

    # Optional color: bright white
    curses.init_pair(1, 15, -1)

    stdscr.nodelay(True)
    stdscr.timeout(30)  # lower = faster

    A = 0.0
    B = 0.0

    while True:
        h, w = stdscr.getmaxyx()
        top = 2
        ph = max(1, h - top)
        pw = max(1, w)

        # Header
        now = time.strftime("%H:%M:%S")
        date = time.strftime("%a %b %d")
        try:
            stdscr.addstr(0, 0, ("  tuiwall  " + now).ljust(pw)[:pw], curses.A_BOLD)
            stdscr.addstr(1, 0, ("  " + date).ljust(pw)[:pw])
        except curses.error:
            pass

        # Buffers
        out = [" "] * (pw * ph)
        zbuf = [0.0] * (pw * ph)

        # Donut constants (tweak for size/shape)
        R1 = 2.0   # tube radius
        R2 = 3.0   # ring radius
        K2 = 100.0   # distance from viewer
        # Projection scale based on terminal size
        K1 = pw * K2 * 1 / (8 * (R1 + R2))

        # Render
        theta = 0.0
        while theta < 2 * math.pi:
            phi = 0.0
            costh = math.cos(theta)
            sinth = math.sin(theta)

            while phi < 2 * math.pi:
                cosph = math.cos(phi)
                sinph = math.sin(phi)

                # Point on torus before rotation
                circlex = R2 + R1 * costh
                circley = R1 * sinth

                # 3D rotation (A around x, B around z-ish)
                x = circlex * (math.cos(B) * cosph + math.sin(A) * math.sin(B) * sinph) - circley * math.cos(A) * math.sin(B)
                y = circlex * (math.sin(B) * cosph - math.sin(A) * math.cos(B) * sinph) + circley * math.cos(A) * math.cos(B)
                z = K2 + math.cos(A) * circlex * sinph + circley * math.sin(A)
                ooz = 1.0 / z

                # Project to 2D
                xp = int(pw / 2 + K1 * ooz * x)
                yp = int(ph / 2 - K1 * ooz * y)

                # Luminance (simple directional light)
                L = (
                    cosph * costh * math.sin(B)
                    - math.cos(A) * costh * sinph
                    - math.sin(A) * sinth
                    + math.cos(B) * (math.cos(A) * sinth - costh * math.sin(A) * sinph)
                )

                if L > 0 and 0 <= xp < pw and 0 <= yp < ph:
                    idx = xp + pw * yp
                    if ooz > zbuf[idx]:
                        zbuf[idx] = ooz
                        shade_i = int(L * (len(SHADE) - 1))
                        if shade_i < 0: shade_i = 0
                        if shade_i >= len(SHADE): shade_i = len(SHADE) - 1
                        out[idx] = SHADE[shade_i]

                phi += 0.07
            theta += 0.03

        # Draw frame
        attr = curses.color_pair(1)
        for y in range(ph):
            row = "".join(out[y * pw:(y + 1) * pw])
            try:
                stdscr.addstr(top + y, 0, row[:pw], attr)
            except curses.error:
                pass

        stdscr.refresh()

        # Spin speed (tweak)
        A += 0.07
        B += 0.03

        # Exit on keypress
        if stdscr.getch() != -1:
            break

if __name__ == "__main__":
    curses.wrapper(main)

