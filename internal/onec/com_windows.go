//go:build windows

package onec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"death1c/internal/config"
)

const comPowerShellScript = `
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

$progIDs = @('V83.COMConnector', 'V85.COMConnector', 'V82.COMConnector')
$connector = $null
$lastComErr = $null
foreach ($pidName in $progIDs) {
    try {
        $connector = New-Object -ComObject $pidName
        if ($null -ne $connector) { break }
    } catch {
        $lastComErr = $_
    }
}
if ($null -eq $connector) {
    Write-Error "CLASS_NOT_REGISTERED: Не удалось создать COM-объект V83.COMConnector ($lastComErr)"
    exit 3
}

try {
    $agentAddr = [string]$env:DEATH1C_AGENT
    $adminUser = [string]$env:DEATH1C_USER
    $adminPass = [string]$env:DEATH1C_PASS
    $action    = [string]$env:DEATH1C_ACTION

    $agent = $connector.ConnectAgent($agentAddr)
    $clusters = @($agent.GetClusters())

    if ($action -eq 'list') {
        $items = @()
        foreach ($cl in $clusters) {
            $clHost = [string]$cl.HostName
            $clPort = [int]$cl.MainPort
            $clName = [string]$cl.ClusterName
            $clID   = "${clHost}:${clPort}|${clName}"

            $agent.Authenticate($cl, $adminUser, $adminPass)
            $sessions = @($agent.GetSessions($cl))
            foreach ($s in $sessions) {
                $ibName = ''
                $ibDescr = ''
                if ($null -ne $s.infoBase) {
                    $ibName  = [string]$s.infoBase.Name
                    $ibDescr = [string]$s.infoBase.Descr
                }
                $startedStr = ''
                if ($null -ne $s.StartedAt) {
                    $startedStr = ([datetime]$s.StartedAt).ToString('yyyy-MM-ddTHH:mm:ss')
                }
                $activeStr = ''
                if ($null -ne $s.LastActiveAt) {
                    $activeStr = ([datetime]$s.LastActiveAt).ToString('yyyy-MM-ddTHH:mm:ss')
                }
                $sidNum = [int]$s.SessionID
                $uid = "${clID}|${ibName}|${sidNum}|${startedStr}"
                $items += [PSCustomObject]@{
                    uuid           = $uid
                    session_id     = $sidNum
                    cluster_id     = $clID
                    cluster_name   = $clName
                    cluster_host   = $clHost
                    cluster_port   = $clPort
                    infobase_name  = $ibName
                    infobase_descr = $ibDescr
                    user_name      = [string]$s.UserName
                    app_id         = [string]$s.AppID
                    host           = [string]$s.Host
                    started_at     = $startedStr
                    last_active_at = $activeStr
                    hibernate      = [bool]$s.Hibernate
                }
            }
        }
        ConvertTo-Json -InputObject @($items) -Depth 4 -Compress
    }
    elseif ($action -eq 'terminate') {
        $targetUID = [string]$env:DEATH1C_TARGET_UUID
        $terminated = $false
        foreach ($cl in $clusters) {
            $clHost = [string]$cl.HostName
            $clPort = [int]$cl.MainPort
            $clName = [string]$cl.ClusterName
            $clID   = "${clHost}:${clPort}|${clName}"

            $agent.Authenticate($cl, $adminUser, $adminPass)
            $sessions = @($agent.GetSessions($cl))
            foreach ($s in $sessions) {
                $ibName = ''
                if ($null -ne $s.infoBase) {
                    $ibName = [string]$s.infoBase.Name
                }
                $startedStr = ''
                if ($null -ne $s.StartedAt) {
                    $startedStr = ([datetime]$s.StartedAt).ToString('yyyy-MM-ddTHH:mm:ss')
                }
                $sidNum = [int]$s.SessionID
                $uid = "${clID}|${ibName}|${sidNum}|${startedStr}"
                if ($uid -eq $targetUID) {
                    $agent.TerminateSession($cl, $s)
                    $terminated = $true
                    break
                }
            }
            if ($terminated) { break }
        }
        if (-not $terminated) {
            Write-Output '{"already_closed":true}'
        } else {
            Write-Output '{"terminated":true}'
        }
    }
} finally {
    if ($null -ne $connector) {
        [void][System.Runtime.InteropServices.Marshal]::ReleaseComObject($connector)
    }
}
`

