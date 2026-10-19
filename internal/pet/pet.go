// Package pet is Puchi's brain: stats, state transitions, the Loaf-mode
// autopilot and persistence. It deliberately has no UI dependencies, so every
// rule in here can be unit-tested without a terminal.
package pet

import (
	"math/rand"
	"time"
)

const (
	// TickInterval is how often the UI should call Tick. Animations advance
	// one frame per tick.
	TickInterval = 250 * time.Millisecond

	// ticksPerStatus ticks make one "status step" (hunger/energy/happiness
	// drift): 12 * 250ms = 3s.
	ticksPerStatus = 12

	// FeedLimit is the buffer size at which Puchi panics and locks up.
	FeedLimit = 100
	// PressureAt is where warnings start and Loaf mode flushes proactively.
	PressureAt = 80
	// NumStyles is the number of hair styles and outfits.
	NumStyles = 4

	// Bond runs from BondMin (hates you) to BondMax (adores you).
	BondMin = -100
	BondMax = 100

	// Affection only grows once per cooldown, so spamming a key can't farm it.
	careCooldownTicks = 16 // 4s
)

const msgFull = "🛑 CRITICAL ERROR: System Full! Force toilet break needed [t]! 🚽"

// HairNames and ClothNames are shown when cycling the wardrobe.
var (
	HairNames  = [NumStyles]string{"Freshly shaved bald! 🥚", "Spiky Punk Fringe! ⚡", "Classy Top Hat! 🎩", "Minimalist Headband! 🎗️"}
	ClothNames = [NumStyles]string{"Comfy birthday suit! 🐳", "Classic Utility Jacket! 🧥", "Sharp Tuxedo Cross-Tie! 👔", "Warm Grid Scarf! 🧣"}
)

// Activity is what Puchi is visibly doing. Activities are mutually exclusive,
// which replaces the pile of isDancing/isBathing/... booleans.
type Activity int

const (
	Idle Activity = iota
	Eating
	Petted
	Dancing
	Bathing
	Brushing
	Playing
	Toilet
)

// Tier is how Puchi feels about you, in words.
type Tier int

const (
	Hates Tier = iota
	Dislikes
	Neutral
	Likes
	Adores
)

// Messages shown when a bond change crosses into a new tier.
var (
	tierUp = [...]string{
		Dislikes: "😒 Puchi... tolerates you. Barely.",
		Neutral:  "🙂 Puchi is willing to give you a chance.",
		Likes:    "😊 Puchi is warming up to you!",
		Adores:   "💖 Puchi adores you!",
	}
	tierDown = [...]string{
		Hates:    "💔 Puchi hates you now.",
		Dislikes: "😒 Puchi is starting to dislike you...",
		Neutral:  "😐 Puchi doesn't feel as close to you anymore.",
		Likes:    "😊 Puchi still likes you, but he's less starry-eyed.",
	}
)

// Pet is Puchi. His stats are saved on quit and restored on launch (store.go),
// so closing the program is never a way to reset him.
type Pet struct {
	Bond      int // -100 (hates you) .. +100 (adores you)
	Hunger    int
	Happiness int
	Energy    int
	Feeds     int // the "cache buffer"; weight derives from it
	Hair      int
	Cloth     int
	Sleeping  bool
	Loaf      bool // autopilot

	activity      Activity
	activityTicks int
	angry         bool
	angryTicks    int
	speech        string
	speechTicks   int // >0 counts down, 0 = expired, <0 = pinned until replaced
	pinFortune    bool
	frame         int
	awakeSteps    int
	careCooldown  int
	neglectSteps  int
	rng           *rand.Rand
}

// New returns a fresh Puchi.
func New() *Pet {
	return NewWithRand(rand.New(rand.NewSource(time.Now().UnixNano())))
}

// NewWithRand is New with a caller-supplied RNG (handy for deterministic tests).
func NewWithRand(r *rand.Rand) *Pet {
	return &Pet{
		Hunger:      30,
		Happiness:   70,
		Energy:      80,
		speech:      "Mwahaha! I am alive inside your terminal! 🔮",
		speechTicks: 12,
		rng:         r,
	}
}

// ---- read-only view for the UI ----

func (p *Pet) Activity() Activity { return p.activity }
func (p *Pet) Angry() bool        { return p.angry }
func (p *Pet) Speech() string     { return p.speech }
func (p *Pet) Frame() int         { return p.frame }
func (p *Pet) Blinking() bool     { return p.frame%12 == 0 }
func (p *Pet) Full() bool         { return p.Feeds >= FeedLimit }
func (p *Pet) Hungry() bool       { return p.Hunger > 70 }
func (p *Pet) Miserable() bool    { return p.Hungry() || p.Happiness < 30 }

