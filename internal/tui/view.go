package tui

import (
	"fmt"
	"strings"

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

func (k kind) style() lipgloss.Style {
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
// line and the help bar.
func (m Model) listHeight() int {
	used := 1 + 2 + (detailLines + 2) + lipgloss.Height(m.helpView())
	if m.showFilterLine() {
		used++
	}
	return max(m.height-used, 1)
}

func (m Model) showFilterLine() bool {
	return m.filtering || m.filter.Value() != ""
}

func (m Model) helpView() string {
	if m.filtering {
		return m.help.View(filterKeys{m.keys})
	}
	return m.help.View(m.keys)
}

// render assembles the screen top to bottom.
func (m Model) render() string {
	parts := []string{m.header(), m.list(), m.details()}
	if m.showFilterLine() {
		parts = append(parts, m.filter.View())
	}
	parts = append(parts, m.helpView())
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// header: name and cloud folder on the left, a count per status on the right. The counts
// win when space is short; the folder is shortened first.
func (m Model) header() string {
	// One count per icon: "only in $HOME" and "local + cloud" are both ●
	counts := map[string]int{}
	for _, e := range m.entries {
		counts[e.icon()]++
	}
	var summary []string
	for _, k := range []kind{kindLinked, kindNotLinked, kindLocalOnly, kindBadLink, kindNotInCloud, kindOther} {
		icon := (entry{kind: k}).icon()
		if counts[icon] > 0 {
			summary = append(summary, k.style().Render(fmt.Sprintf("%s %d", icon, counts[icon])))
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
			itemWidth = max(itemWidth, lipgloss.Width(m.entries[idx].item))
			labelWidth = max(labelWidth, lipgloss.Width(m.entries[idx].label()))
		}
		itemWidth = min(itemWidth, inner/2)

		for i := m.offset; i < len(m.visible) && i < m.offset+rows; i++ {
			e := m.entries[m.visible[i]]
			marker := "  "
			item := truncate(e.item, itemWidth)
			if i == m.cursor {
				marker = cursorStyle.Render("▸ ")
				item = lipgloss.NewStyle().Bold(true).Render(item)
			}
			line := marker + e.kind.style().Render(e.icon()) + "  " + item +
				strings.Repeat(" ", itemWidth-lipgloss.Width(truncate(e.item, itemWidth))+2) +
				e.kind.style().Render(e.label())
			if hint := e.hint(); hint != "" {
				line += strings.Repeat(" ", labelWidth-lipgloss.Width(e.label())+2) + hintStyle.Render("→ "+hint)
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
	if e, ok := m.selected(); ok {
		var blocks [][]string
		for _, d := range e.details() {
			style := lipgloss.NewStyle()
			if strings.HasPrefix(d, "Run: ") {
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
		lines = append(lines, lipgloss.NewStyle().Bold(true).Render(e.item)+"  "+e.kind.style().Render(e.label()))
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