type comSessionDTO struct {
	UUID          string `json:"uuid"`
	SessionID     int    `json:"session_id"`
	ClusterID     string `json:"cluster_id"`
	ClusterName   string `json:"cluster_name"`
	ClusterHost   string `json:"cluster_host"`
	ClusterPort   int    `json:"cluster_port"`
	InfobaseName  string `json:"infobase_name"`
	InfobaseDescr string `json:"infobase_descr"`
	UserName      string `json:"user_name"`
	AppID         string `json:"app_id"`
	Host          string `json:"host"`
	StartedAt     string `json:"started_at"`
	LastActiveAt  string `json:"last_active_at"`
	Hibernate     bool   `json:"hibernate"`
}

type comBackend struct {
	cfg config.Config
}

func powerShellCandidates() []string {
	sysRoot := os.Getenv("SystemRoot")
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}
	var out []string
	ps64 := filepath.Join(sysRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	ps32 := filepath.Join(sysRoot, "SysWOW64", "WindowsPowerShell", "v1.0", "powershell.exe")
	for _, p := range []string{ps64, ps32} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		out = append(out, "powershell.exe")
	}
	return out
}

func (c *comBackend) runScript(ctx context.Context, action string, targetUUID string) (string, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
	}

	candidates := powerShellCandidates()
	var lastErr error

	for _, psBin := range candidates {
		cmd := exec.CommandContext(
			ctx,
			psBin,
			"-NoProfile",
			"-NonInteractive",
			"-WindowStyle", "Hidden",
			"-Command", "-",
		)
		configureHiddenProcess(cmd)
		cmd.Stdin = strings.NewReader(comPowerShellScript)
		cmd.Env = append(os.Environ(),
			"DEATH1C_AGENT="+c.cfg.EffectiveAgentAddr(),
			"DEATH1C_USER="+strings.TrimSpace(c.cfg.ClusterUser),
			"DEATH1C_PASS="+c.cfg.ClusterPassword,
			"DEATH1C_ACTION="+action,
			"DEATH1C_TARGET_UUID="+targetUUID,
		)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		outStr := strings.TrimSpace(DecodeConsoleOutput(stdout.Bytes()))
		errStr := strings.TrimSpace(DecodeConsoleOutput(stderr.Bytes()))
		if err == nil {
			return outStr, nil
		}

		// If 64-bit COM class is not registered, try 32-bit SysWOW64 PowerShell next
		if strings.Contains(errStr, "CLASS_NOT_REGISTERED") || (cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 3) {
			lastErr = fmt.Errorf("COM-компонента V83.COMConnector не зарегистрирована (выполните regsvr32 comcntr.dll от имени администратора или используйте режим RAC)")
			continue
		}
		if errStr != "" {
			return "", fmt.Errorf("ошибка COM-подключения к %s: %s", c.cfg.EffectiveAgentAddr(), errStr)
		}
		lastErr = fmt.Errorf("ошибка COM-подключения к %s: %w", c.cfg.EffectiveAgentAddr(), err)
		break
	}
	return "", lastErr
}

func (c *comBackend) ListSessions(ctx context.Context) ([]Session, error) {
	rawJSON, err := c.runScript(ctx, "list", "")
	if err != nil {
		return nil, err
	}
	if rawJSON == "" || rawJSON == "null" {
		return nil, nil
	}

	var dtos []comSessionDTO
	if strings.HasPrefix(rawJSON, "{") {
		var single comSessionDTO
		if err := json.Unmarshal([]byte(rawJSON), &single); err != nil {
			return nil, fmt.Errorf("не удалось разобрать ответ COM: %w", err)
		}
		dtos = []comSessionDTO{single}
	} else {
		if err := json.Unmarshal([]byte(rawJSON), &dtos); err != nil {
			return nil, fmt.Errorf("не удалось разобрать список сеансов COM: %w", err)
		}
	}

	sessions := make([]Session, 0, len(dtos))
	for _, d := range dtos {
		sessions = append(sessions, Session{
			SessionUUID:   d.UUID,
			SessionNumber: d.SessionID,
			ClusterID:     d.ClusterID,
			ClusterName:   d.ClusterName,
			ClusterHost:   d.ClusterHost,
			ClusterPort:   d.ClusterPort,
			InfobaseID:    d.InfobaseName,
			InfobaseName:  d.InfobaseName,
			InfobaseDescr: d.InfobaseDescr,
			UserName:      d.UserName,
			AppID:         d.AppID,
			Host:          d.Host,
			StartedAt:     parse1CTime(d.StartedAt),
			StartedAtRaw:  d.StartedAt,
			LastActiveAt:  parse1CTime(d.LastActiveAt),
			LastActiveRaw: d.LastActiveAt,
			Hibernate:     d.Hibernate,
		})
	}
	return sessions, nil
}

func (c *comBackend) TerminateSession(ctx context.Context, s Session) error {
	_, err := c.runScript(ctx, "terminate", s.SessionUUID)
	return err
}
