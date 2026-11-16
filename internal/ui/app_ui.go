package ui

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"death1c/internal/config"
	"death1c/internal/onec"
)

const (
	modeOptionRAC = "RAC / RAS (рекомендуется, порт 1545)"
	modeOptionCOM = "COM (V83.COMConnector, агент 1540)"

	groupOptionPerIB  = "В рамках каждой информационной базы"
	groupOptionServer = "По всему серверу 1С"
)

var tableHeaders = []struct {
	Title string
	Width float32
}{
	{Title: "Статус", Width: 145},
	{Title: "№", Width: 65},
	{Title: "Пользователь", Width: 185},
	{Title: "Информационная база", Width: 220},
	{Title: "Приложение", Width: 175},
	{Title: "Компьютер / IP", Width: 175},
	{Title: "Начало сеанса", Width: 155},
	{Title: "Последняя активность", Width: 185},
}

// Controller manages the main window state and interactions.
type Controller struct {
	app fyne.App
	win fyne.Window
	cfg config.Config

	mu          sync.RWMutex
	allSessions []onec.Session
	filtered    []onec.Session
	stats       onec.SessionStats
	busy        bool

	// Credentials widgets
	credsEditorBox  *fyne.Container
	credsSummaryBox *fyne.Container
	summaryLabel    *widget.Label
	modeSelect      *widget.Select
	serverEntry     *widget.Entry
	racRow          *fyne.Container
	racEntry        *widget.Entry
	userEntry       *widget.Entry
	passEntry       *widget.Entry
	groupSelect     *widget.Select
	protectDesCheck *widget.Check
	confirmCheck    *widget.Check
	cancelCredsBtn  *widget.Button

	// Action & filter widgets
	showBtn         *widget.Button
	deleteOldBtn    *widget.Button
	filterEntry     *widget.Entry
	onlyDupesCheck  *widget.Check
	progressSpinner *widget.ProgressBarInfinite
	statusLabel     *widget.Label

	// Counters
	totalStatLbl  *widget.Label
	usersStatLbl  *widget.Label
	dupesStatLbl  *widget.Label
	systemStatLbl *widget.Label

	// Table & Log
	table      *widget.Table
	emptyHint  *fyne.Container
	tableStack *fyne.Container
	logLines   []string
	logEntry   *widget.Entry
}

// NewController initializes the UI controller with persisted settings.
func NewController(app fyne.App, win fyne.Window) *Controller {
	cfg, loadErr := config.Load()
	discoveredRAC := onec.DiscoverRACPaths()
	if strings.TrimSpace(cfg.RACPath) == "" && len(discoveredRAC) > 0 {
		cfg.RACPath = discoveredRAC[0]
	}

	c := &Controller{
		app: app,
		win: win,
		cfg: cfg,
	}
	c.buildUI(discoveredRAC, loadErr)
	return c
}

func (c *Controller) appendLog(format string, args ...any) {
	ts := time.Now().Format("15:04:05")
	msg := fmt.Sprintf("[%s] %s", ts, fmt.Sprintf(format, args...))
	c.logLines = append(c.logLines, msg)
	if len(c.logLines) > 300 {
		c.logLines = c.logLines[len(c.logLines)-300:]
	}
	if c.logEntry != nil {
		c.logEntry.SetText(strings.Join(c.logLines, "\n"))
		c.logEntry.CursorRow = len(c.logLines) - 1
	}
}

func (c *Controller) syncConfigFromForm() {
	if c.modeSelect != nil {
		if c.modeSelect.Selected == modeOptionCOM {
			c.cfg.ConnectionMode = config.ModeCOM
		} else {
			c.cfg.ConnectionMode = config.ModeRAC
		}
	}
	if c.serverEntry != nil {
		c.cfg.ServerAddr = strings.TrimSpace(c.serverEntry.Text)
	}
	if c.racEntry != nil {
		c.cfg.RACPath = strings.TrimSpace(c.racEntry.Text)
	}
	if c.userEntry != nil {
		c.cfg.ClusterUser = strings.TrimSpace(c.userEntry.Text)
	}
	if c.passEntry != nil {
		c.cfg.ClusterPassword = c.passEntry.Text
	}
	if c.groupSelect != nil {
		c.cfg.GroupByServer = c.groupSelect.Selected == groupOptionServer
	}
	if c.protectDesCheck != nil {
		c.cfg.ProtectDesigner = c.protectDesCheck.Checked
	}
	if c.confirmCheck != nil {
		c.cfg.ConfirmBeforeDelete = c.confirmCheck.Checked
	}
}

