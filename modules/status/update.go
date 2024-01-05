package status

import (
	"bufio"
	"os/exec"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"fyshos.com/fynedesk"
)

const (
	dotSize         = 12
	headerLineCount = 1
)

var updateMeta = fynedesk.ModuleMetadata{
	Name:        "Software Updates",
	NewInstance: newUpdate,
}

type update struct {
	launch  *fyne.Container
	updater string
}

func newUpdate() fynedesk.Module {
	return &update{}
}

// Destroy tidies up resources
func (u *update) Destroy() {
}

func (u *update) setup() error {
	if u.updater == "" {
		apt, err := exec.LookPath("apt")
		if err != nil {
			return err
		}
		u.updater = apt
	}

	go func() {
		time.Sleep(time.Second * 2) //10)
		for {
			u.updateCheck()

			time.Sleep(time.Hour * 12)
		}
	}()
	return nil
}

func (u *update) LaunchSuggestions(input string) []fynedesk.LaunchSuggestion {
	lower := strings.ToLower(input)
	matches := false
	if startsWith(lower, "software") {
		matches = true
	} else if startsWith(lower, "update") {
		matches = true
	}

	if !matches {
		return nil
	}

	return []fynedesk.LaunchSuggestion{&updateItem{u: u}}
}

func (u *update) Shortcuts() map[*fynedesk.Shortcut]func() {
	return nil
}

// StatusAreaWidget builds the widget
func (u *update) StatusAreaWidget() fyne.CanvasObject {
	if err := u.setup(); err != nil {
		fyne.LogError("Unable to start update module", err)
		return nil
	}

	btn := &widget.Button{Icon: theme.ComputerIcon(), Importance: widget.LowImportance, OnTapped: u.launchWindow}
	dot := canvas.NewCircle(theme.WarningColor())
	over := container.NewWithoutLayout(dot)
	dot.Resize(fyne.NewSquareSize(dotSize))
	dot.Move(fyne.NewPos(btn.MinSize().Width-dotSize-2, 2))
	u.launch = container.NewStack(btn, over)
	u.launch.Hide()
	return u.launch
}

// Metadata returns ModuleMetadata
func (u *update) Metadata() fynedesk.ModuleMetadata {
	return updateMeta
}

func (u *update) launchWindow() {
	w := fyne.CurrentApp().NewWindow("Software Update")
	w.SetIcon(theme.ComputerIcon())

	out, err := exec.Command(u.updater, "list", "--upgradable", "-a").Output()
	if err != nil {
		dialog.ShowError(err, w)
		return
	}

	raw := strings.Split(string(out), "\n")
	var tmp []string
	for _, line := range raw[headerLineCount:] {
		if len(line) == 0 {
			continue
		}

		tmp = append(tmp, line)
	}
	lines := binding.BindStringList(&tmp)

	display := widget.NewListWithData(lines,
		func() fyne.CanvasObject {
			l := widget.NewLabel("")
			l.TextStyle.Monospace = true
			return l
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			o.(*widget.Label).Bind(i.(binding.String))
		})

	status := widget.NewLabelWithStyle("Upgrades required:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	cancel := widget.NewButton("Cancel", func() {
		w.Close()
	})
	var upgrade *widget.Button
	upgrade = widget.NewButtonWithIcon("Upgrade", theme.ComputerIcon(), func() {
		go func() {
			cmd := exec.Command("pkexec", "apt-get", "upgrade", "-y")
			upgrade.Disable()
			defer upgrade.Enable()

			outCh, _ := cmd.StdoutPipe()
			errCh, _ := cmd.StderrPipe()
			status.SetText("Upgrading...")
			cmd.Start()

			lines.Set([]string{})
			display.Refresh()
			outLines := bufio.NewScanner(outCh)
			errLines := bufio.NewScanner(errCh)

			go func() {
				for outLines.Scan() {
					line := outLines.Text()
					lines.Append(line)
					display.ScrollToBottom()
				}
			}()
			go func() {
				for errLines.Scan() {
					line := errLines.Text()
					lines.Append(line)
					display.ScrollToBottom()
				}
			}()

			_ = cmd.Wait()

			_ = lines.Append("Done...")
			display.Refresh()
			display.ScrollToBottom()

			cancel.SetText("Done")
			u.updateCheck()
		}()
	})
	upgrade.Importance = widget.HighImportance
	buttons := container.NewHBox(
		layout.NewSpacer(),
		cancel, upgrade,
		layout.NewSpacer())

	w.SetContent(container.NewBorder(status, buttons, nil, nil, display))
	w.Resize(fyne.NewSize(300, 240))
	w.Show()
}

func (u *update) updateCheck() {
	out, err := exec.Command(u.updater, "list", "--upgradable", "-a").Output()
	if err != nil {
		fyne.LogError("Failed to lookup package list", err)
	} else {
		count := len(strings.Split(string(out), "\n")) - 1
		if count > headerLineCount {
			u.launch.Show()
		} else {
			u.launch.Hide()
		}
	}
}

type updateItem struct {
	u *update
}

func (i *updateItem) Icon() fyne.Resource {
	return theme.ComputerIcon()
}

func (i *updateItem) Title() string {
	return "Update Software"
}

func (i *updateItem) Launch() {
	i.u.launchWindow()
}
