package pet

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTest() *Pet { return NewWithRand(rand.New(rand.NewSource(1))) }

func tick(p *Pet, n int) {
	for i := 0; i < n; i++ {
		p.Tick()
	}
}

func TestStatsDriftOnTheDocumentedSchedule(t *testing.T) {
	p := newTest()
	tick(p, 2*ticksPerStatus) // two 3s steps = 6s
	if p.Hunger != 36 || p.Happiness != 66 {
		t.Fatalf("hunger/happiness = %d/%d, want 36/66", p.Hunger, p.Happiness)
	}
	if p.Energy != 75 { // -5% per 6s, no matter how the timers drift
		t.Fatalf("energy = %d, want 75", p.Energy)
	}
}

func TestHungerAbove70DrainsHappinessFaster(t *testing.T) {
	p := newTest()
	p.Hunger = 80
	p.statusStep()
	if p.Happiness != 70-3 {
		t.Fatalf("happiness = %d, want 67", p.Happiness)
	}
}

func TestFeed(t *testing.T) {
	p := newTest()
	p.Feed()
	if p.Hunger != 10 || p.Feeds != 1 || p.Activity() != Eating {
		t.Fatalf("bad feed state: hunger=%d feeds=%d act=%v", p.Hunger, p.Feeds, p.Activity())
	}
	p.Sleeping = true
	p.Feed()
	if p.Feeds != 1 {
		t.Fatal("slept-through feeding should be refused")
	}
}

func TestFullBufferLocksEverythingButToilet(t *testing.T) {
	p := newTest()
	p.Feeds = FeedLimit
	p.Feed()
	p.Dance()
	p.ToggleLoaf()
	if p.Feeds != FeedLimit || p.Loaf || p.Activity() != Idle {
		t.Fatal("actions should be locked while full")
	}
	p.UseToilet()
	if p.Feeds != 0 || p.Activity() != Toilet {
		t.Fatal("toilet should always work")
	}
}

func TestFullAndAsleepCanStillBeFlushed(t *testing.T) {
	p := newTest()
	p.Feeds, p.Sleeping = FeedLimit, true
	p.UseToilet()
	if p.Feeds != 0 || p.Sleeping {
		t.Fatal("emergency flush should wake him and clear the buffer")
	}
	p.Sleeping = true
	p.UseToilet()
	if !p.Sleeping {
		t.Fatal("normal toilet breaks still need him awake")
	}
}

func TestDanceNeedsEnergy(t *testing.T) {
	p := newTest()
	p.Energy = 19
	p.Dance()
	if p.Activity() == Dancing {
		t.Fatal("too tired to dance")
	}
	p.Energy = 20
	p.Dance()
	if p.Activity() != Dancing || p.Energy != 12 {
		t.Fatalf("energy=%d act=%v", p.Energy, p.Activity())
	}
}

func TestWakingHimUpMakesHimGrumpy(t *testing.T) {
	p := newTest()
	p.ToggleSleep()
	p.ToggleSleep()
	if p.Sleeping || !p.Angry() || p.Happiness != 45 {
		t.Fatalf("sleeping=%v angry=%v happiness=%d", p.Sleeping, p.Angry(), p.Happiness)
	}
	p.RequestSpeech()
	if p.pinFortune {
		t.Fatal("angry Puchi must not talk")
	}
}

func TestCollapseAndWake(t *testing.T) {
	p := newTest()
	p.Energy = 5
	p.awakeSteps = 1 // next awake step drains
	p.statusStep()
	if !p.Sleeping {
		t.Fatal("should collapse at 0 energy")
	}
	p.Energy = 90
	p.statusStep()
	if p.Sleeping || p.Energy != 100 {
		t.Fatal("should wake at full energy")
	}
}