func (c *Controller) updateSummaryBar() {
	modeShort := "RAC/RAS"
	addr := c.cfg.EffectiveRASAddr()
	if c.cfg.ConnectionMode == config.ModeCOM {
		modeShort = "COM"
		addr = c.cfg.EffectiveAgentAddr()
	}
	adminDisplay := c.cfg.ClusterUser
	if strings.TrimSpace(adminDisplay) == "" {
		adminDisplay = "без пароля кластера"
	}
	c.summaryLabel.SetText(fmt.Sprintf("Сервер 1С: %s (%s)   •   Администратор: %s", addr, modeShort, adminDisplay))
}

func (c *Controller) setCredentialsExpanded(expanded bool) {
	if expanded {
		c.credsEditorBox.Show()
		c.credsSummaryBox.Hide()
	} else {
		c.updateSummaryBar()
		c.credsEditorBox.Hide()
		c.credsSummaryBox.Show()
	}
}

func (c *Controller) saveCredentials(collapseAfter bool) bool {
	c.syncConfigFromForm()
	if strings.TrimSpace(c.cfg.ServerAddr) == "" {
		dialog.ShowError(fmt.Errorf("укажите адрес сервера 1С (например, localhost:1545)"), c.win)
		return false
	}
	if err := c.cfg.Save(); err != nil {
		c.appendLog("Ошибка сохранения учётных данных: %v", err)
		dialog.ShowError(err, c.win)
		return false
	}
	if c.cancelCredsBtn != nil {
		c.cancelCredsBtn.Show()
	}
	c.updateSummaryBar()
	if collapseAfter {
		c.setCredentialsExpanded(false)
	}
	return true
}

func (c *Controller) setBusy(busy bool, status string) {
	c.mu.Lock()
	c.busy = busy
	c.mu.Unlock()

	c.statusLabel.SetText(status)
	if busy {
		c.showBtn.Disable()
		c.deleteOldBtn.Disable()
		c.progressSpinner.Show()
		c.progressSpinner.Start()
	} else {
		c.progressSpinner.Stop()
		c.progressSpinner.Hide()
		c.showBtn.Enable()
		c.deleteOldBtn.Enable()
	}
}

