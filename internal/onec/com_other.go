//go:build !windows

package onec

import (
	"context"
	"fmt"

	"death1c/internal/config"
)

type comBackend struct {
	cfg config.Config
}

func (c *comBackend) ListSessions(_ context.Context) ([]Session, error) {
	return nil, fmt.Errorf("режим COM (V83.COMConnector) поддерживается только в ОС Windows; используйте режим RAC")
}

func (c *comBackend) TerminateSession(_ context.Context, _ Session) error {
	return fmt.Errorf("режим COM (V83.COMConnector) поддерживается только в ОС Windows; используйте режим RAC")
}
