// Puchi — a tamagotchi-style pet that lives in your terminal.
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"terminal-pet/internal/pet"
)

const autosaveEvery = 120 // ticks, i.e. every ~30s

type (
	tickMsg    time.Time
	fortuneMsg string
)

type model struct {
	pet           *pet.Pet
	inStyle       bool // wardrobe is open
	fetching      bool // a fortune lookup is in flight
	inline        bool // no alternate screen
	width, height int
	ticks         int
	path          string // "" disables saving
}

func newModel(p *pet.Pet, path string, inline bool) *model {
	return &model{pet: p, path: path, inline: inline}
}

func tick() tea.Cmd {
	return tea.Tick(pet.TickInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

var fallbackLines = []string{
	"Puchi is contemplating the nature of source code...",
	"There are two hard things: cache invalidation, naming things, and off-by-one errors.",
	"It works on my terminal. 🐾",
	"A watched build never compiles.",
	"Have you tried turning it off and on again?",
	"Hydrate. Stretch. Commit.",
	"sudo make me a sandwich.",
}

// fetchFortune asks the system's `fortune` for a line, off the UI goroutine and
// with a timeout, and falls back to a built-in line if it isn't installed.
func fetchFortune() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "fortune", "-s").Output()
		text := strings.Join(strings.Fields(string(out)), " ")
		if err != nil || text == "" {
			return fortuneMsg(fallbackLines[rand.Intn(len(fallbackLines))])
		}
		return fortuneMsg(text)
	}
}

func (m *model) save() error {
	if m.path == "" {
		return nil
	}
	return m.pet.Save(m.path, time.Now())
}

func (m *model) Init() tea.Cmd { return tick() }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		return m, m.handleKey(msg.String())

	case tickMsg:
		wantFortune := m.pet.Tick()
		if m.inStyle && (m.pet.Sleeping || m.pet.Loaf) {
			m.inStyle = false // collapsed mid-fitting
		}
		if m.ticks++; m.ticks%autosaveEvery == 0 {
			_ = m.save() // best effort; the final save on exit reports errors
		}
		cmds := []tea.Cmd{tick()}
		if wantFortune && !m.fetching {
			m.fetching = true
			cmds = append(cmds, fetchFortune())
		}
		return m, tea.Batch(cmds...)

	case fortuneMsg:
		m.fetching = false
		m.pet.SetFortune(string(msg))
	}
	return m, nil
}

func (m *model) handleKey(key string) tea.Cmd {
	p := m.pet
	if key == "q" || key == "ctrl+c" {
		return tea.Quit
	}
	if m.inStyle {
		m.handleStyleKey(key)
		return nil
	}
	switch key {
	case "f":
		p.Feed()
	case "p":
		p.Pat()
	case "d":
		p.Dance()
	case "b":
		p.Bathe()
	case "t":
		p.UseToilet()
	case "e":
		p.ToggleSleep()
	case "l":
		p.ToggleLoaf()
	case "c":
		m.inStyle = p.OpenWardrobe()
	case "s":
		if p.RequestSpeech() && !m.fetching {
			m.fetching = true
			return fetchFortune()
		}
	}
	return nil
}

func (m *model) handleStyleKey(key string) {
	switch key {
	case "c", "esc":
		m.inStyle = false
		m.pet.CloseWardrobe()
	case "1":
		m.pet.BrushTeeth()
	case "2":
		m.pet.CycleHair()
	case "3":
		m.pet.CycleCloth()
	}
}

func main() {
	reset := flag.Bool("reset", false, "abandon the saved Puchi and start over with a new one")
	noSave := flag.Bool("no-save", false, "don't load or save Puchi (a throwaway session)")
	inline := flag.Bool("inline", false, "draw in the normal screen instead of the alternate screen")
	flag.Parse()

	var path string
	if !*noSave {
		p, err := pet.DefaultPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "puchi: can't find a place to save, running without saving:", err)
		} else {
			path = p
		}
	}

	puchi := pet.New()
	if path != "" && !*reset {
		loaded, err := pet.Load(path, time.Now())
		if err != nil {
			fmt.Fprintln(os.Stderr, "puchi:", err)
		}
		puchi = loaded
	}

	opts := []tea.ProgramOption{}
	if !*inline {
		opts = append(opts, tea.WithAltScreen())
	}
	final, err := tea.NewProgram(newModel(puchi, path, *inline), opts...).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Alas, an error occurred: %v\n", err)
		os.Exit(1)
	}
	if fm, ok := final.(*model); ok {
		if err := fm.save(); err != nil {
			fmt.Fprintln(os.Stderr, "puchi: couldn't save:", err)
		}
	}
}