// Tier summarises Bond.
func (p *Pet) Tier() Tier {
	switch {
	case p.Bond >= 60:
		return Adores
	case p.Bond >= 20:
		return Likes
	case p.Bond > -20:
		return Neutral
	case p.Bond > -60:
		return Dislikes
	}
	return Hates
}

// Weight is 0..5, one level per 20 feeds.
func (p *Pet) Weight() int { return p.Feeds / 20 }

// WeightKg is the display weight.
func (p *Pet) WeightKg() int { return 4 + p.Weight()*5 }

// ---- time ----

// Tick advances Puchi by one animation frame. It reports whether the autopilot
// would like a fortune fetched (the UI does that asynchronously and answers
// with SetFortune).
func (p *Pet) Tick() (wantFortune bool) {
	p.frame++

	if p.careCooldown > 0 {
		p.careCooldown--
	}
	if p.angryTicks > 0 {
		p.angryTicks--
		if p.angryTicks == 0 {
			p.angry = false
		}
	}
	if p.activityTicks > 0 {
		p.activityTicks--
		if p.activityTicks == 0 {
			p.activity = Idle
		}
	}
	if p.speechTicks > 0 {
		p.speechTicks--
		if p.speechTicks == 0 {
			p.speech = "..."
		}
	}

	if p.frame%ticksPerStatus == 0 {
		return p.statusStep()
	}
	return false
}

// statusStep is the 3-second heartbeat: stats drift, then the autopilot gets a
// chance to act.
func (p *Pet) statusStep() (wantFortune bool) {
	if p.Feeds >= PressureAt && !p.Full() && !p.Sleeping && !p.Loaf && p.speechTicks >= 0 {
		p.say("⚠️ [System Warning]: High buffer pressure detected. Toilet break suggested! 🚽", 12)
	}

	if p.Sleeping {
		p.Energy = min(100, p.Energy+10)
		p.Hunger = min(100, p.Hunger+2)
		if p.Energy == 100 {
			p.Sleeping = false
			p.say("🌅 (Puchi woke up refreshed on his own!)", 12)
		}
	} else {
		p.awakeSteps++
		if p.awakeSteps%2 == 0 { // -5% energy every 6s
			p.Energy = max(0, p.Energy-5)
		}
		p.Hunger = min(100, p.Hunger+3)
		p.Happiness = max(0, p.Happiness-2)
		if p.Hungry() { // hunger above 70% makes him sadder faster
			p.Happiness = max(0, p.Happiness-1)
		}
		if !p.Loaf { // on autopilot he's looking after himself
			p.neglect()
		}
		if p.Energy == 0 {
			p.Sleeping = true
			p.stopActivity()
			p.say("⚠️ *Thud* (Puchi collapsed from pure exhaustion!)", 16)
		}
	}

	if p.Loaf && !p.Sleeping && p.Energy > 0 && p.activityTicks == 0 {
		return p.autopilot()
	}
	return false
}

// autopilot is Loaf mode: Puchi looks after himself, most urgent need first.
func (p *Pet) autopilot() (wantFortune bool) {
	switch {
	case p.Feeds >= PressureAt:
		p.flush("🤖 [Autopilot] High buffer pressure! Flushing system cache... 🚽")
	case p.Hunger > 65:
		p.eat(25, 0, "🤖 [Autopilot] Eating a snack by myself! 🍪", 24)
	case p.Energy < 25:
		p.Sleeping = true
		p.stopActivity()
		p.say("🤖 [Autopilot] Taking a quick power nap... 💤", 32)
	case p.Happiness < 40:
		p.Happiness = min(100, p.Happiness+15)
		p.act(Bathing, "🤖 [Autopilot] Taking a self-care shower! 🧼", 24)
	case p.Happiness < 60 && p.Energy > 40:
		p.Happiness = min(100, p.Happiness+20)
		p.act(Playing, "🤖 [Autopilot] Chasing a rubber ball! ⚽", 28)
	default:
		switch p.rng.Intn(20) {
		case 0:
			p.Hair = p.rng.Intn(NumStyles)
			p.Cloth = p.rng.Intn(NumStyles)
			p.act(Brushing, "🤖 [Autopilot] Redressing my system configs! ✨", 24)
		case 1:
			p.pinFortune = false
			return true
		}
	}
	return false
}

