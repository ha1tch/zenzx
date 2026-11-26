// Copyright (c) 2026 haitch
// Licensed under the Apache License, Version 2.0

package zenzx

import (
	"fmt"
	"strings"
	"time"
)

// Snapshot > Load Timestamped lists the snapshots that Save Timestamped made. The newest few are
// rows of the menu itself; the rest are behind its "More..." row, which is a submenu of its own.
// A person who saves often has far more than a menu holds, so past a few the rest are grouped by
// when they were made: the rest of today, by day for the last week, by week for the five weeks
// after that, by month for everything older. This is the arithmetic, apart from the window, so
// that it can be tested without one (snapshotmenu_gui.go makes the menus from it).

const (
	// loadTimestampedRows is how many of the newest snapshots are rows of Load Timestamped.
	loadTimestampedRows = 10
	// moreFlatMax is how many older snapshots "More..." lists as they are, without groups. More
	// than that, and they are grouped.
	moreFlatMax = 20
	// groupRowsMax is how many snapshots a group's menu has before it says how many are left and
	// points to the Manage Snapshots window.
	groupRowsMax = 25

	// dayGroups and weekGroups are how many days back (today is day 0) the groups are by day,
	// and how many days back they are by week; further back they are by month.
	dayGroups  = 7
	weekGroups = 35
)

// groupKind says how a group's snapshots were gathered.
type groupKind int

const (
	groupFlat  groupKind = iota // all of More's snapshots, not grouped
	groupToday                  // the rest of today
	groupDay                    // one of the last days
	groupWeek                   // one week
	groupMonth                  // one calendar month
)

// snapshotGroup is a group of "More...": a label, and the snapshots in it, the newest first.
type snapshotGroup struct {
	Kind  groupKind
	Label string
	Items []snapshotFile
}

// snapshotPlan is what Load Timestamped shows: the newest snapshots as rows, and "More..."'s
// groups (none when there are no more than the rows).
type snapshotPlan struct {
	Head []snapshotFile
	More []snapshotGroup
}

// planSnapshotMenu divides the timestamped snapshots, which are newest first, into what the
// menu itself lists and what More... holds. now is the time the groups are counted from.
func planSnapshotMenu(stamped []snapshotFile, now time.Time) snapshotPlan {
	if len(stamped) <= loadTimestampedRows {
		return snapshotPlan{Head: stamped}
	}
	p := snapshotPlan{Head: stamped[:loadTimestampedRows]}
	p.More = groupSnapshots(stamped[loadTimestampedRows:], now)
	return p
}

// dayNumber is the date of t as a count of days, so that the days between two times are a
// subtraction, with whole days: 23 hours is not a day.
func dayNumber(t time.Time) int {
	y, m, d := t.Date()
	return int(time.Date(y, m, d, 12, 0, 0, 0, time.UTC).Unix() / 86400)
}

// monday is the first day of the week t is in, the week beginning on a Monday.
func monday(t time.Time) time.Time {
	y, m, d := t.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, t.Location())
	back := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -back)
}

// groupSnapshots gathers snapshots, the newest first, into More...'s groups, the newest group
// first. A few are one flat group (moreFlatMax); more are grouped by today, day, week and month.
func groupSnapshots(rest []snapshotFile, now time.Time) []snapshotGroup {
	if len(rest) == 0 {
		return nil
	}
	if len(rest) <= moreFlatMax {
		return []snapshotGroup{{Kind: groupFlat, Items: rest}}
	}
	today := dayNumber(now)
	var groups []snapshotGroup
	for _, f := range rest {
		age := today - dayNumber(f.When)
		if age < 0 {
			age = 0 // a clock that was set back: it is today's
		}
		var kind groupKind
		var label string
		switch {
		case age == 0:
			kind, label = groupToday, "Today"
		case age == 1:
			kind, label = groupDay, "Yesterday"
		case age < dayGroups:
			kind, label = groupDay, f.When.Format("Mon 2 Jan")
		case age < weekGroups:
			kind, label = groupWeek, "Week of "+monday(f.When).Format("2 Jan")
		default:
			kind, label = groupMonth, f.When.Format("January 2006")
		}
		if n := len(groups); n > 0 && groups[n-1].Kind == kind && groups[n-1].Label == label {
			groups[n-1].Items = append(groups[n-1].Items, f)
			continue
		}
		groups = append(groups, snapshotGroup{Kind: kind, Label: label, Items: []snapshotFile{f}})
	}
	return groups
}

// snapshotRowLabel is a snapshot's row in a menu: the date and time, or just the time where the
// menu's heading already says the day, and the format when it is not the usual one. Only ASCII,
// which is what the menu's typeface has.
func snapshotRowLabel(f snapshotFile, withDate bool) string {
	label := f.When.Format("15:04:05")
	if withDate {
		label = f.When.Format("Mon 2 Jan 15:04:05")
	}
	if f.Format != "zxs" {
		label += " (" + strings.ToUpper(f.Format) + ")"
	}
	return label
}

// groupMenuLabel is the row that opens a group: its name and how many snapshots it holds.
func groupMenuLabel(g snapshotGroup) string {
	return fmt.Sprintf("%s (%d)", g.Label, len(g.Items))
}

// shownInGroup is the snapshots of a group that its menu lists, and how many it leaves to the
// Manage Snapshots window.
func shownInGroup(g snapshotGroup) (shown []snapshotFile, left int) {
	if len(g.Items) <= groupRowsMax {
		return g.Items, 0
	}
	return g.Items[:groupRowsMax], len(g.Items) - groupRowsMax
}