func TestAutopilotPriorities(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*Pet)
		check func(*Pet) bool
	}{
		{"flushes before panic", func(p *Pet) { p.Feeds = PressureAt }, func(p *Pet) bool { return p.Feeds == 0 && p.Activity() == Toilet }},
		{"eats when hungry", func(p *Pet) { p.Hunger = 70 }, func(p *Pet) bool { return p.Feeds == 1 && p.Activity() == Eating }},
		{"naps when tired", func(p *Pet) { p.Energy = 20 }, func(p *Pet) bool { return p.Sleeping }},
		{"showers when sad", func(p *Pet) { p.Happiness = 30 }, func(p *Pet) bool { return p.Activity() == Bathing }},
		{"plays when bored", func(p *Pet) { p.Happiness = 55 }, func(p *Pet) bool { return p.Activity() == Playing }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newTest()
			p.Loaf = true
			c.setup(p)
			p.autopilot()
			if !c.check(p) {
				t.Fatalf("unexpected state: %+v", p)
			}
		})
	}
}

func TestAutopilotLeavesPinnedSpeechAlone(t *testing.T) {
	p := newTest()
	p.Loaf = true
	p.Hunger, p.Happiness, p.Energy = 20, 90, 90
	p.say("pinned", -1)
	for i := 0; i < 4; i++ { // stats stay comfortable, so only the idle branch runs
		p.statusStep()
	}
	if p.Speech() != "pinned" {
		t.Fatalf("speech was overwritten: %q", p.Speech())
	}
}

func TestFortuneIgnoredWhileAsleep(t *testing.T) {
	p := newTest()
	p.Sleeping = true
	p.SetFortune("hello")
	if p.Speech() == "💬 hello" {
		t.Fatal("sleeping pets don't talk")
	}
}

func TestPettingBuildsAffectionButNotByButtonMashing(t *testing.T) {
	p := newTest()
	p.Pat()
	if p.Bond != 2 {
		t.Fatalf("bond = %d, want 2", p.Bond)
	}
	p.Pat() // cooldown: no extra affection
	if p.Bond != 2 {
		t.Fatalf("spamming gave affection: %d", p.Bond)
	}
	tick(p, careCooldownTicks)
	p.Pat()
	if p.Bond != 4 {
		t.Fatalf("bond after cooldown = %d, want 4", p.Bond)
	}
}

func TestFeedingAffectionDependsOnHunger(t *testing.T) {
	cases := []struct{ hunger, want int }{
		{10, -2}, // force-fed: he wasn't hungry
		{30, 1},  // a snack
		{60, 3},  // fed when he really needed it
	}
	for _, c := range cases {
		p := newTest()
		p.Hunger = c.hunger
		p.Feed()
		if p.Bond != c.want {
			t.Errorf("hunger %d: bond = %d, want %d", c.hunger, p.Bond, c.want)
		}
	}
}

func TestOverfeedingToPanicCostsAffection(t *testing.T) {
	p := newTest()
	p.Feeds, p.Hunger = FeedLimit-1, 50
	p.Feed()
	if !p.Full() || p.Bond >= 0 {
		t.Fatalf("full=%v bond=%d: panic should hurt the relationship", p.Full(), p.Bond)
	}
}

func TestNeglectCostsAffection(t *testing.T) {
	p := newTest()
	for i := 0; i < 4; i++ { // four steps of "hungry" = -1
		p.Hunger = 80
		p.statusStep()
	}
	if p.Bond != -1 {
		t.Fatalf("hungry for 12s: bond = %d, want -1", p.Bond)
	}

	p = newTest()
	for i := 0; i < 2; i++ { // starving is twice as bad
		p.Hunger = 95
		p.statusStep()
	}
	if p.Bond != -1 {
		t.Fatalf("starving for 6s: bond = %d, want -1", p.Bond)
	}

	p = newTest()
	p.Loaf = true // autopilot is looking after himself
	for i := 0; i < 8; i++ {
		p.Hunger = 95
		p.statusStep()
	}
	if p.Bond != 0 {
		t.Fatalf("loaf mode should not count as neglect, bond = %d", p.Bond)
	}
}

