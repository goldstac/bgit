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
	"unicode"

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
	}
	a.m = m
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
				m.status = "sent: " + text
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
			if msg.Type == tea.KeyRunes {
				for _, r := range msg.Runes {
					if unicode.IsDigit(r) {
						m = m.selectItem(int(r - '0'))
					}
					m.input += string(r)
				}
			}
		}
		a.m = m
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

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("63")).
		Padding(0, 2).
		Render("BGIT")
	sub := m.status
	if m.status == "process exited" {
		sub += " (" + m.exitInfo + ")"
	}
	sub = lipgloss.NewStyle().Faint(true).Render(sub + " · " + m.bin)
	header := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", sub)

	menu := m.menuView()
	menu = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1).
		Width(w - 6).
		Render(menu)

	inputRow := ""
	if m.inputMode {
		label := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			Render(m.inputLabel + ":")
		field := lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("236")).
			Padding(0, 1).
			Render(truncate(m.input+"█", w-12))
		inputRow = lipgloss.JoinHorizontal(lipgloss.Left, label, " ", field)
	}

	logH := h - 12 - len(m.items)
	if logH < 3 {
		logH = 3
	}
	logPanel := a.logView(logH, w)

	var footer string
	if m.inputMode {
		footer = "enter send · ↑/↓ pick · digits select · esc cancel · ctrl+c quit"
	} else if m.stdin == nil {
		footer = "r restart · q quit"
	} else {
		footer = "↑/↓ pick · digits select · q quit"
	}
	footer = lipgloss.NewStyle().Faint(true).Render(footer)

	parts := []string{header, "", menu}
	if inputRow != "" {
		parts = append(parts, "", inputRow)
	}
	parts = append(parts, "", logPanel, "", footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) menuView() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Render("Options") + "\n\n")
	if len(m.items) == 0 {
		b.WriteString(lipgloss.NewStyle().Faint(true).Render("waiting for program output…") + "\n")
	}
	for i, it := range m.items {
		cursor := "  "
		if i == m.selected {
			cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Render("▸")
		}
		num := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("81")).
			Render(fmt.Sprintf("[%d]", it.num))
		text := lipgloss.NewStyle().Padding(0, 1).Render(it.text)
		if i == m.selected {
			text = lipgloss.NewStyle().
				Background(lipgloss.Color("63")).
				Foreground(lipgloss.Color("15")).
				Padding(0, 1).
				Render(it.text)
		}
		b.WriteString(cursor + " " + num + " " + text + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (a *app) logView(maxLines int, w int) string {
	var b strings.Builder
	if len(a.m.log) == 0 {
		b.WriteString(lipgloss.NewStyle().Faint(true).Render("(no output yet)"))
	} else {
		start := len(a.m.log) - maxLines
		if start < 0 {
			start = 0
		}
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		for _, l := range a.m.log[start:] {
			b.WriteString(dim.Render(truncate(l, w-6)) + "\n")
		}
		b.WriteString(lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("238")).
			Render(fmt.Sprintf("(%d lines, last %d shown)", len(a.m.log), len(a.m.log)-start)))
	}
	body := strings.TrimSuffix(b.String(), "\n")
	if body == "" {
		body = " "
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).
		Width(w - 6).
		Render(body)
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
