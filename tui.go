package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mattn/go-runewidth"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type logLineMsg struct {
	text string
}

type addItemMsg struct {
	num  int
	text string
}

type promptMsg struct {
	label string
}

type procInfoMsg struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
}

type procExitMsg struct {
	err error
}

type tickMsg struct{}

const spinnerFrames = "⣾⣽⣻⢿⡿⣟⣯⣷"

func tickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

type menuItem struct {
	num  int
	text string
}

type model struct {
	bin       string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	items     []menuItem
	selected  int
	input     string
	inputMode bool
	inputLabel string
	log       []string
	status    string
	exitInfo  string
	width     int
	height    int
	spinner   int
}

type app struct {
	m model
	p *tea.Program
}

func (a *app) Init() tea.Cmd {
	return a.startCmd()
}

func (a *app) startCmd() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command(a.m.bin)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return procExitMsg{err}
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return procExitMsg{err}
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return procExitMsg{err}
		}
		if err := cmd.Start(); err != nil {
			return procExitMsg{err}
		}
		go a.stream(stdout)
		go a.stream(stderr)
		go func() {
			a.p.Send(procExitMsg{err: cmd.Wait()})
		}()
		return procInfoMsg{cmd: cmd, stdin: stdin}
	}
}

func (a *app) stream(r io.Reader) {
	buf := make([]byte, 4096)
	acc := make([]byte, 0, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			acc = a.consume(acc)
		}
		if err != nil {
			if len(acc) > 0 {
				a.consume(acc)
			}
			return
		}
	}
}

func (a *app) consume(acc []byte) []byte {
	for {
		idx := bytes.IndexByte(acc, '\n')
		if idx < 0 {
			break
		}
		line := string(acc[:idx])
		acc = acc[idx+1:]
		a.handleLine(line)
	}
	if len(acc) > 0 && bytes.Contains(acc, []byte("-->")) {
		a.handleLine(string(acc))
		acc = nil
	}
	return acc
}

var itemRe = regexp.MustCompile(`^\[(\d+)\]\s*(.*)$`)

func (a *app) handleLine(line string) {
	if strings.Contains(line, "-->") {
		label := strings.TrimSpace(strings.SplitN(line, "-->", 2)[0])
		if label == "" {
			label = "Input"
		}
		a.p.Send(promptMsg{label: label})
		return
	}
	if mm := itemRe.FindStringSubmatch(line); mm != nil {
		num, _ := strconv.Atoi(mm[1])
		a.p.Send(addItemMsg{num: num, text: strings.TrimSpace(mm[2])})
		return
	}
	a.p.Send(logLineMsg{text: line})
}

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m := a.m
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		return a.handleKey(m, msg)
	case logLineMsg:
		m.log = append(m.log, msg.text)
		if len(m.log) > 1000 {
			m.log = m.log[len(m.log)-1000:]
		}
	case addItemMsg:
		idx := -1
		for i, it := range m.items {
			if it.num == msg.num {
				idx = i
				break
			}
		}
		if idx >= 0 {
			m.items[idx].text = msg.text
		} else {
			m.items = append(m.items, menuItem{num: msg.num, text: msg.text})
			if m.selected < 0 {
				m.selected = 0
			}
		}
	case promptMsg:
		m.inputMode = true
		m.input = ""
		m.inputLabel = msg.label
		m.status = "awaiting input"
	case procInfoMsg:
		m.cmd = msg.cmd
		m.stdin = msg.stdin
		m.status = "running"
	case procExitMsg:
		m.inputMode = false
		m.input = ""
		m.stdin = nil
		if msg.err != nil {
			m.exitInfo = msg.err.Error()
		} else {
			m.exitInfo = "exit 0"
		}
		m.status = "process exited"
	case tickMsg:
		m.spinner++
	}
	a.m = m
	if m.status == "running" {
		return a, tickCmd()
	}
	return a, nil
}

func (a *app) handleKey(m model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		a.kill()
		return a, tea.Quit
	}
	if m.inputMode {
		switch msg.String() {
		case "enter":
			text := m.input
			if text == "" && m.selected >= 0 {
				text = strconv.Itoa(m.items[m.selected].num)
			}
			m.inputMode = false
			m.input = ""
			if m.stdin != nil {
				fmt.Fprintf(m.stdin, "%s\n", text)
				m.status = "running"
			}
		case "backspace":
			r := []rune(m.input)
			if len(r) > 0 {
				m.input = string(r[:len(r)-1])
			}
		case "esc":
			m.inputMode = false
			m.input = ""
			if m.stdin != nil {
				fmt.Fprintln(m.stdin)
				m.status = "input cancelled"
			}
		case "up":
			m = m.moveSel(-1)
		case "down":
			m = m.moveSel(1)
		default:
			switch {
			case msg.Type == tea.KeyRunes:
				for _, r := range msg.Runes {
					if unicode.IsDigit(r) {
						m = m.selectItem(int(r - '0'))
					}
					m.input += string(r)
				}
			case msg.String() == " ":
				m.input += " "
			}
		}
		a.m = m
		if m.status == "running" {
			return a, tickCmd()
		}
		return a, nil
	}
	switch msg.String() {
	case "q":
		a.kill()
		return a, tea.Quit
	case "r":
		if m.stdin == nil {
			m.items = nil
			m.selected = -1
			m.inputMode = false
			m.input = ""
			m.exitInfo = ""
			m.status = "restarting"
			a.m = m
			return a, a.startCmd()
		}
	case "up":
		m = m.moveSel(-1)
	case "down":
		m = m.moveSel(1)
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m = m.selectItem(int(msg.String()[0] - '0'))
	}
	a.m = m
	if m.status == "running" {
		return a, tickCmd()
	}
	return a, nil
}