func (c *Controller) applyFilterAndRefresh() {
	c.mu.Lock()
	query := strings.ToLower(strings.TrimSpace(c.filterEntry.Text))
	onlyDupes := c.onlyDupesCheck.Checked

	filtered := make([]onec.Session, 0, len(c.allSessions))
	for _, s := range c.allSessions {
		if onlyDupes && s.Status != onec.StatusDuplicateOld {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(strings.Join([]string{
				s.UserDisplay(),
				s.InfobaseDisplay(),
				s.AppDisplayName(),
				s.HostDisplay(),
				fmt.Sprintf("%d", s.SessionNumber),
				s.Status.Label(),
			}, " "))
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		filtered = append(filtered, s)
	}
	c.filtered = filtered
	stats := c.stats
	c.mu.Unlock()

	c.totalStatLbl.SetText(fmt.Sprintf("Запущено сеансов: %d", stats.Total))
	c.usersStatLbl.SetText(fmt.Sprintf("Пользователей: %d", stats.UniqueUsers))
	c.dupesStatLbl.SetText(fmt.Sprintf("Старых дублей: %d", stats.DuplicatesOld))
	c.systemStatLbl.SetText(fmt.Sprintf("Системных / защищённых: %d", stats.SystemOrProtected))

	if stats.DuplicatesOld > 0 {
		c.deleteOldBtn.SetText(fmt.Sprintf("Удалить старые (%d)", stats.DuplicatesOld))
	} else {
		c.deleteOldBtn.SetText("Удалить старые")
	}

	if c.emptyHint != nil {
		if len(filtered) == 0 {
			c.emptyHint.Show()
		} else {
			c.emptyHint.Hide()
		}
	}
	if c.table != nil {
		c.table.Refresh()
	}
}

func (c *Controller) reclassifyLoadedSessions() {
	c.syncConfigFromForm()
	if c.cfg.Configured {
		_ = c.cfg.Save()
	}

	c.mu.Lock()
	raw := make([]onec.Session, len(c.allSessions))
	copy(raw, c.allSessions)
	c.mu.Unlock()

	if len(raw) == 0 {
		return
	}
	classified, stats := onec.ClassifySessions(raw, onec.DedupeOptions{
		GroupByServer:   c.cfg.GroupByServer,
		ProtectDesigner: c.cfg.ProtectDesigner,
	})
	c.mu.Lock()
	c.allSessions = classified
	c.stats = stats
	c.mu.Unlock()

	c.applyFilterAndRefresh()
}

func (c *Controller) handleShowSessions() {
	if c.credsEditorBox.Visible() {
		if !c.saveCredentials(true) {
			return
		}
		c.appendLog("Учётные данные администратора сохранены.")
	} else {
		c.syncConfigFromForm()
	}

	cfgSnapshot := c.cfg
	c.setBusy(true, "Загрузка списка запущенных сеансов с сервера 1С...")
	c.appendLog("Запрос списка сеансов (%s)...", cfgSnapshot.ServerAddr)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		client := onec.NewClient(cfgSnapshot)
		sessions, stats, err := client.FetchAndClassify(ctx)

		fyne.Do(func() {
			if err != nil {
				c.setBusy(false, "Ошибка подключения к серверу 1С")
				c.appendLog("ОШИБКА: %v", err)
				dialog.ShowError(err, c.win)
				return
			}

			c.mu.Lock()
			c.allSessions = sessions
			c.stats = stats
			c.mu.Unlock()

			c.applyFilterAndRefresh()
			c.setBusy(false, fmt.Sprintf(
				"Обновлено в %s — всего сеансов: %d, старых дублей: %d",
				time.Now().Format("15:04:05"),
				stats.Total,
				stats.DuplicatesOld,
			))
			c.appendLog(
				"Получено сеансов: %d (уникальных пользователей: %d, старых задублированных: %d).",
				stats.Total,
				stats.UniqueUsers,
				stats.DuplicatesOld,
			)
		})
	}()
}

func (c *Controller) handleDeleteOldSessions() {
	if c.credsEditorBox.Visible() {
		if !c.saveCredentials(true) {
			return
		}
		c.appendLog("Учётные данные администратора сохранены.")
	} else {
		c.syncConfigFromForm()
	}

	if c.cfg.ConfirmBeforeDelete {
		c.mu.RLock()
		dupCount := c.stats.DuplicatesOld
		c.mu.RUnlock()

		msg := "Завершить все старые задублированные сеансы по каждому пользователю, оставив только максимально актуальные?"
		if dupCount > 0 {
			msg = fmt.Sprintf("Найдено старых дублирующихся сеансов: %d.\nУдалить их, оставив по каждому пользователю только максимально актуальный сеанс?", dupCount)
		}
		dialog.ShowConfirm("Подтверждение удаления", msg, func(ok bool) {
			if ok {
				c.executeDeleteOldSessions()
			}
		}, c.win)
		return
	}

	c.executeDeleteOldSessions()
}