// ---- interactions ----

// Feed gives Puchi a cookie.
func (p *Pet) Feed() {
	if !p.can("(Puchi will eat when he feels like it!)", "zzz (Puchi is fast asleep, don't shove food... 💤)") {
		return
	}
	hunger := p.Hunger
	p.eat(20, 5, "✨ Nom nom! *crunchy noises* 🍖", 16)
	switch {
	case hunger < 15: // force-feeding is not caring
		p.say("🤢 Puchi wasn't hungry... he eats it anyway, annoyed.", 16)
		p.adjustBond(-2)
	case hunger >= 50:
		p.care(3)
	default:
		p.care(1)
	}
	if p.Full() {
		p.adjustBond(-10)
		p.say(msgFull, 12)
	}
}

// Pat pets him.
func (p *Pet) Pat() {
	if !p.can("(Loaf Mode active: Puchi looks cozy and complete)", "zzz (Puchi twitches his ears but stays asleep... 🐱)") {
		return
	}
	if p.Tier() == Hates {
		p.say("😾 Puchi dodges your hand and glares at you.", 12)
		return
	}
	joy := 15
	if p.Tier() == Adores {
		joy = 20
	}
	p.Happiness = min(100, p.Happiness+joy)
	p.act(Petted, "❤️ Puchi purrs like a well-optimized system! ❤️", 8)
	p.care(2)
}

// Dance needs at least 20% energy.
func (p *Pet) Dance() {
	if !p.can("(Puchi is in deep loaf format, no dancing allowed)", "zzz (Puchi is sleeping deep, he can't hear the beat... 🎷)") {
		return
	}
	if p.Energy < 20 {
		p.say("❌ (Puchi is way too tired to dance right now!)", 12)
		return
	}
	p.Happiness = min(100, p.Happiness+10)
	p.Energy = max(0, p.Energy-8)
	p.act(Dancing, "🕺 *Bass Drops* Check out these layout moves! 🕺", 16)
	p.care(1)
}

// Bathe gives him a bath.
func (p *Pet) Bathe() {
	if !p.can("(Puchi washes behind his own ears in Loaf mode)", "zzz (Don't splash water on a sleeping pet! 🌊)") {
		return
	}
	p.Happiness = min(100, p.Happiness+10)
	p.Energy = max(0, p.Energy-5)
	p.act(Bathing, "🫧 *Scrub Scrub* Squeaky clean compilation! 🫧", 12)
	p.care(1)
}

// UseToilet flushes the buffer. When the buffer is completely full it works
// even on a sleeping Puchi (otherwise the lock-up could not be escaped).
func (p *Pet) UseToilet() {
	if p.Sleeping && !p.Full() {
		p.say("zzz (Wake him up first!)", 12)
		return
	}
	rescued := p.Feeds >= PressureAt
	p.Sleeping = false
	p.flush("🚽 *Flushhhhh* System cache cleared! Back to baseline speed. ✨")
	if rescued {
		p.care(2)
	}
}

// ToggleSleep puts him to bed, or wakes him up (grumpily).
func (p *Pet) ToggleSleep() {
	if !p.can("(Disable Loaf Mode [l] to handle sleep schedules)", "") {
		return
	}
	if p.Sleeping {
		p.Sleeping = false
		p.angry = true
		p.angryTicks = 16
		p.Happiness = max(0, p.Happiness-25)
		p.say("Academic routine offline! Why did you wake me up?! (╬◣_◢)", 12)
		p.adjustBond(-5)
		return
	}
	p.Sleeping = true
	p.angry = false
	p.stopActivity()
	p.say("💤 Goodnight... going to sleep mode.", 8)
}

// ToggleLoaf switches the autopilot on or off.
func (p *Pet) ToggleLoaf() {
	if !p.can("", "") {
		return
	}
	p.Loaf = !p.Loaf
	p.stopActivity()
	if p.Loaf {
		p.say("🍞 Loaf Mode ON: Puchi has engaged autopilot.", 12)
	} else {
		p.say("🛑 Loaf Mode OFF: Manual overrides restored.", 12)
	}
}

