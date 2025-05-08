package service

import (
	"strings"
	"testing"
	"time"

	"github.com/pentops/trigger/utils"
)

func TestValidateCronString(t *testing.T) {
	err := utils.ValidateCronString("0 18 1 * *")
	if err != nil {
		t.Error("validate cron string failed, expected no error")
	}

	err = utils.ValidateCronString("0 Fail 1 * *")
	if err == nil {
		t.Error("validate cron string failed, expected an error")
	}
}

func TestCalcDelay(t *testing.T) {
	now := mustParseTime(t, "2025-01-01 06:46:00")

	// tick was in the past so should not have a delay
	delay := calcDelay(mustParseTime(t, "2025-01-01 06:35:00"), now)
	if delay != 0 {
		t.Error("calcDelay failed, expected 0")
	}

	// tick was in the past so should not have a delay
	delay = calcDelay(mustParseTime(t, "2025-01-01 06:40:00"), now)
	if delay != 0 {
		t.Error("calcDelay failed, expected 0")
	}

	// tick is just a minute in the in the past so should have a 1 minute delay
	delay = calcDelay(mustParseTime(t, "2025-01-01 06:45:00"), now)
	if time.Duration(delay.Seconds()) == 60 {
		t.Error("calcDelay failed, expected 1 minute got ", delay.Minutes())
	}
}

func TestNextTick(t *testing.T) {
	lastTick := mustParseTime(t, "2025-01-01 13:00:00Z")
	tick, err := nextTick(lastTick)
	if err != nil {
		t.Error("nextTick failed, expected no error")
	}
	if tick.String() != "2025-01-01 13:01:00 +0000 UTC" {
		t.Fatal("expected 2025-01-01 13:01:00, got", tick.String())
	}

	lastTick = mustParseTime(t, "2025-01-31 23:59:51")
	tick, err = nextTick(lastTick)
	if err != nil {
		t.Error("nextTick failed, expected no error")
	}
	if tick.String() != "2025-02-01 00:00:00 +0000 UTC" {
		t.Fatal("expected 2025-02-01 00:00:00, got", tick.String())
	}
}

func TestCheckCron(t *testing.T) {
	// localized time during standard time
	shouldTigger, err := checkCron("CRON_TZ=America/New_York 0 7 * * *", mustParseTime(t, "2025-01-04 12:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if !shouldTigger {
		t.Error("checkCron should return true")
	}
	shouldTigger, err = checkCron("CRON_TZ=America/New_York 0 8 * * *", mustParseTime(t, "2025-01-04 12:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}

	// localized time during daylight savings time
	shouldTigger, err = checkCron("CRON_TZ=America/New_York 0 7 * * *", mustParseTime(t, "2025-08-04 11:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if !shouldTigger {
		t.Error("checkCron should return true")
	}
	shouldTigger, err = checkCron("CRON_TZ=America/New_York 0 8 * * *", mustParseTime(t, "2025-08-04 11:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}

	shouldTigger, err = checkCron("59 12 * * *", mustParseTime(t, "2025-01-01 13:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}

	shouldTigger, err = checkCron("0 13 * * *", mustParseTime(t, "2025-01-01 13:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if !shouldTigger {
		t.Error("checkCron should return true")
	}

	shouldTigger, err = checkCron("1 13 * * *", mustParseTime(t, "2025-01-01 13:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}

	shouldTigger, err = checkCron("5 13 * * *", mustParseTime(t, "2025-01-01 13:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}

	shouldTigger, err = checkCron("@hourly", mustParseTime(t, "2025-01-01 13:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if !shouldTigger {
		t.Error("checkCron should return true")
	}

	shouldTigger, err = checkCron("@hourly", mustParseTime(t, "2025-01-01 13:01:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}

	shouldTigger, err = checkCron("@yearly", mustParseTime(t, "2025-01-01 00:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if !shouldTigger {
		t.Error("checkCron should return true")
	}
	shouldTigger, err = checkCron("@yearly", mustParseTime(t, "2025-01-01 13:01:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}

	shouldTigger, err = checkCron("@monthly", mustParseTime(t, "2025-01-01 00:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if !shouldTigger {
		t.Error("checkCron should return true")
	}
	shouldTigger, err = checkCron("@monthly", mustParseTime(t, "2024-12-31 23:23:23Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}
	shouldTigger, err = checkCron("@monthly", mustParseTime(t, "2025-01-02 00:01:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}

	shouldTigger, err = checkCron("@weekly", mustParseTime(t, "2025-01-05 00:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if !shouldTigger {
		t.Error("checkCron should return true")
	}
	shouldTigger, err = checkCron("@weekly", mustParseTime(t, "2025-02-04 23:23:23Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}
	shouldTigger, err = checkCron("@weekly", mustParseTime(t, "2025-02-05 00:01:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}

	shouldTigger, err = checkCron("@daily", mustParseTime(t, "2025-02-06 00:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if !shouldTigger {
		t.Error("checkCron should return true")
	}
	shouldTigger, err = checkCron("@daily", mustParseTime(t, "2025-02-06 23:23:23Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}

	shouldTigger, err = checkCron("@hourly", mustParseTime(t, "2024-12-31 13:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if !shouldTigger {
		t.Error("checkCron should return true")
	}
	shouldTigger, err = checkCron("@hourly", mustParseTime(t, "2024-12-31 23:59:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}

	shouldTigger, err = checkCron("@yearly", mustParseTime(t, "2025-01-01 00:00:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if !shouldTigger {
		t.Error("checkCron should return true")
	}
	shouldTigger, err = checkCron("@yearly", mustParseTime(t, "2024-12-31 23:59:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}
	shouldTigger, err = checkCron("@yearly", mustParseTime(t, "2025-01-01 00:01:00Z"))
	if err != nil {
		t.Error("expected no error")
	}
	if shouldTigger {
		t.Error("checkCron should return false")
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	parseString := "2006-01-02 15:04:05"
	if strings.Contains(s, "Z") {
		parseString = "2006-01-02 15:04:05Z"
	}

	tm, err := time.Parse(parseString, s)
	if err != nil {
		t.Fatalf("failed to parse time %s: %v", s, err)
	}
	return tm
}
