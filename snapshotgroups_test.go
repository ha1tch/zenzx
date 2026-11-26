package zenzx

import (
	"strings"
	"testing"
	"time"
)

// stamps makes n timestamped snapshots, the first at start and each one gap before the last,
// newest first as listSnapshots gives them.
func stamps(n int, start time.Time, gap time.Duration) []snapshotFile {
	out := make([]snapshotFile, n)
	for i := range out {
		when := start.Add(-time.Duration(i) * gap)
		out[i] = snapshotFile{Name: timestampedName(when, "zxs"), Format: "zxs", When: when, Timestamped: true}
	}
	return out
}

func at(y int, m time.Month, d, hh, mm, ss int) time.Time {
	return time.Date(y, m, d, hh, mm, ss, 0, time.Local)
}

// Up to ten snapshots are all in the menu and there is no More...; past ten, the rest are More's.
func TestPlanSnapshotMenu(t *testing.T) {
	now := at(2026, 3, 9, 15, 0, 0)
	for _, n := range []int{0, 1, 9, 10} {
		p := planSnapshotMenu(stamps(n, now, time.Minute), now)
		if len(p.Head) != n || len(p.More) != 0 {
			t.Errorf("%d snapshots: %d in the menu, %d groups", n, len(p.Head), len(p.More))
		}
	}
	all := stamps(11, now, time.Minute)
	p := planSnapshotMenu(all, now)
	if len(p.Head) != loadTimestampedRows || p.Head[0].Name != all[0].Name || p.Head[9].Name != all[9].Name {
		t.Errorf("the menu has %d rows, from %s", len(p.Head), p.Head[0].Name)
	}
	if len(p.More) != 1 || p.More[0].Kind != groupFlat || len(p.More[0].Items) != 1 || p.More[0].Items[0].Name != all[10].Name {
		t.Errorf("More is %+v", p.More)
	}
}

// Up to twenty more are listed as they are; twenty-one are grouped.
func TestMoreIsFlatUntilThereAreMany(t *testing.T) {
	now := at(2026, 3, 9, 15, 0, 0)
	flat := groupSnapshots(stamps(moreFlatMax, now, time.Hour), now)
	if len(flat) != 1 || flat[0].Kind != groupFlat || len(flat[0].Items) != moreFlatMax {
		t.Fatalf("%d snapshots are %d groups of kind %v", moreFlatMax, len(flat), flat[0].Kind)
	}
	many := groupSnapshots(stamps(moreFlatMax+1, now, time.Hour), now)
	if many[0].Kind == groupFlat {
		t.Errorf("%d snapshots are not grouped", moreFlatMax+1)
	}
	if groupSnapshots(nil, now) != nil {
		t.Error("no snapshots make a group")
	}
}

// Grouped, the snapshots are today's, then a day each for the week, then a week each, then a month
// each, the newest first, and every snapshot is in exactly one group.
func TestGroupSnapshotsByDayWeekAndMonth(t *testing.T) {
	now := at(2026, 3, 11, 15, 0, 0) // a Wednesday
	var rest []snapshotFile
	add := func(when time.Time) {
		rest = append(rest, snapshotFile{Name: timestampedName(when, "zxs"), Format: "zxs", When: when, Timestamped: true})
	}
	// today (three), yesterday (two), the day before (one), six days back (one: still a day)
	add(at(2026, 3, 11, 14, 0, 0))
	add(at(2026, 3, 11, 9, 0, 0))
	add(at(2026, 3, 11, 0, 0, 1))
	add(at(2026, 3, 10, 23, 59, 59))
	add(at(2026, 3, 10, 8, 0, 0))
	add(at(2026, 3, 9, 8, 0, 0))
	add(at(2026, 3, 5, 8, 0, 0))
	// seven days back is a week: the week of Monday 2 March holds 4 March, and the last one of 2 March
	add(at(2026, 3, 4, 8, 0, 0))
	add(at(2026, 3, 2, 8, 0, 0))
	// the week of Monday 23 February
	add(at(2026, 3, 1, 8, 0, 0))
	add(at(2026, 2, 24, 8, 0, 0))
	// 34 days back is a week, 35 is a month
	add(now.AddDate(0, 0, -34))
	add(now.AddDate(0, 0, -35))
	add(at(2026, 1, 20, 8, 0, 0))
	add(at(2025, 12, 31, 8, 0, 0))
	for i := 0; i < 10; i++ { // enough to be grouped at all
		add(at(2025, 6, 10-i, 8, 0, 0))
	}

	groups := groupSnapshots(rest, now)
	var got []string
	total := 0
	for _, g := range groups {
		got = append(got, groupMenuLabel(g))
		total += len(g.Items)
	}
	if total != len(rest) {
		t.Errorf("the groups hold %d snapshots, there are %d", total, len(rest))
	}
	for i, want := range []string{
		"Today (3)", "Yesterday (2)", "Mon 9 Mar (1)", "Thu 5 Mar (1)",
		"Week of 2 Mar (2)", "Week of 23 Feb (2)",
	} {
		if i >= len(got) || got[i] != want {
			t.Fatalf("the groups are %v, want them to begin %s", got, want)
		}
	}
	// the rest are months or weeks, newest first, and no group appears twice
	seen := map[string]bool{}
	for _, g := range groups {
		if seen[g.Label] {
			t.Errorf("%s is two groups", g.Label)
		}
		seen[g.Label] = true
		for i := 1; i < len(g.Items); i++ {
			if g.Items[i].When.After(g.Items[i-1].When) {
				t.Errorf("%s is not newest first", g.Label)
			}
		}
	}
	for _, label := range []string{"January 2026", "December 2025", "June 2025"} {
		if !seen[label] {
			t.Errorf("no group %q in %v", label, got)
		}
	}
	// 34 days back is a week and 35 a month
	if g := groupOf(groups, now.AddDate(0, 0, -34)); g.Kind != groupWeek {
		t.Errorf("34 days back is %v: %s", g.Kind, g.Label)
	}
	if g := groupOf(groups, now.AddDate(0, 0, -35)); g.Kind != groupMonth {
		t.Errorf("35 days back is %v: %s", g.Kind, g.Label)
	}
}

