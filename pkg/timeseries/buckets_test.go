package timeseries

import (
	"testing"
	"time"
)

func TestFirstBucketIsEpochAligned(t *testing.T) {
	cases := []struct {
		interval time.Duration
		want     int64
	}{
		{time.Minute, 960},
		{7 * time.Second, 994},
	}
	for _, c := range cases {
		plan := NewPlan(time.Unix(1000, 0), time.Unix(1200, 0), c.interval, 0, 1000, "s", time.UTC)

		if got := plan.Buckets[0].Start.Unix(); got != c.want {
			t.Errorf("interval %v: first bucket start = %d, want %d", c.interval, got, c.want)
		}
	}
}

func TestBucketsCoverRangeUntilTo(t *testing.T) {
	plan := NewPlan(time.Unix(1000, 0), time.Unix(1200, 0), time.Minute, 0, 1000, "s", time.UTC)

	if len(plan.Buckets) != 4 {
		t.Fatalf("bucket count = %d, want 4", len(plan.Buckets))
	}
	last := plan.Buckets[3]
	if last.From != 1140 || last.To != 1200 || !last.Start.Equal(time.Unix(1140, 0)) {
		t.Fatalf("last bucket = %+v, want [1140, 1200) starting at %v", last, time.Unix(1140, 0))
	}
}

func TestMinimumIntervalOverridesSmallerGrafanaInterval(t *testing.T) {
	plan := NewPlan(time.Unix(1000, 0), time.Unix(1200, 0), 10*time.Second, time.Minute, 1000, "s", time.UTC)

	if len(plan.Buckets) != 4 || !plan.Buckets[0].Start.Equal(time.Unix(960, 0)) || plan.Buckets[0].To != 1020 {
		t.Fatalf("buckets = %+v, want 4 one-minute buckets from 960", plan.Buckets)
	}
}

func TestMillisecondUnitValues(t *testing.T) {
	plan := NewPlan(time.UnixMilli(1500), time.UnixMilli(2500), 500*time.Millisecond, 0, 1000, "ms", time.UTC)

	first := plan.Buckets[0]
	if len(plan.Buckets) != 2 || first.From != 1500 || first.To != 2000 {
		t.Fatalf("buckets = %+v, want 2 buckets starting [1500, 2000)", plan.Buckets)
	}
}

func TestWidenUsesSmallestMultipleWithinMax(t *testing.T) {
	plan := NewPlan(time.Unix(5, 0), time.Unix(1005, 0), 10*time.Second, 0, 1000, "s", time.UTC)

	wide, widened := plan.Widen(50)

	if !widened {
		t.Fatal("widened = false, want true")
	}
	if len(wide.Buckets) != 34 || !wide.Buckets[0].Start.Equal(time.Unix(0, 0)) || wide.Buckets[0].To != 30 {
		t.Fatalf("got %d buckets starting %+v, want 34 thirty-second buckets from 0", len(wide.Buckets), wide.Buckets[0])
	}
}

func TestWidenLeavesPlanWithinMaxUnchanged(t *testing.T) {
	plan := NewPlan(time.Unix(1000, 0), time.Unix(1200, 0), time.Minute, 0, 1000, "s", time.UTC)

	wide, widened := plan.Widen(50)

	if widened {
		t.Fatal("widened = true, want false")
	}
	if len(wide.Buckets) != 4 || wide.Buckets[0].To != 1020 {
		t.Fatalf("buckets = %+v, want the original 4 one-minute buckets", wide.Buckets)
	}
}

func TestBucketStartIsUTC(t *testing.T) {
	plan := NewPlan(time.Unix(1000, 0), time.Unix(1200, 0), time.Minute, 0, 1000, "s", time.UTC)

	if loc := plan.Buckets[0].Start.Location(); loc != time.UTC {
		t.Fatalf("start location = %v, want UTC", loc)
	}
}

func TestZeroIntervalDividesRangeByMaxDataPoints(t *testing.T) {
	plan := NewPlan(time.Unix(1000, 0), time.Unix(1200, 0), 0, 0, 4, "s", time.UTC)

	if len(plan.Buckets) != 4 || plan.Buckets[0].To != 1050 {
		t.Fatalf("buckets = %+v, want 4 fifty-second buckets", plan.Buckets)
	}
}

func TestSecondsUnitRaisesSubSecondIntervalToOneSecond(t *testing.T) {
	plan := NewPlan(time.Unix(1000, 0), time.Unix(1003, 0), 200*time.Millisecond, 0, 1000, "s", time.UTC)

	if len(plan.Buckets) != 3 || plan.Buckets[0].To != 1001 {
		t.Fatalf("buckets = %+v, want 3 one-second buckets", plan.Buckets)
	}
}

func TestEdgeBucketsAreClippedToRange(t *testing.T) {
	plan := NewPlan(time.Unix(1000, 0), time.Unix(1190, 0), time.Minute, 0, 1000, "s", time.UTC)

	first, last := plan.Buckets[0], plan.Buckets[len(plan.Buckets)-1]
	if first.From != 1000 || !first.Start.Equal(time.Unix(960, 0)) {
		t.Errorf("first bucket = %+v, want From 1000 with Start 960", first)
	}
	if last.To != 1190 {
		t.Errorf("last bucket = %+v, want To 1190", last)
	}
}

