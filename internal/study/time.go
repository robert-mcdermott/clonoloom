package study

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Time is a sample's time point as the annotation gives it, read: a number of days from the
// subject's reference event (an infusion, a transplant, a vaccination: day 0), or, where the
// annotation says only that, before it (baseline), after it (post), or a numbered visit.
type Time struct {
	Text  string   `json:"text"`
	Kind  string   `json:"kind"`           // day, before, after or visit
	Days  *float64 `json:"days,omitempty"` // for a day
	Value float64  `json:"value"`          // as written, in Unit (a visit's number)
	Unit  string   `json:"unit,omitempty"` // hour, day, week, month or year; visit
	Event string   `json:"event,omitempty"`
	Label string   `json:"label"` // as the views show it: day 28, week 4, baseline, visit 2
	Bare  bool     `json:"bare,omitempty"`
}

// The kinds of time point.
const (
	Day    = "day"
	Before = "before"
	After  = "after"
	Visit  = "visit"
)

// Days in each unit: a month is a twelfth of a Julian year (30.4375 days), as most clinical
// schedules count them only roughly; the label keeps the unit as written.
var unitDays = map[string]float64{"hour": 1.0 / 24, "day": 1, "week": 7, "month": 365.25 / 12, "year": 365.25}

var (
	// An event in parentheses, as AIRR Repertoire metadata name theirs: "14 day (vaccination)".
	inParentheses = regexp.MustCompile(`^(.*?)\s*\(([^()]*)\)$`)
	beforeWords   = regexp.MustCompile(`^(baseline|bl|pre|before|screening|pre[- ]?(treatment|infusion|vaccination|transplant|dose|therapy))$`)
	afterWords    = regexp.MustCompile(`^(post|after|follow[- ]?up|post[- ]?(treatment|infusion|vaccination|transplant|dose|therapy))$`)
	// day 28, d28, D+28, d-7, wk4, week 4, m3, month 3, y1, h6.
	unitFirst = regexp.MustCompile(`^(hours?|hrs?|h|days?|d|weeks?|wks?|w|months?|mos?|m|years?|yrs?|y)\s*([+-]?\d+(?:\.\d+)?)$`)
	// 28d, 28 days, 4 weeks, 3mo, 6h; and 28 days post-infusion (the event after it).
	numberFirst = regexp.MustCompile(`^([+-]?\d+(?:\.\d+)?)\s*(hours?|hrs?|h|days?|d|weeks?|wks?|w|months?|mos?|m|years?|yrs?|y)(?:\s+(?:post|after|from|since)[- ]?(.+))?$`)
	visitWords  = regexp.MustCompile(`^(t|tp|v|visit|time ?point)\s*(\d+)$`)
	bareNumber  = regexp.MustCompile(`^[+-]?\d+(?:\.\d+)?$`)
)

// unitOf names a unit written as h, hr, d, wk, mo, y and the like.
func unitOf(s string) string {
	switch {
	case strings.HasPrefix(s, "h"):
		return "hour"
	case strings.HasPrefix(s, "d"):
		return "day"
	case strings.HasPrefix(s, "w"):
		return "week"
	case strings.HasPrefix(s, "m"):
		return "month"
	}
	return "year"
}

// ParseTime reads a time point (see Time), false where it is not one ClonoLoom reads.
func ParseTime(text string) (Time, bool) {
	t := Time{Text: strings.TrimSpace(text)}
	s := strings.ToLower(t.Text)
	if m := inParentheses.FindStringSubmatch(s); m != nil && m[1] != "" {
		s, t.Event = m[1], strings.TrimSpace(t.Text[strings.LastIndex(t.Text, "(")+1:len(t.Text)-1])
	}
	s = strings.Join(strings.Fields(s), " ")
	day := func(value float64, unit string) (Time, bool) {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return t, false
		}
		days := value * unitDays[unit]
		if math.IsInf(days, 0) {
			return t, false
		}
		t.Kind, t.Value, t.Unit, t.Days = Day, value, unit, &days
		t.Label = unit + " " + strconv.FormatFloat(value, 'f', -1, 64)
		return t, true
	}
	switch {
	case s == "":
		return t, false
	case beforeWords.MatchString(s):
		t.Kind, t.Label = Before, "baseline"
		return t, true
	case afterWords.MatchString(s):
		t.Kind, t.Label = After, "after"
		return t, true
	}
	if m := unitFirst.FindStringSubmatch(s); m != nil {
		v, _ := strconv.ParseFloat(m[2], 64)
		return day(v, unitOf(m[1]))
	}
	if m := numberFirst.FindStringSubmatch(s); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		if m[3] != "" && t.Event == "" {
			t.Event = m[3]
		}
		return day(v, unitOf(m[2]))
	}
	if m := visitWords.FindStringSubmatch(s); m != nil {
		n, _ := strconv.ParseFloat(m[2], 64)
		t.Kind, t.Value, t.Unit, t.Label = Visit, n, Visit, "visit "+m[2]
		return t, true
	}
	if bareNumber.MatchString(s) {
		v, _ := strconv.ParseFloat(s, 64)
		t.Bare = true
		return day(v, "day")
	}
	return t, false
}

// Same says whether two time points are the same time: the same number of days (day 28 and week 4
// are), both baseline or both after, or the same visit.
func Same(a, b Time) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case Day:
		return math.Abs(*a.Days-*b.Days) < 1e-9
	case Visit:
		return a.Value == b.Value
	}
	return true
}

// rank orders time points: baseline first, then days, then visits, then after (days and visits
// cannot be ordered against each other, and the checks say so where a subject has both).
func rank(t Time) (int, float64) {
	switch t.Kind {
	case Before:
		return 0, 0
	case Day:
		return 1, *t.Days
	case Visit:
		return 2, t.Value
	}
	return 3, 0
}

func earlier(a, b Time) bool {
	ra, va := rank(a)
	rb, vb := rank(b)
	if ra != rb {
		return ra < rb
	}
	return va < vb
}
