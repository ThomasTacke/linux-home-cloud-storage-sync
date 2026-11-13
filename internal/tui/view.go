package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Colors from the s3r3nity-labs icon.
var (
	magenta = lipgloss.Color("#c22d6f")
	green   = lipgloss.Color("#41a35b")
	orange  = lipgloss.Color("#e8814a")
	red     = lipgloss.Color("#e05252")
	gray    = lipgloss.Color("#8a9091")
)

// Styles are values: each call returns a changed copy, so they can be shared freely.
var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(magenta)
	faintStyle  = lipgloss.NewStyle().Foreground(gray)
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(magenta)
	hintStyle   = lipgloss.NewStyle().Foreground(orange)
	boxStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(magenta).Padding(0, 1)
	detailStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(gray).Padding(0, 1)
)

// detailLines is the fixed inner height of the details box, so the list doesn't jump
// when the selection changes.
const detailLines = 5

func kindStyle(k kind) lipgloss.Style {
	s := lipgloss.NewStyle()
	switch k {
	case kindLinked:
		return s.Foreground(green)
	case kindNotLinked, kindLocalOnly, kindBothExist:
		return s.Foreground(orange)
	case kindBadLink, kindOther:
		return s.Foreground(red)
	default:
		return s.Foreground(gray)
	}
}

// listHeight is how many entry rows fit: the screen minus header, both boxes, the filter
// line, the status line and the help bar.
func (m Model) listHeight() int {
	used := 1 + 2 + (detailLines + 2) + lipgloss.Height(m.helpView())
	if m.showFilterLine() {
		used++
	}
	if m.status != "" {
		used++
	}
	return max(m.height-used, 1)
}

func (m Model) showFilterLine() bool {
	return m.mode == modeFilter || m.filter.Value() != ""
}

// helpView is the help bar for whatever has the keys right now.
func (m Model) helpView() string {
	switch m.mode {
	case modeFilter:
		return m.help.View(filterKeys{m.keys})
	case modeConfirm:
		return m.help.View(confirmKeys{m.keys})
	case modeResult:
		return m.help.View(staticKeys{key.NewBinding(key.WithKeys("any"), key.WithHelp("any key", "close"))})
	case modeBusy:
		return faintStyle.Render("working…")
	case modePicker:
		return m.help.View(staticKeys{
			m.picker.KeyMap.Up, m.picker.KeyMap.Down, m.picker.KeyMap.Open, m.picker.KeyMap.Back,
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "add")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		})
	}
	return m.help.View(m.keys)
}

// render assembles the screen top to bottom, with a dialog on top if one is open.
func (m Model) render() string {
	parts := []string{m.header(), m.list(), m.details()}
	if m.showFilterLine() {
		parts = append(parts, m.filter.View())
	}
	if m.status != "" {
		parts = append(parts, hintStyle.Render(truncate(m.status, m.width)))
	}
	parts = append(parts, m.helpView())
	screen := lipgloss.JoinVertical(lipgloss.Left, parts...)

	var dialog string
	switch m.mode {
	case modeConfirm, modeBusy:
		dialog = m.confirmDialog()
	case modeResult:
		dialog = m.resultDialog()
	case modePicker:
		dialog = m.pickerDialog()
	default:
		return screen
	}
	// Layers: the screen at the bottom, the dialog centered above it (Z 1)
	x := max((m.width-lipgloss.Width(dialog))/2, 0)
	y := max((m.height-lipgloss.Height(dialog))/2, 0)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(screen),
		lipgloss.NewLayer(dialog).X(x).Y(y).Z(1),
	).Render()
}

// dialogWidth fits dialogs into small terminals and keeps them readable in large ones.
func (m Model) dialogWidth() int {
	return min(m.width-4, 84)
}

