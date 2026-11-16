package onec

import (
	"context"
	"strings"

	"death1c/internal/config"
)

type backend interface {
	ListSessions(ctx context.Context) ([]Session, error)
	TerminateSession(ctx context.Context, s Session) error
}

// Client manages listing and cleaning up duplicate 1C:Enterprise Server sessions.
type Client struct {
	cfg     config.Config
	backend backend
}

// NewClient constructs a Client for the configured connection mode (RAC or COM).
func NewClient(cfg config.Config) *Client {
	var b backend
	if cfg.ConnectionMode == config.ModeCOM {
		b = &comBackend{cfg: cfg}
	} else {
		b = &racBackend{cfg: cfg}
	}
	return &Client{
		cfg:     cfg,
		backend: b,
	}
}

func (c *Client) dedupeOptions() DedupeOptions {
	return DedupeOptions{
		GroupByServer:   c.cfg.GroupByServer,
		ProtectDesigner: c.cfg.ProtectDesigner,
	}
}

// FetchAndClassify retrieves all running sessions from the 1C server and classifies
// the most up-to-date session vs older duplicate sessions per user.
func (c *Client) FetchAndClassify(ctx context.Context) ([]Session, SessionStats, error) {
	raw, err := c.backend.ListSessions(ctx)
	if err != nil {
		return nil, SessionStats{}, err
	}
	classified, stats := ClassifySessions(raw, c.dedupeOptions())
	return classified, stats, nil
}

func isAlreadyGoneError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "сеанс отсутствует") ||
		strings.Contains(msg, "сеанс не найден") ||
		strings.Contains(msg, "session does not exist") ||
		strings.Contains(msg, "session not found")
}

// CleanupOldDuplicates fetches the latest live sessions, terminates all old duplicate sessions
// while keeping the single most up-to-date session per user, and returns a full CleanupReport.
func (c *Client) CleanupOldDuplicates(
	ctx context.Context,
	onProgress func(index, total int, s Session),
) (CleanupReport, error) {
	classified, _, err := c.FetchAndClassify(ctx)
	if err != nil {
		return CleanupReport{}, err
	}

	report := CleanupReport{
		EvaluatedTotal: len(classified),
	}

	targets := SelectDuplicatesToDelete(classified)
	if len(targets) == 0 {
		report.Remaining = classified
		return report, nil
	}

	for i, s := range targets {
		if ctx.Err() != nil {
			break
		}
		if onProgress != nil {
			onProgress(i+1, len(targets), s)
		}
		termErr := c.backend.TerminateSession(ctx, s)
		if termErr != nil && !isAlreadyGoneError(termErr) {
			report.Failed = append(report.Failed, TerminateFailure{
				Session: s,
				Err:     termErr,
			})
		} else {
			report.Deleted = append(report.Deleted, s)
		}
	}

	// Refresh remaining sessions from server
	if updated, _, refreshErr := c.FetchAndClassify(ctx); refreshErr == nil {
		report.Remaining = updated
	}

	return report, nil
}