func (c *Controller) executeDeleteOldSessions() {
	cfgSnapshot := c.cfg
	c.setBusy(true, "Поиск и завершение старых дублирующихся сеансов...")
	c.appendLog("Запуск очистки старых задублированных сеансов...")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		client := onec.NewClient(cfgSnapshot)
		report, err := client.CleanupOldDuplicates(ctx, func(idx, total int, s onec.Session) {
			fyne.Do(func() {
				c.statusLabel.SetText(fmt.Sprintf(
					"Удаление старого сеанса %d из %d: %s (#%d)...",
					idx, total, s.UserDisplay(), s.SessionNumber,
				))
			})
		})

		fyne.Do(func() {
			if err != nil {
				c.setBusy(false, "Ошибка при удалении старых сеансов")
				c.appendLog("ОШИБКА очистки: %v", err)
				dialog.ShowError(err, c.win)
				return
			}

			for _, del := range report.Deleted {
				c.appendLog(
					"Удалён старый сеанс #%d — пользователь: %s | база: %s | старт: %s | активн.: %s",
					del.SessionNumber,
					del.UserDisplay(),
					del.InfobaseDisplay(),
					del.StartedAtDisplay(),
					del.LastActiveDisplay(),
				)
			}
			for _, fail := range report.Failed {
				c.appendLog(
					"НЕ УДАЛОСЬ завершить сеанс #%d (%s): %v",
					fail.Session.SessionNumber,
					fail.Session.UserDisplay(),
					fail.Err,
				)
			}

			if report.Remaining != nil {
				_, newStats := onec.ClassifySessions(report.Remaining, onec.DedupeOptions{
					GroupByServer:   cfgSnapshot.GroupByServer,
					ProtectDesigner: cfgSnapshot.ProtectDesigner,
				})
				c.mu.Lock()
				c.allSessions = report.Remaining
				c.stats = newStats
				c.mu.Unlock()
				c.applyFilterAndRefresh()
			}

			if len(report.Deleted) == 0 && len(report.Failed) == 0 {
				c.setBusy(false, "Дублирующихся старых сеансов не обнаружено — у всех пользователей только актуальные сеансы")
				c.appendLog("Очистка завершена: старых задублированных сеансов не найдено.")
				dialog.ShowInformation(
					"Дубликатов нет",
					"Старых задублированных сеансов не обнаружено.\nУ каждого пользователя оставлен только актуальный сеанс.",
					c.win,
				)
				return
			}

			summary := fmt.Sprintf("Завершено старых сеансов: %d", len(report.Deleted))
			if len(report.Failed) > 0 {
				summary += fmt.Sprintf(" (ошибок: %d — см. журнал)", len(report.Failed))
			}
			c.setBusy(false, summary)
			c.appendLog("Итог очистки: %s.", summary)
		})
	}()
}

