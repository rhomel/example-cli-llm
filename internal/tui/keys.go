package tui

type Key int

const (
	KeyUnknown Key = iota
	KeyEnter
	KeyCtrlC
	KeyEsc
	KeyUp
	KeyDown
	KeyJ
	KeyK
)

func ParseKey(buf []byte) Key {
	switch {
	case len(buf) == 0:
		return KeyUnknown
	case buf[0] == '\r' || buf[0] == '\n':
		return KeyEnter
	case buf[0] == 3:
		return KeyCtrlC
	case buf[0] == 'j':
		return KeyJ
	case buf[0] == 'k':
		return KeyK
	case len(buf) >= 3 && buf[0] == 27 && buf[1] == 91 && buf[2] == 65:
		return KeyUp
	case len(buf) >= 3 && buf[0] == 27 && buf[1] == 91 && buf[2] == 66:
		return KeyDown
	case buf[0] == 27:
		return KeyEsc
	default:
		return KeyUnknown
	}
}
