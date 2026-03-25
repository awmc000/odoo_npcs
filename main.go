package main

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/playwright-community/playwright-go"
)

type SimUser struct {
	ID    string
	Name  string
	Group string
}

type BlockKind int

const (
	BlockUnavailable BlockKind = iota
	BlockTask
)

type ScheduleBlock struct {
	ID       string
	UserID   string
	Title    string
	StartMin int
	EndMin   int
	Kind     BlockKind
}

type ScheduleModel struct {
	StartMin    int
	EndMin      int
	SlotMinutes int
	Users       []SimUser
	Blocks      []ScheduleBlock
}

type ScheduleWidget struct {
	widget.BaseWidget
	model        *ScheduleModel
	visibleUsers []SimUser
	onEmptyTap   func(user SimUser, minute int)
	onBlockTap   func(block ScheduleBlock)
}

func NewScheduleWidget(model *ScheduleModel, users []SimUser) *ScheduleWidget {
	sw := &ScheduleWidget{model: model, visibleUsers: users}
	sw.ExtendBaseWidget(sw)
	return sw
}

func (sw *ScheduleWidget) SetVisibleUsers(users []SimUser) {
	sw.visibleUsers = users
	sw.Refresh()
}

func (sw *ScheduleWidget) SetHandlers(onEmpty func(user SimUser, minute int), onBlock func(block ScheduleBlock)) {
	sw.onEmptyTap = onEmpty
	sw.onBlockTap = onBlock
}

func (sw *ScheduleWidget) MinSize() fyne.Size {
	columns := len(sw.visibleUsers)
	if columns < 1 {
		columns = 1
	}

	hours := float32(sw.model.EndMin-sw.model.StartMin) / 60.0
	return fyne.NewSize(92+float32(columns)*160, 74+hours*110)
}

func (sw *ScheduleWidget) Tapped(ev *fyne.PointEvent) {
	geom := sw.geometry(sw.Size())
	x := ev.Position.X
	y := ev.Position.Y

	if x < geom.gridX || x > geom.gridX+geom.gridW || y < geom.gridY || y > geom.gridY+geom.gridH {
		return
	}

	col := int((x - geom.gridX) / geom.colW)
	if col < 0 || col >= len(sw.visibleUsers) {
		return
	}

	minute := sw.model.StartMin + int((y-geom.gridY)/geom.minuteH)
	minute = snapDown(minute, sw.model.SlotMinutes)

	user := sw.visibleUsers[col]
	for _, b := range sw.model.Blocks {
		if b.UserID != user.ID {
			continue
		}
		if minute >= b.StartMin && minute < b.EndMin {
			if sw.onBlockTap != nil {
				sw.onBlockTap(b)
			}
			return
		}
	}

	if sw.onEmptyTap != nil {
		sw.onEmptyTap(user, minute)
	}
}

func (sw *ScheduleWidget) TappedSecondary(*fyne.PointEvent) {}

func (sw *ScheduleWidget) CreateRenderer() fyne.WidgetRenderer {
	r := &scheduleRenderer{sw: sw}
	r.Refresh()
	return r
}

type scheduleGeometry struct {
	outerW, outerH float32
	gridX, gridY   float32
	gridW, gridH   float32
	colW, minuteH  float32
	leftW, topH    float32
}

func (sw *ScheduleWidget) geometry(size fyne.Size) scheduleGeometry {
	columns := len(sw.visibleUsers)
	if columns < 1 {
		columns = 1
	}

	leftW := float32(92)
	topH := float32(74)
	contentW := float32(columns) * 160
	contentH := float32(sw.model.EndMin-sw.model.StartMin) * 1.8

	outerW := maxf(size.Width, leftW+contentW)
	outerH := maxf(size.Height, topH+contentH)

	gridW := outerW - leftW
	gridH := outerH - topH
	minuteH := gridH / float32(sw.model.EndMin-sw.model.StartMin)
	colW := gridW / float32(columns)

	return scheduleGeometry{
		outerW:  outerW,
		outerH:  outerH,
		gridX:   leftW,
		gridY:   topH,
		gridW:   gridW,
		gridH:   gridH,
		colW:    colW,
		minuteH: minuteH,
		leftW:   leftW,
		topH:    topH,
	}
}