func (a *app) kill() {
	m := a.m
	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
	}
}

func (m model) moveSel(d int) model {
	if len(m.items) == 0 {
		return m
	}
	if m.selected < 0 {
		m.selected = 0
	} else {
		m.selected = (m.selected + d + len(m.items)) % len(m.items)
	}
	return m
}

func (m model) selectItem(num int) model {
	for i, it := range m.items {
		if it.num == num {
			m.selected = i
			break
		}
	}
	return m
}

func (a *app) View() string {
	m := a.m
	w := m.width
	if w <= 0 {
		w = 80
	}
	h := m.height
	if h <= 0 {
		h = 24
	}

	header := lipgloss.NewStyle().
		Padding(0, 1).
		Render(lipgloss.JoinHorizontal(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("240")).Padding(0, 1).Render("BGIT"),
			" ",
			lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(headerStatus(m)),
		))
	rule := lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Render(strings.Repeat("─", w))

	menu := a.menuView(w)

	parts := []string{header, rule, "", menu, ""}

	if m.inputMode {
		parts = append(parts, promptRow(m, w), "")
	}

	parts = append(parts, a.logView(outLinesFor(m, h), w))
	parts = append(parts, "", footerText(m))
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func outLinesFor(m model, h int) int {
	rows := len(m.items)
	if rows == 0 {
		rows = 1
	}
	used := 1 + 1 + 1 + 1 + rows + 1 + 1 + 1 + 1
	if m.inputMode {
		used += 2
	}
	n := h - used
	if n < 2 {
		n = 2
	}
	return n
}

func headerStatus(m model) string {
	switch m.status {
	case "running":
		f := string(spinnerFrames[m.spinner%len(spinnerFrames)])
		return fmt.Sprintf("running %s · %s", f, m.bin)
	case "awaiting input":
		return "awaiting input · " + m.bin
	case "process exited":
		return fmt.Sprintf("exited (%s) · %s", m.exitInfo, m.bin)
	default:
		return m.status + " · " + m.bin
	}
}

func (a *app) menuView(w int) string {
	if len(a.m.items) == 0 {
		return lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("238")).
			Render("  waiting for program output…")
	}
	max := 0
	for _, it := range a.m.items {
		l := runewidth.StringWidth(fmt.Sprintf("[%d] %s", it.num, it.text))
		if l > max {
			max = l
		}
	}
	if max > w-4 {
		max = w - 4
	}
	var b strings.Builder
	for i, it := range a.m.items {
		label := fmt.Sprintf("[%d] %s", it.num, it.text)
		label = truncate(label, max)
		pad := max - runewidth.StringWidth(label)
		label += strings.Repeat(" ", pad+2)
		if i == a.m.selected {
			b.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("240")).
				Render("▸ " + label))
		} else {
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Render("  " + label))
		}
		if i < len(a.m.items)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func promptRow(m model, w int) string {
	label := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Render(m.inputLabel + ":")
	fieldW := w - runewidth.StringWidth(label) - 2
	if fieldW < 10 {
		fieldW = 10
	}
	disp := truncate(m.input+"█", fieldW)
	disp += strings.Repeat(" ", fieldW-runewidth.StringWidth(disp))
	field := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("237")).
		Render(disp)
	return lipgloss.JoinHorizontal(lipgloss.Left, label, " ", field)
}

func (a *app) logView(outLines int, w int) string {
	ruleText := "─── output"
	fill := w - runewidth.StringWidth(ruleText)
	if fill < 0 {
		fill = 0
	}
	rule := lipgloss.NewStyle().Foreground(lipgloss.Color("236")).
		Render(ruleText + strings.Repeat("─", fill))

	var b strings.Builder
	b.WriteString(rule + "\n")
	if len(a.m.log) == 0 {
		b.WriteString(lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("238")).
			Render("  (no output yet)"))
	} else {
		start := len(a.m.log) - outLines
		if start < 0 {
			start = 0
		}
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		for i := start; i < len(a.m.log); i++ {
			b.WriteString("  " + dim.Render(truncate(a.m.log[i], w-2)) + "\n")
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func footerText(m model) string {
	var t string
	switch {
	case m.inputMode:
		t = "enter send · ↑/↓ pick · 1-9 select · esc cancel"
	case m.stdin == nil:
		t = "r restart · q quit"
	default:
		t = "↑/↓ pick · 1-9 select · q quit"
	}
	return lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("238")).Render("  " + t)
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 3 {
		return string(r[:w])
	}
	return string(r[:w-3]) + "..."
}

func main() {
	bin := os.Getenv("BGIT_BIN")
	if bin == "" {
		bin = "./bgit"
	}
	a := &app{m: model{bin: bin, selected: -1}}
	p := tea.NewProgram(a, tea.WithAltScreen())
	a.p = p
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