// dialogBox frames a dialog: title, wrapped text, then the step lines, cut to fit.
func (m Model) dialogBox(title, text string, lines []string) string {
	inner := m.dialogWidth() - 4
	room := max(m.height-8, 3) // keep the help bar visible
	body := []string{titleStyle.Render(title)}
	if text != "" {
		body = append(body, "", lipgloss.NewStyle().Width(inner).Render(text))
	}
	if len(lines) > 0 {
		body = append(body, "")
	}
	free := room - lipgloss.Height(strings.Join(body, "\n"))
	if len(lines) > free {
		more := len(lines) - (free - 1)
		lines = append(lines[:max(free-1, 0)], faintStyle.Render(fmt.Sprintf("… and %d more", more)))
	}
	for _, l := range lines {
		body = append(body, truncate(l, inner))
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(magenta).
		Padding(0, 1).
		Width(m.dialogWidth()).
		Render(strings.Join(body, "\n"))
}

// stepLines: one line per entry, item column aligned.
func stepLines(a action, steps []step, done bool) []string {
	width := 0
	for _, s := range steps {
		width = max(width, lipgloss.Width(s.Item))
	}
	var lines []string
	for _, s := range steps {
		icon, style := "→", hintStyle
		switch {
		case done && s.Does:
			icon, style = "✓", lipgloss.NewStyle().Foreground(green)
		case done:
			icon, style = "✗", lipgloss.NewStyle().Foreground(red)
		case !s.Does:
			icon, style = "·", faintStyle
		}
		pad := strings.Repeat(" ", width-lipgloss.Width(s.Item)+2)
		lines = append(lines, style.Render(icon)+" "+s.Item+pad+style.Render(describe(a, s)))
	}
	return lines
}

func (m Model) confirmDialog() string {
	p := m.plan
	n := p.Count()
	title := fmt.Sprintf("%s %d %s?", actionTitle(p.Action), n, plural(n, "entry", "entries"))
	if m.mode == modeBusy {
		title = actionTitle(p.Action) + " …"
	}
	text := actionExplain(p.Action)
	if skipped := len(p.Steps) - n; skipped > 0 {
		text += fmt.Sprintf(" %d %s nothing to do and %s skipped.", skipped, plural(skipped, "has", "have"), plural(skipped, "is", "are"))
	}
	return m.dialogBox(title, text, stepLines(p.Action, p.Steps, false))
}

func (m Model) resultDialog() string {
	d := m.done
	if d.Err != nil {
		return m.dialogBox(actionTitle(d.Action)+" failed", d.Err.Error(), nil)
	}
	return m.dialogBox(actionTitle(d.Action)+": done", "", stepLines(d.Action, d.Steps, true))
}

func (m Model) pickerDialog() string {
	text := "Choose a file or folder in " + short(m.cfg.LocalHomeDir) + ". → opens a folder, enter adds the highlighted one."
	return m.dialogBox("New entry  "+faintStyle.Render(short(m.picker.CurrentDirectory)), text,
		strings.Split(strings.TrimRight(m.picker.View(), "\n"), "\n"))
}

// pickerHeight is how many files the picker lists inside its dialog.
func (m Model) pickerHeight() int {
	return max(m.height-16, 3)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// header: name and cloud folder on the left, a count per status on the right. The counts
// win when space is short; the folder is shortened first.
func (m Model) header() string {
	// One count per icon: "only in $HOME" and "local + cloud" are both ●
	counts := map[string]int{}
	for _, e := range m.entries {
		counts[icon(e)]++
	}
	var summary []string
	for _, k := range []kind{kindLinked, kindNotLinked, kindLocalOnly, kindBadLink, kindNotInCloud, kindOther} {
		icon := icon(entry{Kind: k})
		if counts[icon] > 0 {
			summary = append(summary, kindStyle(k).Render(fmt.Sprintf("%s %d", icon, counts[icon])))
		}
	}
	right := strings.Join(summary, "  ")

	const name = "linux-home-cloud-storage-sync"
	info := short(m.cfg.CloudStorageHomeDir)
	if m.note != "" {
		info += "  (" + m.note + ")"
	}
	room := m.width - lipgloss.Width(right) - lipgloss.Width(name) - 4 // two gaps of two
	left := titleStyle.Render(name)
	if room > 1 {
		left += "  " + faintStyle.Render(truncate(info, room))
	}

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

// list: one row per visible entry, only the rows that fit, starting at offset.
func (m Model) list() string {
	inner := m.width - 4 // border and padding on both sides
	rows := m.listHeight()

	var lines []string
	switch {
	case len(m.entries) == 0:
		lines = append(lines, faintStyle.Render("symlinkMap is empty: add entries to the config."))
	case len(m.visible) == 0:
		lines = append(lines, faintStyle.Render("No entry matches the filter."))
	default:
		itemWidth, labelWidth := 0, 0
		for _, idx := range m.visible {
			itemWidth = max(itemWidth, lipgloss.Width(m.entries[idx].Item))
			labelWidth = max(labelWidth, lipgloss.Width(label(m.entries[idx])))
		}
		itemWidth = min(itemWidth, inner/2)

		for i := m.offset; i < len(m.visible) && i < m.offset+rows; i++ {
			e := m.entries[m.visible[i]]
			marker := "  "
			item := truncate(e.Item, itemWidth)
			if i == m.cursor {
				marker = cursorStyle.Render("▸ ")
				item = lipgloss.NewStyle().Bold(true).Render(item)
			}
			if m.selected[e.Item] {
				marker += cursorStyle.Render("◆ ")
			} else {
				marker += "  "
			}
			line := marker + kindStyle(e.Kind).Render(icon(e)) + "  " + item +
				strings.Repeat(" ", itemWidth-lipgloss.Width(truncate(e.Item, itemWidth))+2) +
				kindStyle(e.Kind).Render(label(e))
			if hint := hint(e); hint != "" {
				line += strings.Repeat(" ", labelWidth-lipgloss.Width(label(e))+2) + hintStyle.Render("→ "+hint)
			}
			lines = append(lines, truncate(line, inner))
		}
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	return boxStyle.Width(m.width).Render(strings.Join(lines, "\n"))
}

// details: what's going on with the selected entry, wrapped to the box width. If it
// doesn't fit, the explanation is cut, never the last line (the command to run).
func (m Model) details() string {
	inner := m.width - 4
	var lines []string
	if e, ok := m.current(); ok {
		var blocks [][]string
		for _, d := range details(e) {
			style := lipgloss.NewStyle()
			if strings.HasPrefix(d, "Key: ") {
				style = faintStyle
			}
			blocks = append(blocks, strings.Split(style.Width(inner).Render(d), "\n"))
		}
		last := blocks[len(blocks)-1]
		room := detailLines - 1 - len(last) // minus the title line
		var body []string
		for _, b := range blocks[:len(blocks)-1] {
			body = append(body, b...)
		}
		if len(body) > room {
			body = body[:max(room, 0)]
			if room > 0 {
				body[room-1] = truncate(ansi.Strip(body[room-1]), inner-1) + "…"
			}
		}
		lines = append(lines, lipgloss.NewStyle().Bold(true).Render(e.Item)+"  "+kindStyle(e.Kind).Render(label(e)))
		lines = append(lines, body...)
		lines = append(lines, last...)
	}
	if len(lines) > detailLines {
		lines = lines[:detailLines]
	}
	for len(lines) < detailLines {
		lines = append(lines, "")
	}
	return detailStyle.Width(m.width).Render(strings.Join(lines, "\n"))
}

// truncate shortens s to width cells, with "…" at the end.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width-1).Render(s) + "…"
}