func (c *Controller) buildCredentialsSection(discoveredRAC []string) fyne.CanvasObject {
	// 1. Compact summary bar when credentials are already remembered
	c.summaryLabel = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	editCredsBtn := widget.NewButtonWithIcon("Изменить учётные данные", theme.SettingsIcon(), func() {
		c.setCredentialsExpanded(true)
	})
	aboutBtn := widget.NewButtonWithIcon("О программе", theme.InfoIcon(), func() {
		ShowDeveloperDialog(c.app, c.win)
	})
	c.credsSummaryBox = container.NewVBox(
		container.NewHBox(
			c.summaryLabel,
			layout.NewSpacer(),
			editCredsBtn,
			aboutBtn,
		),
		widget.NewSeparator(),
	)

	// 2. Full credentials editor (shown once on first launch, or on demand)
	c.serverEntry = widget.NewEntry()
	c.serverEntry.SetPlaceHolder("localhost:1545 (адрес сервера RAS или агента 1С)")
	c.serverEntry.SetText(c.cfg.ServerAddr)

	c.racEntry = widget.NewEntry()
	c.racEntry.SetPlaceHolder(`C:\Program Files\1cv8\8.3.xx.xxxx\bin\rac.exe (определяется автоматически)`)
	c.racEntry.SetText(c.cfg.RACPath)

	browseRACBtn := widget.NewButtonWithIcon("Обзор...", theme.FolderOpenIcon(), func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			c.racEntry.SetText(reader.URI().Path())
		}, c.win)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".exe"}))
		fd.Show()
	})

	racControls := []fyne.CanvasObject{browseRACBtn}
	if len(discoveredRAC) > 1 {
		verSelect := widget.NewSelect(discoveredRAC, func(chosen string) {
			if chosen != "" {
				c.racEntry.SetText(chosen)
			}
		})
		verSelect.PlaceHolder = "Выбрать версию 1С"
		racControls = append([]fyne.CanvasObject{verSelect}, racControls...)
	}

	c.racRow = container.NewBorder(
		nil, nil,
		widget.NewLabel("Путь к rac.exe:"),
		container.NewHBox(racControls...),
		c.racEntry,
	)

	c.modeSelect = widget.NewSelect([]string{modeOptionRAC, modeOptionCOM}, nil)
	if c.cfg.ConnectionMode == config.ModeCOM {
		c.modeSelect.SetSelected(modeOptionCOM)
		c.racRow.Hide()
	} else {
		c.modeSelect.SetSelected(modeOptionRAC)
	}
	c.modeSelect.OnChanged = func(selected string) {
		if selected == modeOptionCOM {
			c.racRow.Hide()
			cur := strings.TrimSpace(c.serverEntry.Text)
			if strings.HasSuffix(cur, ":1545") {
				c.serverEntry.SetText(strings.TrimSuffix(cur, ":1545") + ":1540")
			}
		} else {
			c.racRow.Show()
			cur := strings.TrimSpace(c.serverEntry.Text)
			if strings.HasSuffix(cur, ":1540") {
				c.serverEntry.SetText(strings.TrimSuffix(cur, ":1540") + ":1545")
			}
		}
	}

	c.userEntry = widget.NewEntry()
	c.userEntry.SetPlaceHolder("Администратор кластера 1С (оставьте пустым, если не задан)")
	c.userEntry.SetText(c.cfg.ClusterUser)

	c.passEntry = widget.NewPasswordEntry()
	c.passEntry.SetPlaceHolder("Пароль администратора кластера 1С")
	c.passEntry.SetText(c.cfg.ClusterPassword)

	c.groupSelect = widget.NewSelect([]string{groupOptionPerIB, groupOptionServer}, nil)
	if c.cfg.GroupByServer {
		c.groupSelect.SetSelected(groupOptionServer)
	} else {
		c.groupSelect.SetSelected(groupOptionPerIB)
	}

	c.protectDesCheck = widget.NewCheck("Не завершать Конфигуратор (Designer)", nil)
	c.protectDesCheck.SetChecked(c.cfg.ProtectDesigner)

	c.confirmCheck = widget.NewCheck("Запрашивать подтверждение перед удалением", nil)
	c.confirmCheck.SetChecked(c.cfg.ConfirmBeforeDelete)

	c.groupSelect.OnChanged = func(_ string) {
		c.reclassifyLoadedSessions()
	}
	c.protectDesCheck.OnChanged = func(_ bool) {
		c.reclassifyLoadedSessions()
	}
	c.confirmCheck.OnChanged = func(_ bool) {
		c.syncConfigFromForm()
		if c.cfg.Configured {
			_ = c.cfg.Save()
		}
	}

	c.cancelCredsBtn = widget.NewButton("Свернуть настройки", func() {
		c.setCredentialsExpanded(false)
	})
	if !c.cfg.Configured {
		c.cancelCredsBtn.Hide()
	}

	saveBtn := widget.NewButtonWithIcon("Сохранить и запомнить учётные данные", theme.DocumentSaveIcon(), func() {
		if c.saveCredentials(true) {
			c.appendLog("Учётные данные администратора сохранены и зашифрованы.")
		}
	})
	saveBtn.Importance = widget.HighImportance

	aboutEditorBtn := widget.NewButtonWithIcon("О программе", theme.InfoIcon(), func() {
		ShowDeveloperDialog(c.app, c.win)
	})

	connGrid := container.NewGridWithColumns(2,
		container.NewBorder(nil, nil, widget.NewLabel("Режим:"), nil, c.modeSelect),
		container.NewBorder(nil, nil, widget.NewLabel("Сервер 1С:"), nil, c.serverEntry),
	)

	authGrid := container.NewGridWithColumns(2,
		container.NewBorder(nil, nil, widget.NewLabel("Логин адм.:"), nil, c.userEntry),
		container.NewBorder(nil, nil, widget.NewLabel("Пароль адм.:"), nil, c.passEntry),
	)

	optsRow := container.NewHBox(
		widget.NewLabel("Группировка по пользователю:"),
		c.groupSelect,
		c.protectDesCheck,
		c.confirmCheck,
	)

	btnRow := container.NewHBox(
		saveBtn,
		c.cancelCredsBtn,
		layout.NewSpacer(),
		aboutEditorBtn,
	)

	cardTitle := widget.NewLabelWithStyle(
		"Учётные данные администратора сервера 1С (вводятся один раз — программа их запоминает)",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)

	c.credsEditorBox = container.NewVBox(
		cardTitle,
		connGrid,
		c.racRow,
		authGrid,
		optsRow,
		btnRow,
		widget.NewSeparator(),
	)

	c.setCredentialsExpanded(!c.cfg.Configured)

	return container.NewVBox(c.credsSummaryBox, c.credsEditorBox)
}

