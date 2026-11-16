package onec

import (
	"strconv"
	"strings"
	"time"
)

func unquoteRACValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
		inner := v[1 : len(v)-1]
		inner = strings.ReplaceAll(inner, `\"`, `"`)
		inner = strings.ReplaceAll(inner, `\\`, `\`)
		return inner
	}
	return v
}

// parseRACBlocks parses standard `rac.exe` key-value output blocks separated by blank lines.
func parseRACBlocks(text string) []map[string]string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")

	var blocks []map[string]string
	current := make(map[string]string)

	flush := func() {
		if len(current) > 0 {
			blocks = append(blocks, current)
			current = make(map[string]string)
		}
	}

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			flush()
			continue
		}
		idx := strings.IndexByte(rawLine, ':')
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(rawLine[:idx]))
		val := unquoteRACValue(rawLine[idx+1:])
		if key != "" {
			current[key] = val
		}
	}
	flush()

	return blocks
}

var timeLayouts = []string{
	"2006-01-02T15:04:05",
	"2006-01-02T15:04:05.999999999",
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02 15:04:05",
	"02.01.2006 15:04:05",
}

func parse1CTime(raw string) time.Time {
	s := strings.TrimSpace(raw)
	if s == "" || strings.HasPrefix(s, "0001-01-01") || strings.HasPrefix(s, "0000-") {
		return time.Time{}
	}
	for _, layout := range timeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ParseClusters parses `rac cluster list` output.
func ParseClusters(text string) []Cluster {
	blocks := parseRACBlocks(text)
	var out []Cluster
	for _, b := range blocks {
		id := strings.TrimSpace(b["cluster"])
		if id == "" {
			continue
		}
		port, _ := strconv.Atoi(strings.TrimSpace(b["port"]))
		out = append(out, Cluster{
			ID:   id,
			Name: b["name"],
			Host: b["host"],
			Port: port,
		})
	}
	return out
}

// ParseInfobases parses `rac infobase summary list` output into a map keyed by lowercase infobase UUID.
func ParseInfobases(text string) map[string]Infobase {
	blocks := parseRACBlocks(text)
	out := make(map[string]Infobase, len(blocks))
	for _, b := range blocks {
		id := strings.TrimSpace(b["infobase"])
		if id == "" {
			continue
		}
		out[strings.ToLower(id)] = Infobase{
			ID:          id,
			Name:        b["name"],
			Description: b["descr"],
		}
	}
	return out
}

// ParseSessions parses `rac session list` output for a given cluster.
func ParseSessions(text string, cluster Cluster, infobases map[string]Infobase) []Session {
	blocks := parseRACBlocks(text)
	var out []Session
	for _, b := range blocks {
		uuid := strings.TrimSpace(b["session"])
		numStr := strings.TrimSpace(b["session-id"])
		if uuid == "" && numStr == "" {
			continue
		}
		num, _ := strconv.Atoi(numStr)
		ibID := strings.TrimSpace(b["infobase"])
		var ibName, ibDescr string
		if ib, ok := infobases[strings.ToLower(ibID)]; ok {
			ibName = ib.Name
			ibDescr = ib.Description
		}

		hibRaw := strings.ToLower(strings.TrimSpace(b["hibernate"]))
		hib := hibRaw == "yes" || hibRaw == "true" || hibRaw == "1" || hibRaw == "да"

		startedRaw := b["started-at"]
		lastActiveRaw := b["last-active-at"]

		out = append(out, Session{
			SessionUUID:   uuid,
			SessionNumber: num,
			ClusterID:     cluster.ID,
			ClusterName:   cluster.Name,
			ClusterHost:   cluster.Host,
			ClusterPort:   cluster.Port,
			InfobaseID:    ibID,
			InfobaseName:  ibName,
			InfobaseDescr: ibDescr,
			UserName:      b["user-name"],
			AppID:         b["app-id"],
			Host:          b["host"],
			ClientIP:      b["client-ip"],
			StartedAt:     parse1CTime(startedRaw),
			StartedAtRaw:  startedRaw,
			LastActiveAt:  parse1CTime(lastActiveRaw),
			LastActiveRaw: lastActiveRaw,
			Hibernate:     hib,
		})
	}
	return out
}
