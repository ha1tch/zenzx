# Wave plan

**Wave 1 — trace harness generalization (≈ 8.0d, added 2026-08-28).** Ten items generalizing zenzx's trace harness (zzz_trace_harness_test.go) beyond its current single scenario -- boot 48K, load one tape, type LOAD"", trace what happens after -- into something that can trace arbitrary code, memory reads, port writes, call structure, and symbolic addresses, with a way to capture the paused state for interactive inspection. All ten priced Low individually (2026-08-28); item 4 depends on docs/proposals/zen80-tracing-hooks.md landing in a tagged zen80 release, everything else has no external dependency. Full detail, dependencies, and Done-when criteria per item: docs/TRACE_GENERALIZATION_DEVELOPMENT_PLAN.md. Tracked under T-26.


**Wave 2 — advanced tracing features (priced, unscheduled) (≈ 35.0d, added 2026-08-28).** Six tracing features priced 2026-08-28 but not scheduled -- real costs, not rejections. Ranges from Moderate (multi-window tracing, conditional-breakpoint expressions, ring-buffer crash logs) through High (a standalone disassembler, zendis -- inverting zenas's mnemonic-keyed encoder doesn't work, checked directly) to Very High (a live/interactive debugger, reverse/step-back execution) -- the last two are a different order of magnitude from wave 1: closer to a new tool than an extension of this one. None currently has a filed register item. Full pricing and reasoning per item: docs/TRACE_GENERALIZATION_DEVELOPMENT_PLAN.md, Backlog section.


**Wave 3 — ti0.r0 — Z80N CPU support (adjustments) (≈ 0.5d, added 2026-09-14).** The Z80N opcode table itself (zen80, all 26 opcodes) is complete and cross-checked against ZEsarUX -- see Z80N_ZESARUX_CROSSCHECK.md upstream. This wave exists to hold whatever adjustment work surfaces once Z80N is exercised as part of a real Next machine type rather than a standalone CPU-only flag; no concrete item is filed yet.


**Wave 4 — ti0.r1 — NextREG port handler (≈ 1.0d, added 2026-09-14).** Backbone for every other Tier 0 Next feature -- MMU, Layer 2, sprites and palette are all driven through NextREG, so this is sequenced first among the new work.


**Wave 5 — ti0.r2 — Extended MMU (≈ 4.0d, added 2026-09-14).** The most structurally invasive Tier 0 item -- depends on ti0.r1's NextREG port handler for the MMU slot registers (NR 0x50-0x57), so sequenced right after it and before anything (Layer 2, sprites) that assumes Next-sized addressable RAM.


**Wave 6 — ti0.r3 — Layer 2 graphics (≈ 3.0d, added 2026-09-14).** The single most-used Next-only graphics feature in real software. Depends on ti0.r1 (NextREG) and ti0.r2 (MMU, since Layer 2's framebuffer lives outside the classic 128K address space).


**Wave 7 — ti0.r4 — Hardware sprites (≈ 4.0d, added 2026-09-14).** The other half of the standard Next graphics pairing -- sprite-over-Layer-2 is the textbook Next game architecture (see the SpecBong tutorial). Sequenced alongside/after ti0.r3 since both land in the same compositor work.


**Wave 8 — ti0.r5 — Next palette (≈ 1.5d, added 2026-09-14).** Needed for ti0.r3/ti0.r4 to display correct colours, not just correct pixel data. Low risk to existing code since pkg/zxpalette already separates palette concerns from rendering.


**Wave 9 — ti0.r6 — NEX file loading (≈ 2.0d, added 2026-09-14).** Last of the Tier 0 set -- depends on ti0.r2 (MMU, for paging in the loaded banks) and ti0.r3 (Layer 2, since NEX's own screen header targets it directly). This is what actually lets a real Next game run end to end once the other six land.

