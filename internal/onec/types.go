package onec

import (
	"fmt"
	"strings"
	"time"
)

// SessionStatus indicates how a session is classified during duplicate analysis.
type SessionStatus int

const (
	StatusCurrent SessionStatus = iota
	StatusDuplicateOld
	StatusProtected
	StatusSystem
)

// Label returns the Russian status badge text for table display.
func (s SessionStatus) Label() string {
	switch s {
	case StatusDuplicateOld:
		return "СТАРЫЙ (дубль)"
	case StatusProtected:
		return "Конфигуратор"
	case StatusSystem:
		return "Системный"
	default:
		return "Актуальный"
	}
}

// Cluster describes a 1C:Enterprise server cluster.
type Cluster struct {
	ID   string
	Name string
	Host string
	Port int
}

// Infobase describes a 1C:Enterprise infobase summary inside a cluster.
type Infobase struct {
	ID          string
	Name        string
	Description string
}

// Session represents a running session on a 1C:Enterprise server.
type Session struct {
	SessionUUID   string
	SessionNumber int
	ClusterID     string
	ClusterName   string
	ClusterHost   string
	ClusterPort   int
	InfobaseID    string
	InfobaseName  string
	InfobaseDescr string
	UserName      string
	AppID         string
	Host          string
	ClientIP      string
	StartedAt     time.Time
	StartedAtRaw  string
	LastActiveAt  time.Time
	LastActiveRaw string
	Hibernate     bool
	Status        SessionStatus
}

// UserDisplay returns the user name or a placeholder for system sessions without a user.
func (s Session) UserDisplay() string {
	u := strings.TrimSpace(s.UserName)
	if u == "" {
		return "<системный / без имени>"
	}
	return u
}

// InfobaseDisplay returns a readable infobase label.
func (s Session) InfobaseDisplay() string {
	name := strings.TrimSpace(s.InfobaseName)
	descr := strings.TrimSpace(s.InfobaseDescr)
	if name != "" && descr != "" && !strings.EqualFold(name, descr) {
		return fmt.Sprintf("%s (%s)", name, descr)
	}
	if name != "" {
		return name
	}
	if descr != "" {
		return descr
	}
	id := strings.TrimSpace(s.InfobaseID)
	if len(id) > 12 {
		return id[:8] + "…"
	}
	if id != "" {
		return id
	}
	return "—"
}

// AppDisplayName translates 1C app-id codes into clear Russian descriptions.
func (s Session) AppDisplayName() string {
	switch strings.TrimSpace(s.AppID) {
	case "1CV8C":
		return "Тонкий клиент (1CV8C)"
	case "1CV8":
		return "Толстый клиент (1CV8)"
	case "WebClient":
		return "Веб-клиент"
	case "Designer":
		return "Конфигуратор"
	case "BackgroundJob":
		return "Фоновое задание"
	case "SystemBackgroundJob":
		return "Системное фоновое"
	case "JobScheduler":
		return "Планировщик заданий"
	case "SrvrConsole":
		return "Консоль кластера"
	case "RAS":
		return "Сервер администрирования"
	case "COMConnection":
		return "COM-соединение"
	case "COMConsole":
		return "COM-консоль"
	case "WSConnection":
		return "Веб-сервис (WS)"
	case "HTTPServiceConnection":
		return "HTTP-сервис"
	case "ODataConnection":
		return "OData"
	case "":
		return "—"
	default:
		return s.AppID
	}
}

// HostDisplay formats client workstation host and IP.
func (s Session) HostDisplay() string {
	h := strings.TrimSpace(s.Host)
	ip := strings.TrimSpace(s.ClientIP)
	if h != "" && ip != "" && !strings.EqualFold(h, ip) {
		return fmt.Sprintf("%s (%s)", h, ip)
	}
	if h != "" {
		return h
	}
	if ip != "" {
		return ip
	}
	return "—"
}

// StartedAtDisplay formats session start timestamp.
func (s Session) StartedAtDisplay() string {
	if !s.StartedAt.IsZero() && s.StartedAt.Year() > 1970 {
		return s.StartedAt.Format("02.01.2006 15:04:05")
	}
	if strings.TrimSpace(s.StartedAtRaw) != "" {
		return s.StartedAtRaw
	}
	return "—"
}

// LastActiveDisplay formats last activity timestamp and hibernation indicator.
func (s Session) LastActiveDisplay() string {
	base := "—"
	if !s.LastActiveAt.IsZero() && s.LastActiveAt.Year() > 1970 {
		base = s.LastActiveAt.Format("02.01.2006 15:04:05")
	} else if strings.TrimSpace(s.LastActiveRaw) != "" {
		base = s.LastActiveRaw
	}
	if s.Hibernate {
		return base + " (спящий)"
	}
	return base
}

// IsSystemSession returns true if the session is a background/service task or has no user name.
func (s Session) IsSystemSession() bool {
	if strings.TrimSpace(s.UserName) == "" {
		return true
	}
	switch strings.TrimSpace(s.AppID) {
	case "BackgroundJob", "SystemBackgroundJob", "JobScheduler", "SrvrConsole", "RAS":
		return true
	default:
		return false
	}
}

// DedupeOptions configures how duplicate old sessions are identified per user.
type DedupeOptions struct {
	// GroupByServer deduplicates across the entire server by user name when true;
	// when false (default), deduplicates per user within each infobase.
	GroupByServer bool
	// ProtectDesigner keeps Configurator (Designer) sessions untouched when true.
	ProtectDesigner bool
}

// SessionStats summarizes classified sessions for UI badges.
type SessionStats struct {
	Total             int
	UniqueUsers       int
	DuplicatesOld     int
	SystemOrProtected int
}

// TerminateFailure records an error when terminating a specific old session.
type TerminateFailure struct {
	Session Session
	Err     error
}

// CleanupReport summarizes the result of removing old duplicate sessions.
type CleanupReport struct {
	EvaluatedTotal int
	Deleted        []Session
	Failed         []TerminateFailure
	Remaining      []Session
}