type scheduleRenderer struct {
	sw      *ScheduleWidget
	size    fyne.Size
	objects []fyne.CanvasObject
}

func (r *scheduleRenderer) Layout(size fyne.Size) {
	r.size = size
	r.Refresh()
}

func (r *scheduleRenderer) MinSize() fyne.Size {
	return r.sw.MinSize()
}

func (r *scheduleRenderer) Refresh() {
	size := r.size
	if size.Width == 0 || size.Height == 0 {
		size = r.sw.MinSize()
	}
	g := r.sw.geometry(size)
	var objs []fyne.CanvasObject

	bg := canvas.NewRectangle(color.NRGBA{R: 250, G: 251, B: 253, A: 255})
	bg.Resize(fyne.NewSize(g.outerW, g.outerH))
	objs = append(objs, bg)

	topBg := canvas.NewRectangle(color.NRGBA{R: 245, G: 247, B: 250, A: 255})
	topBg.Move(fyne.NewPos(0, 0))
	topBg.Resize(fyne.NewSize(g.outerW, g.topH))
	objs = append(objs, topBg)

	leftBg := canvas.NewRectangle(color.NRGBA{R: 245, G: 247, B: 250, A: 255})
	leftBg.Move(fyne.NewPos(0, g.topH))
	leftBg.Resize(fyne.NewSize(g.leftW, g.gridH))
	objs = append(objs, leftBg)

	cornerBg := canvas.NewRectangle(color.NRGBA{R: 235, G: 238, B: 242, A: 255})
	cornerBg.Resize(fyne.NewSize(g.leftW, g.topH))
	objs = append(objs, cornerBg)

	for m := r.sw.model.StartMin; m <= r.sw.model.EndMin; m += r.sw.model.SlotMinutes {
		y := g.gridY + float32(m-r.sw.model.StartMin)*g.minuteH
		lineColor := color.NRGBA{R: 229, G: 233, B: 240, A: 255}
		if m%60 == 0 {
			lineColor = color.NRGBA{R: 215, G: 221, B: 230, A: 255}
		}
		line := canvas.NewLine(lineColor)
		line.Position1 = fyne.NewPos(g.gridX, y)
		line.Position2 = fyne.NewPos(g.gridX+g.gridW, y)
		objs = append(objs, line)

		if m%60 == 0 {
			label := canvas.NewText(formatClock(m), color.NRGBA{R: 84, G: 93, B: 105, A: 255})
			label.TextSize = 13
			label.Move(fyne.NewPos(12, y-8))
			objs = append(objs, label)
		}
	}

	for i, u := range r.sw.visibleUsers {
		x := g.gridX + float32(i)*g.colW

		vline := canvas.NewLine(color.NRGBA{R: 215, G: 221, B: 230, A: 255})
		vline.Position1 = fyne.NewPos(x, g.gridY)
		vline.Position2 = fyne.NewPos(x, g.gridY+g.gridH)
		objs = append(objs, vline)

		avatar := canvas.NewRectangle(color.NRGBA{R: 221, G: 228, B: 239, A: 255})
		avatar.Move(fyne.NewPos(x+g.colW/2-18, 10))
		avatar.Resize(fyne.NewSize(36, 36))
		objs = append(objs, avatar)

		initials := canvas.NewText(userInitials(u.Name), color.NRGBA{R: 53, G: 69, B: 92, A: 255})
		initials.TextStyle = fyne.TextStyle{Bold: true}
		initials.TextSize = 15
		initials.Move(fyne.NewPos(x+g.colW/2-10, 18))
		objs = append(objs, initials)

		name := canvas.NewText(strings.ToUpper(shortName(u.Name)), color.NRGBA{R: 84, G: 93, B: 105, A: 255})
		name.TextSize = 13
		name.Alignment = fyne.TextAlignCenter
		name.Move(fyne.NewPos(x+8, 50))
		name.Resize(fyne.NewSize(g.colW-16, 20))
		objs = append(objs, name)
	}

	endLine := canvas.NewLine(color.NRGBA{R: 215, G: 221, B: 230, A: 255})
	endLine.Position1 = fyne.NewPos(g.gridX+g.gridW, g.gridY)
	endLine.Position2 = fyne.NewPos(g.gridX+g.gridW, g.gridY+g.gridH)
	objs = append(objs, endLine)

	sorted := append([]ScheduleBlock(nil), r.sw.model.Blocks...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].UserID == sorted[j].UserID {
			return sorted[i].StartMin < sorted[j].StartMin
		}
		return sorted[i].UserID < sorted[j].UserID
	})

	userColumn := map[string]int{}
	for i, u := range r.sw.visibleUsers {
		userColumn[u.ID] = i
	}

	for _, b := range sorted {
		col, ok := userColumn[b.UserID]
		if !ok {
			continue
		}

		x := g.gridX + float32(col)*g.colW + 5
		y := g.gridY + float32(b.StartMin-r.sw.model.StartMin)*g.minuteH + 4
		h := float32(b.EndMin-b.StartMin)*g.minuteH - 8
		if h < 24 {
			h = 24
		}
		w := g.colW - 10

		fill, stroke, txt := blockStyle(b.Kind)

		rect := canvas.NewRectangle(fill)
		rect.CornerRadius = 8
		rect.Move(fyne.NewPos(x, y))
		rect.Resize(fyne.NewSize(w, h))
		objs = append(objs, rect)

		border := canvas.NewRectangle(color.NRGBA{A: 0})
		border.StrokeColor = stroke
		border.StrokeWidth = 1
		border.CornerRadius = 8
		border.Move(fyne.NewPos(x, y))
		border.Resize(fyne.NewSize(w, h))
		objs = append(objs, border)

		if b.Title != "" {
			title := canvas.NewText(strings.ToUpper(b.Title), txt)
			title.TextStyle = fyne.TextStyle{Bold: true}
			title.TextSize = 14
			title.Move(fyne.NewPos(x+10, y+10))
			objs = append(objs, title)
		}
	}

	hint := canvas.NewText("click empty space to add a task", color.NRGBA{R: 107, G: 114, B: 128, A: 255})
	hint.TextSize = 12
	hint.Move(fyne.NewPos(10, g.outerH-20))
	objs = append(objs, hint)

	if nowY, ok := r.currentTimeY(g); ok {
		nowLine := canvas.NewLine(color.NRGBA{R: 217, G: 119, B: 6, A: 255})
		nowLine.StrokeWidth = 2
		nowLine.Position1 = fyne.NewPos(g.gridX, nowY)
		nowLine.Position2 = fyne.NewPos(g.gridX+g.gridW, nowY)
		objs = append(objs, nowLine)

		nowLabel := canvas.NewText("NOW", color.NRGBA{R: 146, G: 64, B: 14, A: 255})
		nowLabel.TextStyle = fyne.TextStyle{Bold: true}
		nowLabel.TextSize = 12
		nowLabel.Move(fyne.NewPos(12, nowY-18))
		objs = append(objs, nowLabel)
	}

	r.objects = objs
}