// RequestSpeech reports whether Puchi is willing to talk. If so the caller
// should fetch a fortune and hand it back via SetFortune.
func (p *Pet) RequestSpeech() bool {
	if !p.can("", "zzz (Puchi snores softly... 💭)") {
		return false
	}
	switch {
	case p.Tier() == Hates:
		p.say("😾 Puchi gives you the silent treatment.", 12)
		return false
	case p.angry:
		p.say("❌ (Puchi is too mad to talk right now!)", 12)
		return false
	case p.Hungry():
		p.say("❌ (Puchi grumbles... too hungry to talk!)", 12)
		return false
	}
	p.stopActivity()
	p.pinFortune = true
	p.say("💬 *clears throat*...", -1)
	return true
}

// SetFortune shows a fortune. Manually requested ones stay on screen until
// something else is said; autopilot ones fade after a while.
func (p *Pet) SetFortune(text string) {
	if p.Sleeping {
		return
	}
	ticks := 32
	if p.pinFortune {
		ticks = -1
	}
	p.say("💬 "+text, ticks)
}

// ---- wardrobe ----

// OpenWardrobe reports whether the wardrobe may be opened.
func (p *Pet) OpenWardrobe() bool {
	if !p.can("(Disable Loaf Mode [l] to open manual style settings)", "zzz (Wake him up before dressing him! 😴)") {
		return false
	}
	p.say("🎨 Wardrobe Open! Select structural styles below.", -1)
	return true
}

func (p *Pet) CloseWardrobe() { p.say("Exited wardrobe setup mode. 👕", 6) }

func (p *Pet) BrushTeeth() {
	if p.Sleeping {
		return
	}
	p.Happiness = min(100, p.Happiness+10)
	p.act(Brushing, "✨ *Scrubba-scrubba* Keeping the bytecode clean! 🦷", 10)
	p.care(1)
}

func (p *Pet) CycleHair() {
	if p.Sleeping {
		return
	}
	p.Hair = (p.Hair + 1) % NumStyles
	p.say("💇 "+HairNames[p.Hair], 8)
}

func (p *Pet) CycleCloth() {
	if p.Sleeping {
		return
	}
	p.Cloth = (p.Cloth + 1) % NumStyles
	p.say("✨ "+ClothNames[p.Cloth], 8)
}

// ---- helpers ----

// can reports whether an interactive action is allowed right now. If not it
// sets an explanation and returns false. An empty message means "allowed".
func (p *Pet) can(loafMsg, sleepMsg string) bool {
	switch {
	case p.Full():
		p.say(msgFull, 12)
		return false
	case p.Loaf && loafMsg != "":
		p.say(loafMsg, 12)
		return false
	case p.Sleeping && sleepMsg != "":
		p.say(sleepMsg, 12)
		return false
	}
	return true
}

func (p *Pet) say(text string, ticks int) {
	p.speech = text
	p.speechTicks = ticks
}

func (p *Pet) act(a Activity, text string, ticks int) {
	p.activity = a
	p.activityTicks = ticks
	p.say(text, ticks)
}

func (p *Pet) stopActivity() {
	p.activity = Idle
	p.activityTicks = 0
}

func (p *Pet) eat(hungerDrop, joy int, text string, ticks int) {
	p.Hunger = max(0, p.Hunger-hungerDrop)
	p.Happiness = min(100, p.Happiness+joy)
	p.Feeds = min(FeedLimit, p.Feeds+1)
	p.act(Eating, text, ticks)
}

func (p *Pet) flush(text string) {
	p.Feeds = 0
	p.act(Toilet, text, 20)
}

// care is affection earned by looking after him. Gains are rate-limited.
func (p *Pet) care(n int) {
	if p.careCooldown > 0 {
		return
	}
	p.careCooldown = careCooldownTicks
	p.adjustBond(n)
}

// adjustBond changes the bond and announces crossing into a new tier.
func (p *Pet) adjustBond(delta int) {
	before := p.Tier()
	p.Bond = max(BondMin, min(BondMax, p.Bond+delta))
	switch after := p.Tier(); {
	case after > before:
		p.say(tierUp[after], 16)
	case after < before:
		p.say(tierDown[after], 16)
	}
}

// neglect runs every status step: letting him stay hungry or miserable slowly
// costs affection (about -5/min when hungry, -10/min when starving).
func (p *Pet) neglect() {
	distress := 0
	switch {
	case p.Hunger >= 90:
		distress += 2
	case p.Hungry():
		distress++
	}
	if p.Happiness < 20 {
		distress++
	}
	if distress == 0 {
		p.neglectSteps = 0
		return
	}
	p.neglectSteps += distress
	if p.neglectSteps >= 4 {
		p.neglectSteps -= 4
		p.adjustBond(-1)
	}
}
