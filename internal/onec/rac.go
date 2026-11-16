package onec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"death1c/internal/config"
)

var versionTokenRe = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)\.(\d+)`)

func extractVersionNumbers(path string) [4]int {
	var ver [4]int
	m := versionTokenRe.FindStringSubmatch(path)
	if len(m) == 5 {
		for i := 0; i < 4; i++ {
			ver[i], _ = strconv.Atoi(m[i+1])
		}
	}
	return ver
}

// DiscoverRACPaths searches standard 1C:Enterprise installation folders and PATH for rac.exe,
// returning found binaries sorted from newest platform version to oldest.
func DiscoverRACPaths() []string {
	var candidates []string

	if runtime.GOOS == "windows" {
		roots := []string{
			os.Getenv("ProgramW6432"),
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
			`C:\Program Files`,
			`C:\Program Files (x86)`,
			`D:\Program Files`,
			`D:\Program Files (x86)`,
		}
		for _, r := range roots {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			matches, _ := filepath.Glob(filepath.Join(r, "1cv8", "*", "bin", "rac.exe"))
			candidates = append(candidates, matches...)
		}
		if lp, err := exec.LookPath("rac.exe"); err == nil {
			candidates = append(candidates, lp)
		}
	} else {
		for _, pattern := range []string{
			"/opt/1cv8/*/*/rac",
			"/opt/1C/v8.3/*/rac",
		} {
			matches, _ := filepath.Glob(pattern)
			candidates = append(candidates, matches...)
		}
		if lp, err := exec.LookPath("rac"); err == nil {
			candidates = append(candidates, lp)
		}
	}

	seen := make(map[string]struct{})
	var unique []string
	for _, c := range candidates {
		clean := filepath.Clean(c)
		info, err := os.Stat(clean)
		if err != nil || info.IsDir() {
			continue
		}
		key := clean
		if runtime.GOOS == "windows" {
			key = strings.ToLower(clean)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, clean)
	}

	sort.SliceStable(unique, func(i, j int) bool {
		vi := extractVersionNumbers(unique[i])
		vj := extractVersionNumbers(unique[j])
		for k := 0; k < 4; k++ {
			if vi[k] != vj[k] {
				return vi[k] > vj[k]
			}
		}
		// Prefer 64-bit Program Files over Program Files (x86) on equal version
		x86i := strings.Contains(strings.ToLower(unique[i]), "(x86)")
		x86j := strings.Contains(strings.ToLower(unique[j]), "(x86)")
		if x86i != x86j {
			return !x86i
		}
		return unique[i] < unique[j]
	})

	return unique
}

type racBackend struct {
	cfg config.Config
}

func (r *racBackend) resolveBinary() (string, error) {
	explicit := strings.TrimSpace(r.cfg.RACPath)
	if explicit != "" {
		base := strings.ToLower(filepath.Base(explicit))
		if base == "ras.exe" || base == "ras" {
			return "", fmt.Errorf("вы указали путь к серверной службе (ras.exe). Пожалуйста, укажите путь к клиентской утилите (rac.exe)")
		}

		info, err := os.Stat(explicit)
		if err == nil && !info.IsDir() {
			return explicit, nil
		}
		return "", fmt.Errorf("указанный файл rac.exe не найден: %s", explicit)
	}
	found := DiscoverRACPaths()
	if len(found) > 0 {
		return found[0], nil
	}
	return "", fmt.Errorf("утилита администрирования 1С (rac.exe) не найдена автоматически — укажите путь к rac.exe в настройках или выберите режим подключения COM (V83.COMConnector)")
}

func (r *racBackend) clusterAuthArgs() []string {
	user := strings.TrimSpace(r.cfg.ClusterUser)
	if user == "" {
		return nil
	}
	return []string{
		"--cluster-user=" + user,
		"--cluster-pwd=" + r.cfg.ClusterPassword,
	}
}

func (r *racBackend) runRAC(ctx context.Context, bin string, args ...string) (string, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	configureHiddenProcess(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outStr := DecodeConsoleOutput(stdout.Bytes())
	errStr := strings.TrimSpace(DecodeConsoleOutput(stderr.Bytes()))

	if err != nil {
		detail := errStr
		if detail == "" {
			detail = strings.TrimSpace(outStr)
		}
		
		// Improve error message if connection is refused (RAS not running)
		if strings.Contains(detail, "Подключение не установлено") || 
			strings.Contains(detail, "connection refused") || 
			strings.Contains(detail, "отверг запрос на подключение") {
			detail += "\n\nСлужба RAS 1С не запущена на порту 1545!\nВам нужно запустить её на сервере 1С командой:\nras.exe cluster --port=1545"
		}

		if detail != "" {
			return outStr, fmt.Errorf("ошибка выполнения rac (%s): %s", r.cfg.EffectiveRASAddr(), detail)
		}
		return outStr, fmt.Errorf("не удалось выполнить rac (%s): %w (проверьте, запущена ли служба RAS 1С на порту 1545)", r.cfg.EffectiveRASAddr(), err)
	}
	return outStr, nil
}

func (r *racBackend) ListSessions(ctx context.Context) ([]Session, error) {
	bin, err := r.resolveBinary()
	if err != nil {
		return nil, err
	}
	rasAddr := r.cfg.EffectiveRASAddr()

	clusterOut, err := r.runRAC(ctx, bin, "cluster", "list", rasAddr)
	if err != nil {
		return nil, err
	}
	clusters := ParseClusters(clusterOut)
	if len(clusters) == 0 {
		return nil, fmt.Errorf("на сервере %s не обнаружено кластеров 1С", rasAddr)
	}

	auth := r.clusterAuthArgs()
	var allSessions []Session

	for _, cl := range clusters {
		// Fetch infobase summaries (best-effort to map infobase UUIDs -> readable names)
		ibArgs := []string{"infobase", "summary", "list", "--cluster=" + cl.ID}
		ibArgs = append(ibArgs, auth...)
		ibArgs = append(ibArgs, rasAddr)
		ibMap := map[string]Infobase{}
		if ibOut, ibErr := r.runRAC(ctx, bin, ibArgs...); ibErr == nil {
			ibMap = ParseInfobases(ibOut)
		}

		// Fetch all active sessions in this cluster
		sessArgs := []string{"session", "list", "--cluster=" + cl.ID}
		sessArgs = append(sessArgs, auth...)
		sessArgs = append(sessArgs, rasAddr)
		sessOut, sessErr := r.runRAC(ctx, bin, sessArgs...)
		if sessErr != nil {
			return nil, sessErr
		}
		parsed := ParseSessions(sessOut, cl, ibMap)
		allSessions = append(allSessions, parsed...)
	}

	return allSessions, nil
}

func (r *racBackend) TerminateSession(ctx context.Context, s Session) error {
	bin, err := r.resolveBinary()
	if err != nil {
		return err
	}
	if strings.TrimSpace(s.ClusterID) == "" || strings.TrimSpace(s.SessionUUID) == "" {
		return fmt.Errorf("отсутствует идентификатор кластера или сеанса (#%d)", s.SessionNumber)
	}

	args := []string{
		"session", "terminate",
		"--cluster=" + s.ClusterID,
		"--session=" + s.SessionUUID,
	}
	args = append(args, r.clusterAuthArgs()...)
	args = append(args, "--error-message=Сеанс завершён администратором (удаление дублирующегося сеанса)")
	args = append(args, r.cfg.EffectiveRASAddr())

	_, err = r.runRAC(ctx, bin, args...)
	return err
}