func (r *scheduleRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *scheduleRenderer) Destroy() {}

func (r *scheduleRenderer) currentTimeY(g scheduleGeometry) (float32, bool) {
	now := time.Now()
	minuteOfDay := float32(now.Hour()*60+now.Minute()) + float32(now.Second())/60 + float32(now.Nanosecond())/float32(time.Minute)
	if minuteOfDay < float32(r.sw.model.StartMin) || minuteOfDay > float32(r.sw.model.EndMin) {
		return 0, false
	}

	return g.gridY + (minuteOfDay-float32(r.sw.model.StartMin))*g.minuteH, true
}

func blockStyle(kind BlockKind) (fill, stroke, txt color.NRGBA) {
	if kind == BlockUnavailable {
		return color.NRGBA{R: 229, G: 231, B: 235, A: 255}, color.NRGBA{R: 209, G: 213, B: 219, A: 255}, color.NRGBA{R: 107, G: 114, B: 128, A: 255}
	}
	return color.NRGBA{R: 213, G: 232, B: 255, A: 255}, color.NRGBA{R: 94, G: 144, B: 230, A: 255}, color.NRGBA{R: 31, G: 41, B: 55, A: 255}
}

func main() {
	a := app.New()
	w := a.NewWindow("Odoo NPCs")
	w.Resize(fyne.NewSize(1420, 900))
	w.SetFixedSize(false)

	model := sampleModel()

	status := widget.NewLabel("Starter view: click an empty slot to create a task block.")
	status.Wrapping = fyne.TextWrapWord

	schedule := NewScheduleWidget(model, model.Users)
	scroll := container.NewScroll(schedule)
	scroll.SetMinSize(fyne.NewSize(400, 200))

	schedule.SetHandlers(
		func(user SimUser, minute int) {
			nameEntry := widget.NewEntry()
			nameEntry.SetText("NEW TASK")

			durationOptions := []string{"15", "30", "45", "60", "90", "120"}
			durationSelect := widget.NewSelect(durationOptions, nil)
			durationSelect.SetSelected("60")

			kindOptions := []string{"Task", "Unavailable"}
			kindSelect := widget.NewSelect(kindOptions, nil)
			kindSelect.SetSelected("Task")

			form := dialog.NewForm(
				fmt.Sprintf("Add block for %s", user.Name),
				"Add",
				"Cancel",
				[]*widget.FormItem{
					widget.NewFormItem("Name", nameEntry),
					widget.NewFormItem("Start", widget.NewLabel(formatClock(minute))),
					widget.NewFormItem("Duration (minutes)", durationSelect),
					widget.NewFormItem("Type", kindSelect),
				},
				func(ok bool) {
					if !ok {
						return
					}

					duration := parseInt(durationSelect.Selected, 60)
					title := strings.TrimSpace(nameEntry.Text)
					if title == "" {
						title = "UNTITLED"
					}

					blockKind := BlockTask
					if kindSelect.Selected == "Unavailable" {
						blockKind = BlockUnavailable
					}

					end := minute + duration
					if end > model.EndMin {
						end = model.EndMin
					}

					model.Blocks = append(model.Blocks, ScheduleBlock{
						ID:       fmt.Sprintf("block-%d", len(model.Blocks)+1),
						UserID:   user.ID,
						Title:    title,
						StartMin: minute,
						EndMin:   end,
						Kind:     blockKind,
					})

					schedule.Refresh()
					status.SetText(fmt.Sprintf("Added %q for %s from %s to %s.", title, user.Name, formatClock(minute), formatClock(end)))
				},
				w,
			)
			form.Resize(fyne.NewSize(420, 260))
			form.Show()
		},
		func(block ScheduleBlock) {
			status.SetText(fmt.Sprintf("%s: %s-%s", displayBlockLabel(block), formatClock(block.StartMin), formatClock(block.EndMin)))
		},
	)

	topBar := container.NewBorder(
		nil,
		nil,
		widget.NewButton("←", func() {
			status.SetText("Hook this up to horizontal paging or future NPC columns.")
		}),
		widget.NewButton("→", func() {
			status.SetText("Hook this up to horizontal paging or previous NPC columns.")
		}),
		widget.NewLabel("Timeline view: scroll vertically to inspect the full day across simulated users."),
	)

	odooURL := widget.NewEntry()
	odooURL.SetPlaceHolder("https://your-odoo-instance.example.com")
	odooURL.SetText("http://localhost:8069")

	odooTestButton := widget.NewButton("Test", nil)
	odooSettingsContent := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("URL", odooURL),
		),
		odooTestButton,
	)
	odooSettingsContent.Hide()

	programRefresh := widget.NewSelect([]string{"15 seconds", "30 seconds", "60 seconds"}, nil)
	programRefresh.SetSelected("30 seconds")
	programTimeFormat := widget.NewSelect([]string{"24-hour", "12-hour"}, nil)
	programTimeFormat.SetSelected("24-hour")
	programSettingsContent := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Refresh cadence", programRefresh),
			widget.NewFormItem("Time format", programTimeFormat),
		),
	)
	programSettingsContent.Hide()

	odooExpanded := false
	programExpanded := false

	var odooSettingsButton *widget.Button
	odooSettingsButton = widget.NewButton("Odoo settings", func() {
		odooExpanded = !odooExpanded
		if odooExpanded {
			odooSettingsContent.Show()
			status.SetText("Odoo settings expanded.")
		} else {
			odooSettingsContent.Hide()
			status.SetText("Odoo settings collapsed.")
		}
		odooSettingsButton.SetText(toggleButtonLabel("Odoo settings", odooExpanded))
	})
	odooSettingsButton.SetText(toggleButtonLabel("Odoo settings", odooExpanded))

	var programSettingsButton *widget.Button
	programSettingsButton = widget.NewButton("Program settings", func() {
		programExpanded = !programExpanded
		if programExpanded {
			programSettingsContent.Show()
			status.SetText("Program settings expanded.")
		} else {
			programSettingsContent.Hide()
			status.SetText("Program settings collapsed.")
		}
		programSettingsButton.SetText(toggleButtonLabel("Program settings", programExpanded))
	})
	programSettingsButton.SetText(toggleButtonLabel("Program settings", programExpanded))

	odooTestButton.OnTapped = func() {
		url := strings.TrimSpace(odooURL.Text)
		if url == "" {
			status.SetText("Enter an Odoo URL before running the Playwright test.")
			return
		}

		odooTestButton.Disable()
		status.SetText(fmt.Sprintf("Testing Odoo URL %s with Playwright...", url))

		go func() {
			err := testURLWithPlaywright(url)
			fyne.Do(func() {
				odooTestButton.Enable()
				if err != nil {
					status.SetText(fmt.Sprintf("Playwright test failed for %s: %s", url, err.Error()))
					return
				}
				status.SetText(fmt.Sprintf("Playwright reached %s successfully.", url))
			})
		}()
	}

	rightPane := container.NewVBox(
		widget.NewLabelWithStyle("Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		odooSettingsButton,
		odooSettingsContent,
		programSettingsButton,
		programSettingsContent,
		widget.NewSeparator(),
	)

	header := container.NewVBox(
		widget.NewLabelWithStyle("Schedule View", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		topBar,
	)

	body := container.NewBorder(
		header,
		container.NewVBox(widget.NewSeparator(), status),
		nil,
		rightPane,
		container.NewPadded(scroll),
	)

	w.SetContent(body)
	w.ShowAndRun()
}

func sampleModel() *ScheduleModel {
	users := []SimUser{
		{ID: "amm", Name: "Ann M.", Group: "Cashier"},
		{ID: "jv", Name: "Jay V.", Group: "Cashier"},
		{ID: "bm", Name: "Bea M.", Group: "Bookkeeper"},
		{ID: "tp", Name: "Terry P.", Group: "Manager"},
	}

	return &ScheduleModel{
		StartMin:    0,
		EndMin:      24*60 - 1,
		SlotMinutes: 15,
		Users:       users,
		Blocks: []ScheduleBlock{
			{ID: "u1", UserID: "amm", StartMin: 8 * 60, EndMin: 10*60 + 30, Kind: BlockTask, Title: "REGISTER"},
			{ID: "u2", UserID: "amm", StartMin: 10*60 + 30, EndMin: 10*60 + 45, Kind: BlockUnavailable},
			{ID: "u3", UserID: "amm", StartMin: 10*60 + 45, EndMin: 13 * 60, Kind: BlockTask, Title: "CHECKOUT"},
			{ID: "u4", UserID: "amm", StartMin: 13 * 60, EndMin: 13*60 + 30, Kind: BlockUnavailable},
			{ID: "u5", UserID: "amm", StartMin: 13*60 + 30, EndMin: 16 * 60, Kind: BlockTask, Title: "CUSTOMER DESK"},

			{ID: "u6", UserID: "jv", StartMin: 9 * 60, EndMin: 11 * 60, Kind: BlockTask, Title: "REGISTER"},
			{ID: "u7", UserID: "jv", StartMin: 11 * 60, EndMin: 11*60 + 15, Kind: BlockUnavailable},
			{ID: "u8", UserID: "jv", StartMin: 11*60 + 15, EndMin: 14 * 60, Kind: BlockTask, Title: "RETURNS"},
			{ID: "u9", UserID: "jv", StartMin: 14 * 60, EndMin: 14*60 + 30, Kind: BlockUnavailable},
			{ID: "u10", UserID: "jv", StartMin: 14*60 + 30, EndMin: 18 * 60, Kind: BlockTask, Title: "CHECKOUT"},

			{ID: "u11", UserID: "bm", StartMin: 8*60 + 30, EndMin: 10 * 60, Kind: BlockTask, Title: "RECONCILIATION"},
			{ID: "u12", UserID: "bm", StartMin: 10 * 60, EndMin: 10*60 + 15, Kind: BlockUnavailable},
			{ID: "u13", UserID: "bm", StartMin: 10*60 + 15, EndMin: 12 * 60, Kind: BlockTask, Title: "AP INVOICES"},
			{ID: "u14", UserID: "bm", StartMin: 13 * 60, EndMin: 15 * 60, Kind: BlockTask, Title: "BANK DEPOSITS"},
			{ID: "u15", UserID: "bm", StartMin: 15 * 60, EndMin: 15*60 + 15, Kind: BlockUnavailable},
			{ID: "u16", UserID: "bm", StartMin: 15*60 + 15, EndMin: 17 * 60, Kind: BlockTask, Title: "LEDGER REVIEW"},

			{ID: "u17", UserID: "tp", StartMin: 7*60 + 45, EndMin: 9 * 60, Kind: BlockTask, Title: "OPENING WALK"},
			{ID: "u18", UserID: "tp", StartMin: 9 * 60, EndMin: 11 * 60, Kind: BlockTask, Title: "FLOOR SUPPORT"},
			{ID: "u19", UserID: "tp", StartMin: 11 * 60, EndMin: 11*60 + 30, Kind: BlockUnavailable},
			{ID: "u20", UserID: "tp", StartMin: 11*60 + 30, EndMin: 13 * 60, Kind: BlockTask, Title: "VENDOR CALLS"},
			{ID: "u21", UserID: "tp", StartMin: 13*60 + 30, EndMin: 15 * 60, Kind: BlockTask, Title: "SCHEDULE REVIEW"},
			{ID: "u22", UserID: "tp", StartMin: 15 * 60, EndMin: 17 * 60, Kind: BlockTask, Title: "CLOSING PREP"},
		},
	}
}

func displayBlockLabel(block ScheduleBlock) string {
	if block.Title != "" {
		return block.Title
	}
	return "Unavailable"
}

func userInitials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "?"
	}
	if len(parts) == 1 {
		return strings.ToUpper(parts[0][:1])
	}
	return strings.ToUpper(parts[0][:1] + parts[len(parts)-1][:1])
}

func shortName(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return name
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + " " + parts[len(parts)-1]
}

func formatClock(minute int) string {
	if minute >= 24*60 {
		minute = 24*60 - 1
	}
	if minute < 0 {
		minute = 0
	}

	h := minute / 60
	m := minute % 60
	return fmt.Sprintf("%02d:%02d", h, m)
}

func snapDown(value, step int) int {
	if step <= 0 {
		return value
	}
	return value - (value % step)
}

func parseInt(value string, fallback int) int {
	var out int
	if _, err := fmt.Sscanf(value, "%d", &out); err != nil {
		return fallback
	}
	return out
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func toggleButtonLabel(base string, expanded bool) string {
	if expanded {
		return "▼ " + base
	}
	return "▶ " + base
}

func testURLWithPlaywright(url string) error {
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("start Playwright: %w", err)
	}
	defer func() {
		_ = pw.Stop()
	}()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	defer func() {
		_ = browser.Close()
	}()

	page, err := browser.NewPage()
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}
	defer func() {
		_ = page.Close()
	}()

	_, err = page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		return fmt.Errorf("open URL: %w", err)
	}

	return nil
}