func TestWellCaredForPetNeverLosesAffection(t *testing.T) {
	p := newTest()
	for i := 0; i < 20; i++ {
		p.Hunger, p.Happiness = 30, 70
		p.statusStep()
	}
	if p.Bond != 0 {
		t.Fatalf("bond = %d", p.Bond)
	}
}

func TestWakingHimCostsAffection(t *testing.T) {
	p := newTest()
	p.ToggleSleep()
	p.ToggleSleep()
	if p.Bond != -5 {
		t.Fatalf("bond = %d, want -5", p.Bond)
	}
}

func TestTiers(t *testing.T) {
	for bond, want := range map[int]Tier{-100: Hates, -60: Hates, -59: Dislikes, -20: Dislikes, -19: Neutral, 0: Neutral, 19: Neutral, 20: Likes, 59: Likes, 60: Adores, 100: Adores} {
		p := newTest()
		p.Bond = bond
		if got := p.Tier(); got != want {
			t.Errorf("bond %d: tier %v, want %v", bond, got, want)
		}
	}
}

func TestCrossingATierAnnouncesIt(t *testing.T) {
	p := newTest()
	p.Bond = 19
	p.Pat() // +2 -> Likes
	if p.Tier() != Likes || p.Speech() != tierUp[Likes] {
		t.Fatalf("tier=%v speech=%q", p.Tier(), p.Speech())
	}
	p.adjustBond(-5)
	if p.Tier() != Neutral || p.Speech() != tierDown[Neutral] {
		t.Fatalf("tier=%v speech=%q", p.Tier(), p.Speech())
	}
}

func TestBondIsClamped(t *testing.T) {
	p := newTest()
	p.adjustBond(500)
	p.adjustBond(-500)
	p.adjustBond(-500)
	if p.Bond != BondMin {
		t.Fatalf("bond = %d", p.Bond)
	}
}

func TestHatedPetRefusesPettingAndTalking(t *testing.T) {
	p := newTest()
	p.Bond = -80
	p.Happiness = 50
	p.Pat()
	if p.Happiness != 50 || p.Activity() == Petted {
		t.Fatal("he should dodge your hand")
	}
	if p.RequestSpeech() {
		t.Fatal("he should give you the silent treatment")
	}
	p.Feed() // ...but caring still works, so the relationship is recoverable
	if p.Feeds != 1 || p.Bond <= -80 {
		t.Fatalf("feeding should still work and help: feeds=%d bond=%d", p.Feeds, p.Bond)
	}
}

func TestAdoredPetEnjoysPettingMore(t *testing.T) {
	p := newTest()
	p.Bond, p.Happiness = 70, 50
	p.Pat()
	if p.Happiness != 70 {
		t.Fatalf("happiness = %d, want 70", p.Happiness)
	}
}

func TestQuittingDoesNotResetAnything(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "puchi.json")
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

	p := newTest()
	p.Bond, p.Hunger, p.Happiness, p.Energy = -42, 81, 23, 64
	p.Feeds, p.Hair, p.Cloth, p.Sleeping = 77, 2, 3, true
	if err := p.Save(path, now); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, now.Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bond != -42 || got.Hunger != 81 || got.Happiness != 23 || got.Energy != 64 ||
		got.Feeds != 77 || got.Hair != 2 || got.Cloth != 3 || !got.Sleeping {
		t.Fatalf("round trip lost data: %+v", got)
	}
}

func TestTimeAwayCountsAgainstHim(t *testing.T) {
	path := filepath.Join(t.TempDir(), "puchi.json")
	now := time.Now()
	p := newTest()
	_ = p.Save(path, now)

	got, _ := Load(path, now.Add(3*time.Hour))
	if got.Hunger != 90 || got.Happiness != 40 || got.Energy != 56 || got.Bond != -1 {
		t.Fatalf("after 3h: %+v", got)
	}

	got, _ = Load(path, now.Add(30*24*time.Hour)) // a month: capped, never negative
	if got.Hunger != 100 || got.Happiness != 0 || got.Energy != 10 || got.Bond != -6 {
		t.Fatalf("after a month: %+v", got)
	}
}

