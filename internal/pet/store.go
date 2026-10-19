package pet

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"time"
)

const (
	stateVersion = 3

	// MaxAway caps how much time away counts.
	MaxAway = 12 * time.Hour
	// awayPerPoint is how long you can be gone before he loses a point of affection.
	awayPerPoint = 2 * time.Hour
)

// saveFile is everything Puchi remembers. Quitting doesn't reset him: the next
// launch picks up exactly where he was, and time spent away counts against him.
type saveFile struct {
	Version   int       `json:"version"`
	SavedAt   time.Time `json:"saved_at"`
	Bond      int       `json:"bond"`
	Hunger    int       `json:"hunger"`
	Happiness int       `json:"happiness"`
	Energy    int       `json:"energy"`
	Feeds     int       `json:"feeds"`
	Hair      int       `json:"hair"`
	Cloth     int       `json:"cloth"`
	Sleeping  bool      `json:"sleeping"`
	Loaf      bool      `json:"loaf"`
}

// DefaultPath is where Puchi lives: $XDG_STATE_HOME/terminal-pet/puchi.json
// (usually ~/.local/state/terminal-pet/puchi.json).
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "terminal-pet", "puchi.json"), nil
}

// Load restores Puchi and applies the time he spent alone. A missing file is
// not an error (you get a new pet); a corrupt one is moved aside to path+".bak"
// and reported, again together with a usable new pet.
//
// Saves from older versions (before v3) only carried the bond, so just that is
// restored from them.
func Load(path string, now time.Time) (*Pet, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return New(), err
	}

	var s saveFile
	if err := json.Unmarshal(data, &s); err != nil {
		_ = os.Rename(path, path+".bak")
		return New(), fmt.Errorf("save file was corrupt (kept as %s.bak), starting fresh: %w", path, err)
	}

	p := New()
	p.Bond = s.Bond
	if s.Version >= 3 {
		p.Hunger, p.Happiness, p.Energy = s.Hunger, s.Happiness, s.Energy
		p.Feeds, p.Hair, p.Cloth = s.Feeds, s.Hair, s.Cloth
		p.Sleeping, p.Loaf = s.Sleeping, s.Loaf
	}
	p.sanitize()
	p.catchUp(now.Sub(s.SavedAt), s.Version >= 3)
	return p, nil
}

// Save writes Puchi's state atomically.
func (p *Pet) Save(path string, now time.Time) error {
	data, err := json.MarshalIndent(saveFile{
		Version: stateVersion, SavedAt: now, Bond: p.Bond,
		Hunger: p.Hunger, Happiness: p.Happiness, Energy: p.Energy,
		Feeds: p.Feeds, Hair: p.Hair, Cloth: p.Cloth,
		Sleeping: p.Sleeping, Loaf: p.Loaf,
	}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".puchi-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// sanitize keeps hand-edited or damaged saves from producing impossible states.
func (p *Pet) sanitize() {
	p.Bond = max(BondMin, min(BondMax, p.Bond))
	p.Hunger = clamp(p.Hunger)
	p.Happiness = clamp(p.Happiness)
	p.Energy = clamp(p.Energy)
	p.Feeds = min(FeedLimit, max(0, p.Feeds))
	p.Hair = (p.Hair%NumStyles + NumStyles) % NumStyles
	p.Cloth = (p.Cloth%NumStyles + NumStyles) % NumStyles
}

func clamp(v int) int { return min(100, max(0, v)) }

// catchUp applies the time spent away: much gentler than live decay and capped
// at MaxAway, so he gets hungry and lonely but is never dead. On autopilot he
// looks after himself. The bond always suffers a little for being left.
func (p *Pet) catchUp(away time.Duration, restoredStats bool) {
	if away < 0 { // clock changed
		away = 0
	}
	capped := min(away, MaxAway)

	if restoredStats {
		hours := capped.Hours()
		drift := func(perHour float64) int { return int(math.Round(perHour * hours)) }
		switch {
		case p.Loaf:
			p.Sleeping = false
		case p.Sleeping:
			p.Energy += drift(30)
			p.Hunger += drift(8)
			p.Happiness -= drift(4)
			if p.Energy >= 100 {
				p.Sleeping = false
			}
		default:
			if p.Energy > 10 {
				p.Energy = max(10, p.Energy-drift(8))
			}
			p.Hunger += drift(20)
			p.Happiness -= drift(10)
		}
	}

	lost := int(capped / awayPerPoint)
	p.Bond = max(BondMin, p.Bond-lost)
	p.sanitize()

	switch tier := p.Tier(); {
	case away < time.Minute:
		p.say("👋 Back already? I barely noticed you left!", 16)
	case p.Hungry() && !p.Loaf:
		p.say(fmt.Sprintf("😭 You were gone %s... I'm starving!", formatAway(away)), 24)
	case p.Loaf:
		p.say("🍞 Welcome back! I looked after myself in Loaf Mode.", 20)
	case tier >= Likes && lost > 0:
		p.say(fmt.Sprintf("🥺 You were gone %s... I missed you!", formatAway(away)), 20)
	case tier >= Likes:
		p.say("💛 Welcome back!", 16)
	case tier == Neutral:
		p.say(fmt.Sprintf("👋 Welcome back! You were gone %s.", formatAway(away)), 20)
	default:
		p.say(fmt.Sprintf("😒 Oh. You're back. (%s)", formatAway(away)), 20)
	}
}

func formatAway(d time.Duration) string {
	d = d.Round(time.Minute)
	h, m := int(d.Hours()), int(d.Minutes())%60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dm", m)
	}
}