func TestDayIntervalsFollowLocalMidnightsAcrossDST(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 3, 28, 12, 0, 0, 0, berlin)
	to := time.Date(2026, 3, 30, 12, 0, 0, 0, berlin)

	plan := NewPlan(from, to, 24*time.Hour, 0, 1000, "s", berlin)

	wantStarts := []time.Time{
		time.Date(2026, 3, 27, 23, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 28, 23, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 29, 22, 0, 0, 0, time.UTC),
	}
	if len(plan.Buckets) != len(wantStarts) {
		t.Fatalf("buckets = %+v, want %d", plan.Buckets, len(wantStarts))
	}
	for i, want := range wantStarts {
		if !plan.Buckets[i].Start.Equal(want) {
			t.Errorf("bucket %d start = %v, want %v", i, plan.Buckets[i].Start, want)
		}
	}
	if dst := plan.Buckets[1]; dst.To-dst.From != 23*60*60 {
		t.Errorf("DST bucket spans %ds, want 23h", dst.To-dst.From)
	}
}

func TestWeekIntervalsStartOnLocalMonday(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	wednesday := time.Date(2026, 11, 18, 10, 0, 0, 0, newYork)

	plan := NewPlan(wednesday, wednesday.AddDate(0, 0, 7), 7*24*time.Hour, 0, 1000, "s", newYork)

	monday := time.Date(2026, 11, 16, 0, 0, 0, 0, newYork)
	if len(plan.Buckets) != 2 || !plan.Buckets[0].Start.Equal(monday) {
		t.Fatalf("buckets = %+v, want 2 weeks starting %v", plan.Buckets, monday)
	}
}

func TestWidenToDaysUsesLocalCalendarDays(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 3, 1, 12, 0, 0, 0, berlin)
	plan := NewPlan(from, from.AddDate(0, 0, 60), 12*time.Hour, 0, 1000, "s", berlin)

	wide, widened := plan.Widen(50)

	if !widened || len(wide.Buckets) > 50 {
		t.Fatalf("widened = %v with %d buckets, want true with at most 50", widened, len(wide.Buckets))
	}
	for _, bucket := range wide.Buckets {
		if local := bucket.Start.In(berlin); local.Hour() != 0 || local.Minute() != 0 {
			t.Fatalf("bucket start %v is not a Berlin midnight", local)
		}
	}
}

func TestPartialDayIntervalsRoundUpToWholeDays(t *testing.T) {
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	plan := NewPlan(from, from.AddDate(0, 0, 4), 36*time.Hour, 0, 1000, "s", time.UTC)

	if len(plan.Buckets) != 2 || plan.Buckets[0].To-plan.Buckets[0].From != 2*24*60*60 {
		t.Fatalf("buckets = %+v, want 2 two-day buckets", plan.Buckets)
	}
}

func TestZeroMaxDataPointsDefaultsToOneThousand(t *testing.T) {
	plan := NewPlan(time.Unix(0, 0), time.Unix(2000, 0), 0, 0, 0, "s", time.UTC)

	if len(plan.Buckets) != 1000 || plan.Buckets[0].To != 2 {
		t.Fatalf("got %d buckets, want 1000 two-second buckets", len(plan.Buckets))
	}
}

func TestMillisecondsUnitRaisesSubMillisecondIntervalToOneMillisecond(t *testing.T) {
	plan := NewPlan(time.UnixMilli(1000), time.UnixMilli(1003), 200*time.Microsecond, 0, 1000, "ms", time.UTC)

	if len(plan.Buckets) != 3 || plan.Buckets[0].To != 1001 {
		t.Fatalf("got %d buckets starting %+v, want 3 one-millisecond buckets", len(plan.Buckets), plan.Buckets[0])
	}
}

func TestNilLocationMeansUTC(t *testing.T) {
	from := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

	plan := NewPlan(from, from.AddDate(0, 0, 1), 24*time.Hour, 0, 1000, "s", nil)

	if want := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC); !plan.Buckets[0].Start.Equal(want) {
		t.Fatalf("first start = %v, want %v", plan.Buckets[0].Start, want)
	}
}

func TestMultiDayIntervalsSitOnFixedLocalDayGrid(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	gridDay := time.Date(2026, 11, 1, 0, 0, 0, 0, newYork)

	for _, from := range []time.Time{gridDay.Add(12 * time.Hour), gridDay.AddDate(0, 0, 1).Add(12 * time.Hour)} {
		plan := NewPlan(from, from.AddDate(0, 0, 4), 48*time.Hour, 0, 1000, "s", newYork)

		if !plan.Buckets[0].Start.Equal(gridDay) {
			t.Errorf("from %v: first start = %v, want %v", from, plan.Buckets[0].Start.In(newYork), gridDay)
		}
	}
}

func TestIntervalReportsEffectiveBucketWidth(t *testing.T) {
	cases := []struct {
		interval, want time.Duration
	}{
		{90 * time.Minute, 90 * time.Minute},
		{36 * time.Hour, 48 * time.Hour},
	}
	for _, c := range cases {
		plan := NewPlan(time.Unix(0, 0), time.Unix(10*24*3600, 0), c.interval, 0, 1000, "s", time.UTC)
		if got := plan.Interval(); got != c.want {
			t.Errorf("interval %v: got %v, want %v", c.interval, got, c.want)
		}
	}
}

func TestIntervalOfWidenedPlanIsTheWidenedWidth(t *testing.T) {
	plan, _ := NewPlan(time.Unix(0, 0), time.Unix(60*3600, 0), 30*time.Minute, 0, 1000, "s", time.UTC).Widen(50)
	if got := plan.Interval(); got != 90*time.Minute {
		t.Fatalf("got %v, want 90m", got)
	}
}

func TestUnitOtherThanMillisecondsMeansSeconds(t *testing.T) {
	plan := NewPlan(time.Unix(1000, 0), time.Unix(1003, 0), time.Second, 0, 1000, "", time.UTC)

	if plan.Buckets[0].From != 1000 || plan.Buckets[0].To != 1001 {
		t.Fatalf("first bucket = %+v, want [1000, 1001) in seconds", plan.Buckets[0])
	}
}
