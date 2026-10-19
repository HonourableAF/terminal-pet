# 🐾 Puchi - The Ultimate Terminal Pet

An interactive, keyboard-driven retro tamagotchi clone built entirely inside the terminal using Go (`Bubble Tea` & `LipGloss`). Puchi has live stats, a state machine, an autopilot ("Loaf Mode"), a wardrobe, and **opinions about you**: treat him well and he adores you, treat him badly and he hates you. Quitting doesn't reset him - he's exactly as you left him, minus the time you were gone.

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
| **Bond** | -100 (hates you) to +100 (adores you). Starts neutral; see below. | Changes his face, and whether he lets you pet or talk to him. |
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

## 💞 Does Puchi like you?

The **Bond** bar shows how he feels about you, from `Hates you 💔` through `Dislikes`, `Neutral` and `Likes` to `Adores you 💖`.

**Looking after him earns affection** (at most once every 4 seconds, so mashing a key doesn't work):

- Feeding him when he's actually hungry: +3 (a snack: +1)
- Petting him: +2 - dancing, baths, tooth-brushing, and flushing the buffer when it's getting full: +1 to +2

**Mistreating him costs it:**

- Letting him stay hungry (over 70%) or miserable: about -5 per minute, doubled while he's starving (90%+)
- Stuffing him when he isn't hungry: -2
- Overfeeding him all the way to the system panic: -10
- Waking him up: -5
- Staying away for hours: -1 per 2 hours, capped at -6 (Loaf Mode doesn't count as neglect - he's looking after himself)

**It shows:** an adored Puchi has hearts in his eyes and enjoys being petted more. A Puchi who hates you sulks, dodges your hand, and gives you the silent treatment - but he'll still eat what you feed him, so you can win him back.

## 💾 There is no escape

Press `q` and Puchi is saved exactly as he is - hunger, happiness, energy, weight, outfit, Loaf Mode and bond - to `~/.local/state/terminal-pet/puchi.json` (or `$XDG_STATE_HOME/terminal-pet/puchi.json`). He is also saved every 30 seconds in case of a crash. Restarting doesn't reset him, and neither does a full buffer.

The time you were gone counts too (gently, and capped at 12 hours): he comes back hungrier, lonelier and less happy, never dead. A sleeping Puchi rests while you're away, and a Puchi on autopilot looks after himself. A corrupt save is kept as `puchi.json.bak` and Puchi starts fresh.

```
pet              # normal run
pet --reset      # abandon the saved Puchi and start over with a new one
pet --no-save    # a throwaway session: don't load or save anything
pet --inline     # draw in the normal screen instead of the alternate screen
```

---

## 🧬 Engineering Notes

- **Brain / body split:** `internal/pet` holds all the rules (stats, activities, autopilot, bond, saving and offline catch-up) with no UI imports and is covered by unit tests; `main.go` and `view.go` are a thin Bubble Tea layer on top.
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