func TestSleepingPetRestsWhileAway(t *testing.T) {
	path := filepath.Join(t.TempDir(), "puchi.json")
	now := time.Now()
	p := newTest()
	p.Sleeping, p.Energy = true, 40
	_ = p.Save(path, now)
	got, _ := Load(path, now.Add(2*time.Hour))
	if got.Energy != 100 || got.Sleeping {
		t.Fatalf("should have slept it off: %+v", got)
	}
}

func TestLoafPetLooksAfterHimselfWhileAway(t *testing.T) {
	path := filepath.Join(t.TempDir(), "puchi.json")
	now := time.Now()
	p := newTest()
	p.Loaf = true
	_ = p.Save(path, now)
	got, _ := Load(path, now.Add(8*time.Hour))
	if got.Hunger != 30 || got.Happiness != 70 || !got.Loaf {
		t.Fatalf("loaf pet should not decay: %+v", got)
	}
}

func TestSanitizeClampsTamperedSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "puchi.json")
	_ = os.WriteFile(path, []byte(`{"version":3,"saved_at":"2026-10-19T12:00:00Z","bond":9999,"hunger":999,"energy":-5,"feeds":5000,"hair":-1,"cloth":9}`), 0o644)
	got, _ := Load(path, time.Date(2026, 10, 19, 12, 0, 5, 0, time.UTC))
	if got.Bond != BondMax || got.Hunger != 100 || got.Energy != 0 || got.Feeds != FeedLimit || got.Hair != 3 || got.Cloth != 1 {
		t.Fatalf("not clamped: %+v", got)
	}
}

func TestBeingAwayCostsAffectionUpToACap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "puchi.json")
	now := time.Now()
	p := newTest()
	p.Bond = 40
	_ = p.Save(path, now)

	for away, want := range map[time.Duration]int{
		time.Hour:           40, // short trips are free
		5 * time.Hour:       38, // a point per 2h
		30 * 24 * time.Hour: 34, // capped at 12h => -6
	} {
		got, _ := Load(path, now.Add(away))
		if got.Bond != want {
			t.Errorf("away %v: bond = %d, want %d", away, got.Bond, want)
		}
	}
}

func TestOlderSavesOnlyRestoreTheBond(t *testing.T) {
	path := filepath.Join(t.TempDir(), "puchi.json")
	old := `{"version":2,"saved_at":"2026-10-19T12:00:00Z","bond":33}`
	_ = os.WriteFile(path, []byte(old), 0o644)
	got, err := Load(path, time.Date(2026, 10, 19, 12, 0, 5, 0, time.UTC))
	if err != nil || got.Bond != 33 || got.Hunger != 30 || got.Energy != 80 {
		t.Fatalf("err=%v got=%+v", err, got)
	}
}

func TestMissingAndCorruptSaves(t *testing.T) {
	dir := t.TempDir()
	if p, err := Load(filepath.Join(dir, "nope.json"), time.Now()); err != nil || p == nil {
		t.Fatalf("missing file should give a fresh pet, got err=%v", err)
	}

	bad := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(bad, []byte("{not json"), 0o644)
	p, err := Load(bad, time.Now())
	if err == nil || p == nil || p.Bond != 0 {
		t.Fatalf("corrupt file should report an error and give a fresh pet (err=%v)", err)
	}
	if _, statErr := os.Stat(bad + ".bak"); statErr != nil {
		t.Fatal("corrupt save should be kept as .bak")
	}
}

func TestWeight(t *testing.T) {
	p := newTest()
	for feeds, want := range map[int]int{0: 4, 19: 4, 20: 9, 80: 24, 100: 29} {
		p.Feeds = feeds
		if got := p.WeightKg(); got != want {
			t.Errorf("feeds=%d: %dkg, want %dkg", feeds, got, want)
		}
	}
}
