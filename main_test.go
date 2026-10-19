package main

import (
	"math/rand"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"terminal-pet/internal/pet"
)

func testModel() *model {
	return newModel(pet.NewWithRand(rand.New(rand.NewSource(1))), "", false)
}

func press(m *model, key string) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	if key == "esc" {
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	}
	m.Update(msg)
}

func TestFeedingViaKeyboard(t *testing.T) {
	m := testModel()
	press(m, "f")
	if m.pet.Hunger != 10 {
		t.Fatalf("hunger = %d, want 10", m.pet.Hunger)
	}
	if !strings.Contains(m.View(), "Nom nom") {
		t.Fatal("speech bubble should show the feeding line")
	}
}

func TestQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		m := testModel()
		var cmd tea.Cmd
		if k == "q" {
			_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
		} else {
			_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		}
		if cmd == nil {
			t.Fatalf("%s should quit", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%s did not produce a QuitMsg", k)
		}
	}
}

func TestWardrobeSwallowsOtherKeys(t *testing.T) {
	m := testModel()
	press(m, "c")
	if !m.inStyle {
		t.Fatal("wardrobe should open")
	}
	press(m, "f") // used to leak through and feed him
	if m.pet.Feeds != 0 {
		t.Fatal("feeding must not work inside the wardrobe")
	}
	press(m, "2")
	if m.pet.Hair != 1 {
		t.Fatal("hair should cycle")
	}
	press(m, "esc")
	if m.inStyle {
		t.Fatal("esc should close the wardrobe")
	}
}

func TestLayoutDoesNotJumpWhenHairChanges(t *testing.T) {
	m := testModel()
	press(m, "c")
	want := lipgloss.Height(m.View())
	for i := 0; i < pet.NumStyles; i++ {
		press(m, "2")
		if got := lipgloss.Height(m.View()); got != want {
			t.Fatalf("hair %d changed the height from %d to %d", m.pet.Hair, want, got)
		}
	}
}

func TestEveryMoodRendersWithoutOverflow(t *testing.T) {
	m := testModel()
	m.inline = true
	for _, act := range []func(){
		m.pet.Feed, m.pet.Pat, m.pet.Dance, m.pet.Bathe, m.pet.UseToilet, m.pet.BrushTeeth,
		m.pet.ToggleSleep, m.pet.ToggleSleep, m.pet.ToggleLoaf,
	} {
		act()
		for f := 0; f < 4; f++ {
			m.Update(tickMsg{})
			for _, line := range strings.Split(m.View(), "\n") {
				if lipgloss.Width(line) > frameWidth+2 {
					t.Fatalf("line wider than the frame: %q", line)
				}
			}
		}
	}
}

func TestTooSmallTerminal(t *testing.T) {
	m := testModel()
	m.Update(tea.WindowSizeMsg{Width: 20, Height: 5})
	if !strings.Contains(m.View(), "too small") {
		t.Fatal("expected a too-small message")
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	if strings.Contains(m.View(), "too small") {
		t.Fatal("large terminal should render Puchi")
	}
}

func TestBondBarReflectsTheRelationship(t *testing.T) {
	m := testModel()
	m.inline = true
	for _, bond := range []int{-100, -60, -7, 0, 7, 20, 60, 100} {
		m.pet.Bond = bond
		view := m.View()
		if !strings.Contains(view, "Bond:") || !strings.Contains(view, "│") {
			t.Fatalf("bond %d: no bond bar in view", bond)
		}
		for _, line := range strings.Split(view, "\n") {
			if lipgloss.Width(line) > frameWidth+2 {
				t.Fatalf("bond %d overflows the frame: %q", bond, line)
			}
		}
	}
	m.pet.Bond = 100
	if !strings.Contains(m.View(), "Adores you") {
		t.Fatal("expected the tier label")
	}
	m.pet.Bond = -100
	if !strings.Contains(m.View(), "Hates you") {
		t.Fatal("expected the tier label")
	}
}

func TestHatedPuchiDodgesPetting(t *testing.T) {
	m := testModel()
	m.pet.Bond = -90
	press(m, "p")
	if !strings.Contains(m.View(), "dodges") {
		t.Fatal("expected Puchi to dodge")
	}
}

func TestLayoutDoesNotShiftWhenTheTierChanges(t *testing.T) {
	m := testModel()
	m.inline = true
	col := -1
	for _, bond := range []int{-100, -40, 0, 40, 100} {
		m.pet.Bond = bond
		for _, line := range strings.Split(m.View(), "\n") {
			if i := strings.Index(line, "Bond:"); i >= 0 {
				if col >= 0 && i != col {
					t.Fatalf("bond %d moved the bar from column %d to %d", bond, col, i)
				}
				col = i
			}
		}
	}
}
