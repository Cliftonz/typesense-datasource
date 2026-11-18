package timeseries

import "time"

type Bucket struct {
	Start    time.Time
	From, To int64
}

type Plan struct {
	Buckets  []Bucket
	from, to time.Time
	interval time.Duration
	unit     string
	location *time.Location
}

func NewPlan(from, to time.Time, interval, minInterval time.Duration, maxDataPoints int64, unit string, location *time.Location) Plan {
	interval = max(interval, minInterval)
	if interval == 0 {
		if maxDataPoints == 0 {
			maxDataPoints = 1000
		}
		interval = to.Sub(from) / time.Duration(maxDataPoints)
	}
	toUnit := time.Time.Unix
	if unit == "ms" {
		interval = max(interval, time.Millisecond)
		toUnit = time.Time.UnixMilli
	} else {
		interval = max(interval, time.Second)
	}
	if location == nil {
		location = time.UTC
	}
	first := time.Unix(0, from.UnixNano()/int64(interval)*int64(interval))
	next := func(t time.Time) time.Time { return t.Add(interval) }
	if interval >= 24*time.Hour {
		days := int((interval + 24*time.Hour - 1) / (24 * time.Hour))
		interval = time.Duration(days) * 24 * time.Hour
		local := from.In(location)
		first = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
		if days%7 == 0 {
			daysSinceMonday := (int(first.Weekday()) + 6) % 7
			first = first.AddDate(0, 0, -daysSinceMonday)
		} else {
			localDaysSince1970 := int(time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC).Unix() / (24 * 60 * 60))
			first = first.AddDate(0, 0, -(localDaysSince1970 % days))
		}
		next = func(t time.Time) time.Time { return t.AddDate(0, 0, days) }
	}
	plan := Plan{from: from, to: to, interval: interval, unit: unit, location: location}
	for start := first; start.Before(to); start = next(start) {
		end := next(start)
		bucket := Bucket{Start: start.UTC(), From: toUnit(start), To: toUnit(end)}
		if start.Before(from) {
			bucket.From = toUnit(from)
		}
		if end.After(to) {
			bucket.To = toUnit(to)
		}
		plan.Buckets = append(plan.Buckets, bucket)
	}
	return plan
}

func (p Plan) Widen(maxBuckets int) (plan Plan, widened bool) {
	for multiple := time.Duration(1); ; multiple++ {
		plan = NewPlan(p.from, p.to, multiple*p.interval, 0, 0, p.unit, p.location)
		if len(plan.Buckets) <= maxBuckets {
			return plan, multiple > 1
		}
	}
}

func (p Plan) Interval() time.Duration {
	return p.interval
}