func groupOf(groups []snapshotGroup, when time.Time) snapshotGroup {
	for _, g := range groups {
		for _, f := range g.Items {
			if f.When.Equal(when) {
				return g
			}
		}
	}
	return snapshotGroup{}
}

// A day is a calendar day: a snapshot from 23 hours ago, the night before, is yesterday's, not
// today's; and one from a clock that was set back is today's.
func TestGroupingCountsCalendarDays(t *testing.T) {
	now := at(2026, 3, 11, 0, 30, 0)
	rest := stamps(moreFlatMax+1, now.Add(-23*time.Hour), time.Minute)
	rest = append([]snapshotFile{{Name: "future", Format: "zxs", When: now.Add(2 * time.Hour), Timestamped: true}}, rest...)
	g := groupSnapshots(rest, now)
	if g[0].Label != "Today" || len(g[0].Items) != 1 || g[0].Items[0].Name != "future" {
		t.Errorf("the first group is %s with %d", g[0].Label, len(g[0].Items))
	}
	if g[1].Label != "Yesterday" {
		t.Errorf("23 hours back is %s", g[1].Label)
	}
}

// Weeks begin on a Monday, and a Sunday is the end of its week.
func TestMonday(t *testing.T) {
	for d, want := range map[int]int{9: 9, 10: 9, 15: 9, 16: 16} { // March 2026: the 9th is a Monday, the 15th a Sunday
		if got := monday(at(2026, 3, d, 13, 0, 0)); got.Day() != want || got.Hour() != 0 {
			t.Errorf("the Monday of March %d is %v, want the %dth", d, got, want)
		}
	}
	// across a month
	if got := monday(at(2026, 3, 1, 8, 0, 0)); got.Month() != time.February || got.Day() != 23 {
		t.Errorf("the Monday of March 1 is %v", got)
	}
}

// A row says the time, or the date as well, and the format when it is not the usual one.
func TestSnapshotRowLabels(t *testing.T) {
	f := snapshotFile{Format: "zxs", When: at(2026, 3, 9, 14, 5, 7)}
	if got := snapshotRowLabel(f, false); got != "14:05:07" {
		t.Errorf("%q", got)
	}
	if got := snapshotRowLabel(f, true); got != "Mon 9 Mar 14:05:07" {
		t.Errorf("%q", got)
	}
	f.Format = "sna"
	if got := snapshotRowLabel(f, true); got != "Mon 9 Mar 14:05:07 (SNA)" {
		t.Errorf("%q", got)
	}
	for _, s := range []string{snapshotRowLabel(f, true), groupMenuLabel(snapshotGroup{Label: "Today", Items: []snapshotFile{f}})} {
		for _, r := range s {
			if r > 126 {
				t.Errorf("%q has a character the typeface lacks", s)
			}
		}
	}
}

// A group's menu lists no more than groupRowsMax, and says how many are left.
func TestShownInGroup(t *testing.T) {
	now := at(2026, 3, 9, 15, 0, 0)
	for _, n := range []int{1, groupRowsMax, groupRowsMax + 1, groupRowsMax + 40} {
		g := snapshotGroup{Label: "Today", Items: stamps(n, now, time.Second)}
		shown, left := shownInGroup(g)
		if len(shown)+left != n || len(shown) > groupRowsMax || (n > groupRowsMax) != (left > 0) {
			t.Errorf("%d: shown %d, left %d", n, len(shown), left)
		}
	}
	if label := groupMenuLabel(snapshotGroup{Label: "Week of 2 Mar", Items: make([]snapshotFile, 12)}); !strings.HasPrefix(label, "Week of 2 Mar (12)") {
		t.Error(label)
	}
}