func (c *Controller) buildActionAndStatsSection() fyne.CanvasObject {
	c.showBtn = widget.NewButtonWithIcon("Показать сеансы", theme.ViewRefreshIcon(), func() {
		c.handleShowSessions()
	})
	c.showBtn.Importance = widget.HighImportance

	c.deleteOldBtn = widget.NewButtonWithIcon("Удалить старые", theme.DeleteIcon(), func() {
		c.handleDeleteOldSessions()
	})
	c.deleteOldBtn.Importance = widget.DangerImportance

	c.filterEntry = widget.NewEntry()
	c.filterEntry.SetPlaceHolder("Поиск по пользователю, базе, компьютеру...")
	c.filterEntry.OnChanged = func(_ string) {
		c.applyFilterAndRefresh()
	}

	c.onlyDupesCheck = widget.NewCheck("Только старые дубли", func(_ bool) {
		c.applyFilterAndRefresh()
	})

	c.progressSpinner = widget.NewProgressBarInfinite()
	c.progressSpinner.Hide()

	c.statusLabel = widget.NewLabel("Готово к работе. Нажмите «Показать сеансы» для загрузки активных сеансов 1С.")

	actionBar := container.NewBorder(
		nil, nil,
		container.NewHBox(c.showBtn, c.deleteOldBtn),
		c.onlyDupesCheck,
		container.NewPadded(c.filterEntry),
	)

	c.totalStatLbl = widget.NewLabelWithStyle("Запущено сеансов: 0", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	c.usersStatLbl = widget.NewLabel("Пользователей: 0")
	c.dupesStatLbl = widget.NewLabelWithStyle("Старых дублей: 0", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	c.systemStatLbl = widget.NewLabel("Системных / защищённых: 0")

	statsRow := container.NewHBox(
		c.totalStatLbl,
		widget.NewLabel("  |  "),
		c.usersStatLbl,
		widget.NewLabel("  |  "),
		c.dupesStatLbl,
		widget.NewLabel("  |  "),
		c.systemStatLbl,
		layout.NewSpacer(),
		c.statusLabel,
	)

	return container.NewVBox(
		actionBar,
		c.progressSpinner,
		statsRow,
		widget.NewSeparator(),
	)
}

func (c *Controller) buildSessionsTable() fyne.CanvasObject {
	c.table = widget.NewTable(
		func() (int, int) {
			c.mu.RLock()
			defer c.mu.RUnlock()
			return len(c.filtered), len(tableHeaders)
		},
		func() fyne.CanvasObject {
			bg := canvas.NewRectangle(color.Transparent)
			lbl := widget.NewLabel("")
			lbl.Truncation = fyne.TextTruncateEllipsis
			return container.NewStack(bg, lbl)
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			stack := cell.(*fyne.Container)
			bg := stack.Objects[0].(*canvas.Rectangle)
			lbl := stack.Objects[1].(*widget.Label)

			c.mu.RLock()
			if id.Row < 0 || id.Row >= len(c.filtered) {
				c.mu.RUnlock()
				lbl.SetText("")
				bg.FillColor = color.Transparent
				bg.Refresh()
				return
			}
			s := c.filtered[id.Row]
			c.mu.RUnlock()

			variant := c.app.Settings().ThemeVariant()
			lbl.TextStyle = fyne.TextStyle{}

			switch id.Col {
			case 0:
				lbl.SetText(s.Status.Label())
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				badgeBg, _ := statusBadgeColors(s.Status, variant)
				bg.FillColor = badgeBg
			case 1:
				lbl.SetText(fmt.Sprintf("%d", s.SessionNumber))
			case 2:
				lbl.SetText(s.UserDisplay())
				if s.Status == onec.StatusCurrent {
					lbl.TextStyle = fyne.TextStyle{Bold: true}
				}
			case 3:
				lbl.SetText(s.InfobaseDisplay())
			case 4:
				lbl.SetText(s.AppDisplayName())
			case 5:
				lbl.SetText(s.HostDisplay())
			case 6:
				lbl.SetText(s.StartedAtDisplay())
			case 7:
				lbl.SetText(s.LastActiveDisplay())
			}

			if id.Col != 0 {
				if s.Status == onec.StatusDuplicateOld {
					if variant == theme.VariantDark {
						bg.FillColor = color.NRGBA{R: 145, G: 28, B: 28, A: 55}
					} else {
						bg.FillColor = color.NRGBA{R: 254, G: 226, B: 226, A: 95}
					}
				} else if id.Row%2 == 1 {
					if variant == theme.VariantDark {
						bg.FillColor = color.NRGBA{R: 255, G: 255, B: 255, A: 10}
					} else {
						bg.FillColor = color.NRGBA{R: 0, G: 0, B: 0, A: 8}
					}
				} else {
					bg.FillColor = color.Transparent
				}
			}

			bg.Refresh()
			lbl.Refresh()
		},
	)

	c.table.ShowHeaderRow = true
	c.table.ShowHeaderColumn = false
	c.table.CreateHeader = func() fyne.CanvasObject {
		lbl := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		lbl.Truncation = fyne.TextTruncateEllipsis
		return lbl
	}
	c.table.UpdateHeader = func(id widget.TableCellID, template fyne.CanvasObject) {
		lbl := template.(*widget.Label)
		if id.Col >= 0 && id.Col < len(tableHeaders) {
			lbl.SetText(tableHeaders[id.Col].Title)
		} else {
			lbl.SetText("")
		}
	}
	for i, h := range tableHeaders {
		c.table.SetColumnWidth(i, h.Width)
	}

	emptyLbl := widget.NewLabelWithStyle(
		"Список сеансов пуст.\nНажмите «Показать сеансы», чтобы отобразить все запущенные сеансы на сервере 1С.",
		fyne.TextAlignCenter,
		fyne.TextStyle{},
	)
	c.emptyHint = container.NewCenter(emptyLbl)

	c.tableStack = container.NewStack(c.table, c.emptyHint)
	return c.tableStack
}

func (c *Controller) buildLogPanel() fyne.CanvasObject {
	c.logEntry = widget.NewMultiLineEntry()
	c.logEntry.Wrapping = fyne.TextWrapWord
	c.logEntry.SetMinRowsVisible(4)

	clearBtn := widget.NewButtonWithIcon("Очистить журнал", theme.ContentClearIcon(), func() {
		c.logLines = nil
		c.logEntry.SetText("")
	})
	clearBtn.Importance = widget.LowImportance

	header := container.NewHBox(
		widget.NewLabelWithStyle("Журнал операций:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		layout.NewSpacer(),
		clearBtn,
	)

	return container.NewBorder(header, nil, nil, nil, c.logEntry)
}

func (c *Controller) buildUI(discoveredRAC []string, loadErr error) {
	topPanel := container.NewVBox(
		c.buildCredentialsSection(discoveredRAC),
		c.buildActionAndStatsSection(),
	)

	centerSplit := container.NewVSplit(
		c.buildSessionsTable(),
		c.buildLogPanel(),
	)
	centerSplit.Offset = 0.76

	footer := BuildFooterBar(c.app, c.win)

	root := container.NewBorder(
		container.NewPadded(topPanel),
		footer,
		nil,
		nil,
		container.NewPadded(centerSplit),
	)

	c.win.SetContent(root)

	if loadErr != nil {
		c.appendLog("Предупреждение при чтении файла настроек: %v", loadErr)
	}
	if len(discoveredRAC) > 0 {
		c.appendLog("Обнаружена утилита 1С rac: %s", discoveredRAC[0])
	}
	if c.cfg.Configured {
		c.appendLog("Загружены сохранённые учётные данные администратора для сервера %s.", c.cfg.ServerAddr)
	} else {
		c.appendLog("Первый запуск: введите адрес сервера 1С и учётные данные администратора (сохраняются автоматически).")
	}
}
