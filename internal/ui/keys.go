package ui

// section is which pane of the app is showing. The redesign splits the old
// single "Downloads" pane into Downloads (in progress) + Seeding (complete) and
// adds a Settings pane; tab cycles through all four.
type section int

const (
	sectionSearch section = iota
	sectionDownloads
	sectionSeeding
	sectionSettings
)

func (s section) String() string {
	switch s {
	case sectionDownloads:
		return "Downloads"
	case sectionSeeding:
		return "Seeding"
	case sectionSettings:
		return "Settings"
	default:
		return "Search"
	}
}

// next returns the section tab should move to (Search → Downloads → Seeding →
// Settings → Search).
func (s section) next() section {
	return (s + 1) % 4
}

// Key handling stays simple and conflict-free. When the search box is focused
// (m.editing) text keys go to the input and only Enter/Esc/Ctrl+C are
// intercepted; when a Settings text field is focused (m.editingSetting) the same
// holds for that field. Otherwise single keys are commands:
//
//	/        focus the search box
//	enter    (search box) run search; (results) download; (settings) edit a field
//	esc      leave the search box / cancel a settings edit
//	↑ / ↓    move the selection (results, or settings rows)
//	← / →    change the media filter (search) or a setting's value (settings)
//	d        download the selected result
//	tab      cycle Search · Downloads · Seeding · Settings
//	?        toggle help
//	q        quit (when not typing)
//	ctrl+c   quit (always)
//
// New since the original: the Seeding and Settings sections, ← / → (filter +
// settings value), and tab cycling four panes instead of toggling two. These
// match bubbletea's KeyMsg.String() values.
