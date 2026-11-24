package zenzx

import "testing"

// docs/RESOLVED.md T-78: the +3's lock latch belongs to port 0x7FFD and is set in every paging
// mode, special (all-RAM) paging included. The write that sets it still takes effect; the
// writes after it, to either port, are refused.
func TestPlus3LockHoldsInSpecialPaging(t *testing.T) {
	zx, err := newMachine("plus3", "./rom")
	if err != nil {
		t.Skip(err)
	}
	m, io := zx.memory, zx.io

	io.WritePort(0x1FFD, 0x05) // special paging, configuration 2: banks 4, 5, 6, 3
	if !m.specialPaging {
		t.Fatal("setting up: special paging is not on")
	}
	io.WritePort(0x7FFD, 0x28) // the lock (bit 5) and the shadow screen (bit 3)
	if !m.pagingLocked {
		t.Fatal("a write of 0x28 to port 0x7FFD in special paging did not lock paging")
	}
	if m.screenBank != 7 {
		t.Errorf("the write that locked did not take effect: screen bank %d, want 7", m.screenBank)
	}

	io.WritePort(0x1FFD, 0x00) // would leave special paging
	if !m.specialPaging || m.port1FFD != 0x05 {
		t.Errorf("a write to port 0x1FFD after the lock was not refused: special %v, port 0x%02X", m.specialPaging, m.port1FFD)
	}
	io.WritePort(0x7FFD, 0x00)
	if m.screenBank != 7 || m.port7FFD != 0x28 {
		t.Errorf("a write to port 0x7FFD after the lock was not refused: screen bank %d, port 0x%02X", m.screenBank, m.port7FFD)
	}
}

// What a write to port 0x7FFD sets while special paging is on is kept in its latch and is back
// in force when special paging is turned off: the RAM at the top and the ROM's low bit are the
// latch's, not what the special configuration left there.
func TestPlus3LatchComesBackWhenSpecialPagingEnds(t *testing.T) {
	zx, err := newMachine("plus3", "./rom")
	if err != nil {
		t.Skip(err)
	}
	m, io := zx.memory, zx.io

	io.WritePort(0x1FFD, 0x01) // special paging, configuration 0: banks 0, 1, 2, 3
	io.WritePort(0x7FFD, 0x14) // RAM 4 at the top, ROM low bit set, while the map is fixed
	if m.ramBankTop != 3 {
		t.Fatalf("special paging configuration 0 has bank %d at the top, want 3", m.ramBankTop)
	}
	io.WritePort(0x1FFD, 0x00) // back to normal
	if m.specialPaging {
		t.Fatal("special paging did not end")
	}
	if m.ramBankTop != 4 {
		t.Errorf("after special paging, RAM at the top is bank %d, want 4 (what port 0x7FFD held)", m.ramBankTop)
	}
	if m.romBank != 1 {
		t.Errorf("after special paging, ROM is %d, want 1 (the latch's bit 4, ROM bit 2 clear)", m.romBank)
	}
}
