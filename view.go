package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"terminal-pet/internal/pet"
)

const (
	frameWidth = 54
	innerWidth = frameWidth - 4 // minus horizontal padding
)

var (
	petStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	actionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	bubbleStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("242")).Width(42).Height(3).Padding(0, 1)
	frameStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).BorderForeground(lipgloss.Color("240")).Width(frameWidth)
)

var (
	hairArt  = [pet.NumStyles]string{"", "|||||||", " ___ \n|___|", "[=====]"}
	clothArt = [pet.NumStyles]string{"=====", "▓▓▓▓▓", "[ X ]", "#####"}

	danceFaces = []string{"/ ( ^ ‿ ^ )", "( ^ ‿ ^ )\\", "\\ ( ^ ‿ ^ )", "( ^ ‿ ^ )/"}
)

func (m *model) View() string {
	frame := frameStyle.Render(m.body())
	if m.inline || m.width == 0 {
		return frame + "\n"
	}
	if lipgloss.Width(frame) > m.width || lipgloss.Height(frame) > m.height {
		return fmt.Sprintf("Terminal too small for Puchi: need %dx%d, have %dx%d.",
			lipgloss.Width(frame), lipgloss.Height(frame), m.width, m.height)
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, frame)
}

func (m *model) body() string {
	p := m.pet
	emotion, face := mood(p)

	center := func(s string) string { return lipgloss.PlaceHorizontal(innerWidth, lipgloss.Center, s) }

	bubble := center(bubbleStyle.Render(actionStyle.Render(p.Speech())))

	avatar := lipgloss.JoinVertical(lipgloss.Center,
		hairBlock(hairArt[p.Hair], lipgloss.Width(face)), face, armLine(p))
	avatar = petStyle.Width(innerWidth).Align(lipgloss.Center).Render(avatar)

	weight := fmt.Sprintf("%d kg", p.WeightKg())
	if p.Weight() >= 4 {
		weight += " (🚨 Unit)"
	}
	status := center(strings.Join([]string{
		statLine("Hunger:", p.Hunger, true),
		statLine("Happiness:", p.Happiness, false),
		statLine("Energy:", p.Energy, false),
		"",
		statusStyle.Render(pad("Emotion: "+emotion, 24) + "Weight: " + weight),
	}, "\n"))

	help := "[f] Feed   [p] Pet     [d] Dance  [b] Bath\n" +
		"[t] Toilet [s] Speak   [c] Style  [l] Loaf\n" +
		"[e] Sleep  [q] Quit"
	helpStyle := statusStyle
	if m.inStyle {
		help = "[1] Brush Teeth   [2] Change Hair\n[3] Clothes        [c] Return Back"
		helpStyle = actionStyle
	}

	return strings.Join([]string{bubble, "", avatar, "", status, "", center(helpStyle.Render(help))}, "\n")
}

// mood picks the face and the emotion label. The order is the priority: being
// asleep beats being grumpy beats whatever he is currently doing, and so on.
func mood(p *pet.Pet) (emotion, face string) {
	f := p.Frame()
	switch {
	case p.Sleeping:
		return "Sleeping 💤", alt(f, "(- ‿ -) zZ", "(- ‿ -)")
	case p.Angry():
		return "Grumpy 💢", "(> ◣ <)"
	}

	switch p.Activity() {
	case pet.Dancing:
		return "Grooving 🕺", danceFaces[f%len(danceFaces)]
	case pet.Bathing:
		return "Bathing 🧼", alt(f, "o . ( ˘ ᵕ ˘ ) .", ". o ( ˘ ᵕ ˘ )o.")
	case pet.Brushing:
		return "Brushing ✨", alt(f, "* ( - ‿ - ) *", "*( - ‿ - )*")
	case pet.Playing:
		return "Playing ⚽", alt(f, "(っ^ ‿ ^)っ ◯", "◯ (っ^ ‿ ^)っ")
	case pet.Toilet:
		return "Relieved 🚽", "🚽 [ ˘ ᵕ ˘ ]"
	case pet.Eating:
		return "Munching 🍪", alt(f, "(っ^ ‿ ^)っ 🍪", "(っ〇 ‿ 〇)っ 🍪")
	case pet.Petted:
		return "Happy ✨", "(っ. ‿ .)っ"
	}

	switch {
	case p.Full():
		return "PANIC 🚨", "(╬〇 ◣ 〇)"
	case p.Loaf:
		if p.Blinking() {
			return "Loafing 🍞", "(- ‿ -)"
		}
		return "Loafing 🍞", "(っ- ‿ -)っ"
	case p.Miserable():
		return "Miserable 😭", "(; ﹏ ;)"
	case p.Blinking():
		return "Content 😐", "(- ‿ -)"
	}
	return "Content 😐", "(^ ‿ ^)"
}

// armLine draws the body: arms and outfit, which bulge as Puchi gains weight.
func armLine(p *pet.Pet) string {
	switch p.Activity() {
	case pet.Toilet:
		return " [ 🧻 ] "
	case pet.Dancing, pet.Bathing, pet.Brushing, pet.Playing, pet.Petted, pet.Eating:
		return ""
	}
	w, c := p.Weight(), clothArt[p.Cloth]
	if p.Loaf {
		switch w {
		case 0:
			return "[===========]"
		case 1:
			return "[(===========)]"
		case 2:
			return "[((===========))]"
		}
		return "(((============)))"
	}
	switch w {
	case 0:
		return "---|" + c + "|---"
	case 1:
		return "---|(" + c + ")|---"
	case 2:
		return "---|( " + c + " )|---"
	case 3:
		return "===( " + c + " )==="
	}
	return "((( " + c + " )))"
}

// hairBlock always occupies two lines so the layout doesn't jump when the hat
// changes, and each line is centred over the face.
func hairBlock(art string, width int) string {
	lines := strings.Split(art, "\n")
	for len(lines) < 2 {
		lines = append([]string{""}, lines...)
	}
	for i, l := range lines {
		lines[i] = lipgloss.PlaceHorizontal(width, lipgloss.Center, l)
	}
	return strings.Join(lines, "\n")
}

// statLine renders "Label:     [████      ]  40%" with a bar that turns orange
// and red as the situation gets worse.
func statLine(label string, value int, higherIsWorse bool) string {
	good := value
	if higherIsWorse {
		good = 100 - value
	}
	color := lipgloss.Color("114")
	switch {
	case good < 25:
		color = "203"
	case good < 50:
		color = "214"
	}
	filled := min(10, max(0, value/10))
	bar := strings.Repeat("█", filled) + strings.Repeat(" ", 10-filled)
	return statusStyle.Render(fmt.Sprintf("%-11s[", label)) +
		lipgloss.NewStyle().Foreground(color).Render(bar) +
		statusStyle.Render(fmt.Sprintf("] %3d%%", value))
}

// alt alternates between two animation frames.
func alt(frame int, a, b string) string {
	if frame%2 == 0 {
		return a
	}
	return b
}

// pad right-pads s with spaces to the given display width (emoji-safe).
func pad(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}
