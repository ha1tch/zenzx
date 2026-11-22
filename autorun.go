package zenzx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ha1tch/zentools/pkg/basic"
	"github.com/ha1tch/zentools/pkg/tap"
)

// Autorun: a BASIC program in a text file, loaded and started with no one
// at the keyboard.
//
// The text is tokenised (zentools' tokeniser), wrapped as a tape holding
// one program that starts itself at its first line, and put in the
// machine's tape deck. When the machine has finished starting up, the keys
// that load from tape are pressed for it: LOAD "" and ENTER on a 48K, ENTER
// on the Tape Loader line of a 128K's menu. The tape is in fast mode, so
// the program is running a moment later.

// autorunGiveUp is how many frames a machine is given to finish starting up
// before the autorun is abandoned.
const autorunGiveUp = 50 * 60

// BasicTape tokenises BASIC source text and returns a .tap image holding it
// as one program, named name, that runs from its first line when loaded.
func BasicTape(name, source string) ([]byte, error) {
	prog, err := basic.Tokenise(source)
	if err != nil {
		return nil, err
	}
	if len(prog) < 4 {
		return nil, errors.New("no BASIC lines: each line must begin with a line number")
	}
	first := uint16(prog[0])<<8 | uint16(prog[1])
	return tap.EncodeProgram(name, prog, first), nil
}

// autorunKeys is what is typed to load from tape once the machine has
// started, by model; a model not listed cannot autorun.
var autorunKeys = map[string]string{
	"48k":          "LOAD \"\"\n",
	"spanish48k":   "LOAD \"\"\n",
	"128k":         "\n",
	"plus2":        "\n",
	"plus2a":       "\n",
	"plus3":        "\n",
	"spanish128k":  "\n",
	"spanishplus2": "\n",
	"spanishplus3": "\n",
}

// autorun is one machine's pending autorun.
type autorun struct {
	model  string
	label  string
	boot   *BootDetector
	keys   *KeyQueue
	frames int
}

// AutorunBASIC arranges for BASIC source text to be loaded and run on this
// machine once it has started. model is the machine's model as -model
// names it; name becomes the program's name on the tape (ten characters at
// most are kept).
func (zx *ZenZX) AutorunBASIC(name, source, model string) error {
	keys, ok := autorunKeys[model]
	if !ok {
		return fmt.Errorf("autorun is not available on model %q", model)
	}
	if zx.tape == nil {
		return errors.New("autorun: this machine has no tape deck")
	}
	image, err := BasicTape(name, source)
	if err != nil {
		return fmt.Errorf("autorun: %s: %w", name, err)
	}
	return zx.autorunTape(name, image, model, keys)
}

// AutorunTAP arranges for a prebuilt tape image (a .tap/.tzx that already
// carries its own loader, such as an assembled machine-code program) to be
// loaded and run once the machine has started. It types the same LOAD ""
// the BASIC path does; the tape's own loader takes it from there.
func (zx *ZenZX) AutorunTAP(name string, image []byte, model string) error {
	keys, ok := autorunKeys[model]
	if !ok {
		return fmt.Errorf("autorun is not available on model %q", model)
	}
	if zx.tape == nil {
		return errors.New("autorun: this machine has no tape deck")
	}
	return zx.autorunTape(name, image, model, keys)
}

// autorunTape loads a tape image into the deck and queues the boot-and-LOAD
// keystrokes. It is the shared tail of AutorunBASIC and AutorunTAP.
func (zx *ZenZX) autorunTape(name string, image []byte, model, keys string) error {
	if err := zx.tape.loadTAP(image); err != nil {
		return fmt.Errorf("autorun: %s: %w", name, err)
	}
	zx.tape.st.Loaded = true
	zx.tape.st.Filename = name
	zx.tape.st.Position = 0
	zx.tape.st.Playing = false
	zx.tape.SetMode(TapeFast)

	rainbow := false
	if b, ok := bootBaselines[model]; ok {
		rainbow = b.rainbow
	}
	q := NewKeyQueue(10, 5)
	q.EnqueueText(keys)
	zx.auto = &autorun{model: model, label: name, boot: NewBootDetector(rainbow), keys: q}
	return nil
}

// AutorunFile is AutorunBASIC for a text file; the program takes the file's
// name without its extension.
func (zx *ZenZX) AutorunFile(path, model string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("autorun: %w", err)
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	switch strings.ToLower(filepath.Ext(path)) {
	case ".tap", ".tzx":
		return zx.AutorunTAP(name, data, model)
	default:
		return zx.AutorunBASIC(name, string(data), model)
	}
}

// AutorunPending reports whether an autorun has yet to finish typing.
func (zx *ZenZX) AutorunPending() bool { return zx.auto != nil }

// autorunStep moves a pending autorun on by one frame. RunFrame calls it
// first, which in the GUI is after the host's keyboard has been read, so
// the keys pressed here are the ones the machine sees.
func (zx *ZenZX) autorunStep() {
	a := zx.auto
	if !a.boot.Ready() {
		a.boot.Update(zx)
		a.frames++
		if a.frames > autorunGiveUp {
			sayf("Autorun: %s did not finish starting up; %s was not loaded\n", a.model, a.label)
			zx.auto = nil
			return
		}
		if !a.boot.Ready() {
			return
		}
		// A reset stops the tape; start it from the top now.
		zx.tape.Rewind()
		zx.tape.Play()
	}
	if !a.keys.Step(zx) {
		zx.io.ResetKeyboard()
		zx.auto = nil
	}
}
