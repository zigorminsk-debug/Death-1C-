package onec

import (
	"testing"
	"time"
)

func mustTime(t *testing.T, v string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02T15:04:05", v)
	if err != nil {
		t.Fatalf("parse time %q: %v", v, err)
	}
	return parsed
}

func TestClassifySessionsKeepsMostActualPerUser(t *testing.T) {
	sessions := []Session{
		{
			SessionUUID:   "s1",
			SessionNumber: 101,
			ClusterID:     "c1",
			InfobaseID:    "ib-buh",
			InfobaseName:  "Бухгалтерия",
			UserName:      "Иванов И.И.",
			AppID:         "1CV8C",
			StartedAt:     mustTime(t, "2026-11-16T08:00:00"),
			LastActiveAt:  mustTime(t, "2026-11-16T08:45:00"),
			Hibernate:     false,
		},
		{
			SessionUUID:   "s2",
			SessionNumber: 115,
			ClusterID:     "c1",
			InfobaseID:    "ib-buh",
			InfobaseName:  "Бухгалтерия",
			UserName:      " иванов и.и. ",
			AppID:         "1CV8C",
			StartedAt:     mustTime(t, "2026-11-16T09:10:00"),
			LastActiveAt:  mustTime(t, "2026-11-16T11:05:30"),
			Hibernate:     false,
		},
		{
			// Newer start time, but hibernating (disconnected) — must NOT beat active session s2!
			SessionUUID:   "s3",
			SessionNumber: 120,
			ClusterID:     "c1",
			InfobaseID:    "ib-buh",
			InfobaseName:  "Бухгалтерия",
			UserName:      "Иванов И.И.",
			AppID:         "1CV8C",
			StartedAt:     mustTime(t, "2026-11-16T10:00:00"),
			LastActiveAt:  mustTime(t, "2026-11-16T11:06:00"),
			Hibernate:     true,
		},
		{
			// Designer session for the same user — protected when ProtectDesigner=true
			SessionUUID:   "s4",
			SessionNumber: 90,
			ClusterID:     "c1",
			InfobaseID:    "ib-buh",
			InfobaseName:  "Бухгалтерия",
			UserName:      "Иванов И.И.",
			AppID:         "Designer",
			StartedAt:     mustTime(t, "2026-11-16T07:50:00"),
			LastActiveAt:  mustTime(t, "2026-11-16T08:00:00"),
		},
		{
			// Same user in a DIFFERENT infobase (ЗУП) — kept in per-IB mode, removed in server-wide mode if older
			SessionUUID:   "s5",
			SessionNumber: 108,
			ClusterID:     "c1",
			InfobaseID:    "ib-zup",
			InfobaseName:  "ЗУП",
			UserName:      "Иванов И.И.",
			AppID:         "1CV8C",
			StartedAt:     mustTime(t, "2026-11-16T09:00:00"),
			LastActiveAt:  mustTime(t, "2026-11-16T10:30:00"),
		},
		{
			// Background job — never touched
			SessionUUID:   "s6",
			SessionNumber: 55,
			ClusterID:     "c1",
			InfobaseID:    "ib-buh",
			InfobaseName:  "Бухгалтерия",
			UserName:      "",
			AppID:         "BackgroundJob",
			StartedAt:     mustTime(t, "2026-11-16T06:00:00"),
		},
	}

	classified, stats := ClassifySessions(sessions, DedupeOptions{
		GroupByServer:   false,
		ProtectDesigner: true,
	})

	if stats.Total != 6 {
		t.Fatalf("expected Total=6, got %d", stats.Total)
	}
	if stats.DuplicatesOld != 2 {
		t.Fatalf("expected DuplicatesOld=2 (s1 and s3), got %d", stats.DuplicatesOld)
	}

	toDelete := SelectDuplicatesToDelete(classified)
	if len(toDelete) != 2 {
		t.Fatalf("expected 2 sessions to delete, got %d", len(toDelete))
	}
	gotIDs := map[string]bool{toDelete[0].SessionUUID: true, toDelete[1].SessionUUID: true}
	if !gotIDs["s1"] || !gotIDs["s3"] {
		t.Fatalf("expected s1 and s3 to be flagged as old duplicates, got %+v", gotIDs)
	}

	// Verify GroupByServer=true also treats s5 (older activity in ЗУП) as duplicate of s2 (newer in Бухгалтерия)
	classifiedGlobal, statsGlobal := ClassifySessions(sessions, DedupeOptions{
		GroupByServer:   true,
		ProtectDesigner: true,
	})
	if statsGlobal.DuplicatesOld != 3 {
		t.Fatalf("expected 3 duplicates in GroupByServer mode, got %d", statsGlobal.DuplicatesOld)
	}
	for _, s := range classifiedGlobal {
		if s.SessionUUID == "s2" && s.Status != StatusCurrent {
			t.Fatalf("expected s2 to remain StatusCurrent in global mode, got %v", s.Status)
		}
	}
}
