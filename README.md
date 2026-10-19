# 🐾 Puchi - The Ultimate Terminal Pet

An interactive, keyboard-driven retro tamagotchi clone built entirely inside the terminal using Go (`Bubble Tea` & `LipGloss`). Puchi has live stats, a state machine, an autopilot ("Loaf Mode"), a wardrobe, and he **remembers you** - his state is saved between sessions, and he notices when you've been away.

## 📸 System Preview

[![Terminal Pet Puchi Interface Screenshot](puchi.png)](puchi.png)

*Puchi, running optimized in a custom terminal environment.*

---

## 🛠️ Operational Core Metrics

Puchi's life cycle runs on a 3-second heartbeat:

| Stat             | Mechanics                                                    | Impact                                                          |
| ---------------- | ------------------------------------------------------------ | --------------------------------------------------------------- |
| **Hunger**       | +3% every 3 seconds (+2% while asleep).                      | Above 70% he is "Miserable", can't talk, and loses happiness faster. |
| **Happiness**    | -2% every 3 seconds (-3% while hunger is above 70%).         | Below 30% he is "Miserable".                                    |
| **Energy**       | -5% every 6 seconds awake, +10% every 3 seconds asleep.      | Hitting 0% collapses him into sleep; he wakes by himself at 100%. |
| **Cache Buffer** | +1 per cookie, only emptied by the toilet.                   | 80 units: warnings. 100 units: system panic and everything locks except `[t]`. |

Weight follows the buffer: one level per 20 cookies.

---

## 🎮 Keybindings

### Core Interaction

- `[f]` **Feed:** Reduces hunger by 20%, +5% happiness, adds 1 to the buffer.
- `[p]` **Pet:** +15% happiness.
- `[s]` **Speak:** Puchi shares a short `fortune` (or a built-in line if `fortune` isn't installed). The text stays on screen until something else happens.
- `[t]` **Flush Buffer:** Toilet break; resets the buffer and his weight. Works even on a sleeping Puchi when the buffer is completely full.

### Active Routines

- `[d]` **Dance:** Animated dance, -8% energy. Needs at least 20% energy.
- `[b]` **Bath:** Bubbles! +10% happiness, -5% energy.
- `[e]` **Power Toggle:** Sends him to bed - or wakes him up into a grumpy routine (`╬◣_◢`, -25% happiness).

### System Modes

- `[l]` **Loaf Mode (Autopilot):** Puchi looks after himself: flushes before the buffer gets dangerous, eats, naps, showers, plays and redresses. If you close the program in Loaf Mode he still looks after himself while you're gone.
- `[c]` **Style Wardrobe:** Pause everything else and dress him up:
  * `[1]` Brush Teeth
  * `[2]` Cycle Hair (Bald 🥚, Punk Fringe ⚡, Top Hat 🎩, Headband 🎗️)
  * `[3]` Cycle Apparel (Suit 🐳, Jacket 🧥, Tuxedo 👔, Scarf 🧣)
  * `[c / Esc]` Back to monitoring.
- `[q / Ctrl+C]` **Quit:** saves and exits.

---

## 💾 He remembers

Puchi's state is saved to `~/.local/state/terminal-pet/puchi.json` (or `$XDG_STATE_HOME/terminal-pet/puchi.json`) every 30 seconds and on quit. When you come back, the time you were away counts - gently: up to 12 hours of it, at a much slower pace than live decay. A corrupt save is kept as `puchi.json.bak` and Puchi starts fresh.

```
pet              # normal run
pet --reset      # start over with a brand-new Puchi
pet --no-save    # don't load or save anything
pet --inline     # draw in the normal screen instead of the alternate screen
```

---

## 🧬 Engineering Notes

- **Brain / body split:** `internal/pet` holds all the rules (stats, activities, autopilot, persistence) with no UI imports and is covered by unit tests; `main.go` and `view.go` are a thin Bubble Tea layer on top.
- **Activities are an enum**, not a pile of booleans, so Puchi can never be dancing and bathing at once.
- **Deterministic time:** a single 250ms tick drives animation *and* a 3s status heartbeat, so stat decay can't drift against the animation clock.
- **Non-blocking fortunes:** `fortune` runs as an async command with a 2s timeout - the UI never freezes waiting for it.
- **Stable layout:** the hat area and speech bubble have fixed minimum heights, so changing hairstyle doesn't make the frame jump; the frame is centred in the terminal and a "terminal too small" notice is shown if it doesn't fit.

---

## 🐧 Linux Installation

### 1. Prerequisites

You need `go`. The `fortune` database is optional but makes him chattier:

- **Arch / EndeavourOS:** `sudo pacman -S go fortune-mod`
- **Debian / Ubuntu:** `sudo apt install golang-go fortune-mod fortunes-min`

### 2. Clone, build, run

```
git clone https://github.com/HonourableAF/terminal-pet.git
cd terminal-pet
go build -ldflags="-s -w" -o pet .
./pet
```

To run it from anywhere without sudo (make sure `~/.local/bin` is on your `PATH`):

```
mkdir -p ~/.local/bin
cp pet ~/.local/bin/
```

### 3. Develop

```
go vet ./...
go test ./...
```

## 🪟 Windows Installation (via WSL)

Open a terminal and run `wsl`. If you don't have a Linux distribution yet, install Ubuntu or Debian from the Microsoft Store first. Then follow the Linux instructions above inside WSL.
