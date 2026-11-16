package onec

import (
	"sort"
	"strings"
	"time"
)

func effectiveActivity(s Session) time.Time {
	if !s.LastActiveAt.IsZero() && s.LastActiveAt.Year() > 1970 {
		return s.LastActiveAt
	}
	if !s.StartedAt.IsZero() && s.StartedAt.Year() > 1970 {
		return s.StartedAt
	}
	return time.Time{}
}

// isMoreActual returns true if session a is strictly more up-to-date ("максимально актуальный") than session b.
func isMoreActual(a, b Session) bool {
	// 1. Active (non-hibernating) session always beats a hibernating (disconnected) session.
	if a.Hibernate != b.Hibernate {
		return !a.Hibernate
	}

	// 2. Most recent activity timestamp wins.
	actA := effectiveActivity(a)
	actB := effectiveActivity(b)
	if !actA.Equal(actB) {
		return actA.After(actB)
	}

	// 3. Most recently started session wins on activity tie.
	if !a.StartedAt.Equal(b.StartedAt) {
		return a.StartedAt.After(b.StartedAt)
	}

	// 4. Higher numeric session-id wins.
	if a.SessionNumber != b.SessionNumber {
		return a.SessionNumber > b.SessionNumber
	}

	// 5. Deterministic tie-breaker on UUID.
	return a.SessionUUID > b.SessionUUID
}

func groupKey(s Session, opts DedupeOptions) string {
	user := strings.ToLower(strings.TrimSpace(s.UserName))
	if opts.GroupByServer {
		return user
	}
	ib := strings.ToLower(strings.TrimSpace(s.InfobaseID))
	if ib == "" {
		ib = strings.ToLower(strings.TrimSpace(s.InfobaseName))
	}
	cluster := strings.ToLower(strings.TrimSpace(s.ClusterID))
	if cluster == "" {
		cluster = strings.ToLower(strings.TrimSpace(s.ClusterName))
	}
	return cluster + "|" + ib + "|" + user
}

// ClassifySessions analyzes all running 1C sessions, marks the single most up-to-date session
// per user as StatusCurrent, and flags all older duplicate sessions as StatusDuplicateOld.
func ClassifySessions(sessions []Session, opts DedupeOptions) ([]Session, SessionStats) {
	result := make([]Session, len(sessions))
	copy(result, sessions)

	groups := make(map[string][]int)
	uniqueUsers := make(map[string]struct{})
	stats := SessionStats{Total: len(result)}

	for i := range result {
		s := &result[i]
		normUser := strings.ToLower(strings.TrimSpace(s.UserName))
		if normUser != "" {
			uniqueUsers[normUser] = struct{}{}
		}

		if s.IsSystemSession() {
			s.Status = StatusSystem
			stats.SystemOrProtected++
			continue
		}

		if opts.ProtectDesigner && strings.EqualFold(strings.TrimSpace(s.AppID), "Designer") {
			s.Status = StatusProtected
			stats.SystemOrProtected++
			continue
		}

		key := groupKey(*s, opts)
		groups[key] = append(groups[key], i)
	}

	for _, idxs := range groups {
		if len(idxs) == 0 {
			continue
		}
		bestIdx := idxs[0]
		for _, candidateIdx := range idxs[1:] {
			if isMoreActual(result[candidateIdx], result[bestIdx]) {
				bestIdx = candidateIdx
			}
		}
		for _, idx := range idxs {
			if idx == bestIdx {
				result[idx].Status = StatusCurrent
			} else {
				result[idx].Status = StatusDuplicateOld
				stats.DuplicatesOld++
			}
		}
	}

	stats.UniqueUsers = len(uniqueUsers)

	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		sysA := a.Status == StatusSystem
		sysB := b.Status == StatusSystem
		if sysA != sysB {
			return !sysA
		}
		ua := strings.ToLower(strings.TrimSpace(a.UserName))
		ub := strings.ToLower(strings.TrimSpace(b.UserName))
		if ua != ub {
			return ua < ub
		}
		iba := strings.ToLower(a.InfobaseDisplay())
		ibb := strings.ToLower(b.InfobaseDisplay())
		if iba != ibb {
			return iba < ibb
		}
		if a.Status != b.Status {
			return a.Status < b.Status
		}
		return isMoreActual(a, b)
	})

	return result, stats
}

// SelectDuplicatesToDelete returns only the sessions marked StatusDuplicateOld.
func SelectDuplicatesToDelete(classified []Session) []Session {
	var out []Session
	for _, s := range classified {
		if s.Status == StatusDuplicateOld {
			out = append(out, s)
		}
	}
	return out
}
