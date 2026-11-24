package zenzx

import (
	"crypto/sha256"
	"encoding/binary"
	"sync"
	"testing"
)

// Two machines in one program (T-39). Each test machine is run alone first,
// then again beside the other; a machine must end in exactly the state it
// reaches alone, whatever its neighbour does. That is what shows the package
// keeps no state that two machines share and write.

// sideMachine is one machine and what is typed into it.
type sideMachine struct {
	zx     *ZenZX
	kq     *KeyQueue
	typeAt int
	text   string
	frame  int
}

// newSideMachine builds a machine of the given model ("48k" or "128k") that
// will type text once it has run typeAt frames.
func newSideMachine(t *testing.T, model string, typeAt int, text string) *sideMachine {
	t.Helper()
	zx := NewZenZX(AudioBackendOto)
	zx.cpu.Reset()
	rom := func(name string) []byte {
		data, err := resolveROMBytes(name, "./rom")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return data
	}
	var err error
	switch model {
	case "48k":
		err = zx.LoadROMBytes(rom("48.rom"))
	case "128k":
		err = zx.Load128KROMBytes(rom("128-0.rom"), rom("128-1.rom"))
	default:
		t.Fatalf("no such test model %q", model)
	}
	if err != nil {
		t.Fatalf("%s ROM: %v", model, err)
	}
	return &sideMachine{zx: zx, kq: NewKeyQueue(10, 5), typeAt: typeAt, text: text}
}

// step runs one frame, typing when the time comes.
func (m *sideMachine) step() {
	if m.frame == m.typeAt {
		m.kq.EnqueueText(m.text)
	}
	if m.kq.Active() {
		m.kq.Step(m.zx)
	}
	m.zx.RunFrame()
	m.frame++
}

// digest is the machine's state: its RAM, its registers and its picture.
func (m *sideMachine) digest() [32]byte {
	h := sha256.New()
	for b := range m.zx.memory.ram {
		h.Write(m.zx.memory.ram[b][:])
	}
	cpu := m.zx.cpu
	binary.Write(h, binary.LittleEndian, []uint16{cpu.PC, cpu.SP, cpu.AF(), cpu.BC(), cpu.DE(), cpu.HL(), cpu.IX(), cpu.IY()})
	h.Write(m.zx.DecodeDisplay().Pix)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

const sideFrames = 400

// The two machines differ in model, in what is typed and in when.
func sideA(t *testing.T) *sideMachine { return newSideMachine(t, "48k", 150, `LOAD ""`) }
func sideB(t *testing.T) *sideMachine { return newSideMachine(t, "128k", 220, "\n") }

func runAlone(m *sideMachine) [32]byte {
	for i := 0; i < sideFrames; i++ {
		m.step()
	}
	return m.digest()
}

func TestTwoMachinesTakeTurns(t *testing.T) {
	aloneA, aloneB := runAlone(sideA(t)), runAlone(sideB(t))
	if aloneA == aloneB {
		t.Fatal("the two test machines end in the same state: the test would show nothing")
	}
	if again := runAlone(sideA(t)); again != aloneA {
		t.Fatal("a machine run alone twice ends in two different states")
	}

	a, b := sideA(t), sideB(t)
	for i := 0; i < sideFrames; i++ {
		a.step()
		b.step()
	}
	if a.digest() != aloneA {
		t.Error("the 48K machine, run in turns with a 128K one, did not end as it does alone")
	}
	if b.digest() != aloneB {
		t.Error("the 128K machine, run in turns with a 48K one, did not end as it does alone")
	}
}

func TestTwoMachinesAtOnce(t *testing.T) {
	aloneA, aloneB := runAlone(sideA(t)), runAlone(sideB(t))

	a, b := sideA(t), sideB(t)
	var wg sync.WaitGroup
	var gotA, gotB [32]byte
	wg.Add(2)
	go func() { defer wg.Done(); gotA = runAlone(a) }()
	go func() { defer wg.Done(); gotB = runAlone(b) }()
	wg.Wait()
	if gotA != aloneA {
		t.Error("the 48K machine, run at the same time as a 128K one, did not end as it does alone")
	}
	if gotB != aloneB {
		t.Error("the 128K machine, run at the same time as a 48K one, did not end as it does alone")
	}
}
