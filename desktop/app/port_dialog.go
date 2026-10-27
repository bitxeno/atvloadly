package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bitxeno/atvloadly/internal/app"
	"github.com/bitxeno/atvloadly/internal/i18n"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type portDialog struct {
	window      *mygo.Window
	configPath  string
	port        string
	currentPort int
	errorText   string
	saved       bool
	input       ui.Handle
}

var activePortDialog *portDialog

// openPortDialog opens a small native MyGo window for editing server.port.
func openPortDialog(configPath string) {
	if activePortDialog != nil && activePortDialog.window != nil && !activePortDialog.window.IsDestroyed() {
		activePortDialog.window.Show()
		if !activePortDialog.saved {
			activePortDialog.input.Focus()
		}
		return
	}

	port := defaultServerPort
	if app.Config != nil && app.Config.Server.Port > 0 {
		port = app.Config.Server.Port
	}

	d := &portDialog{
		configPath:  configPath,
		port:        strconv.Itoa(port),
		currentPort: port,
	}
	d.window = mygo.NewWindow(mygo.WindowOptions{
		Title:             i18n.Localize("tray.server_port_dialog_title"),
		Width:             400,
		Height:            230,
		UseContentSize:    true,
		Panel:             true,
		DisableResize:     true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		Content:           ui.View(d.view),
	})
	d.window.OnClosed(func() {
		if activePortDialog == d {
			activePortDialog = nil
		}
	})
	activePortDialog = d
	d.input.Focus()
}

func closePortDialog() {
	if activePortDialog != nil && activePortDialog.window != nil && !activePortDialog.window.IsDestroyed() {
		activePortDialog.window.Destroy()
	}
	activePortDialog = nil
}

func (d *portDialog) view(c *ui.Context) {
	if d.saved {
		d.savedView(c)
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		d.window.Close()
	}

	t := c.Theme()
	ui.Column(c).Fill().Padding(20).Gap(14).Children(func() {
		ui.Text(c, i18n.LocalizeF("tray.server_port_dialog_message", map[string]any{"port": d.currentPort})).
			FontSize(13).TextColor(t.TextMuted)

		var portInput ui.Element
		ui.Field(c, i18n.Localize("tray.server_port_dialog_field"), func() {
			portInput = ui.TextInput(c, &d.port).Bind(&d.input).Placeholder("5533").Width(340).
				Label(i18n.Localize("tray.server_port_dialog_field"))
		}).Error(d.errorText)
		if portInput.Submitted() {
			d.save()
		}

		ui.Row(c).Gap(8).Children(func() {
			ui.Spacer(c)
			if ui.Button(c, i18n.Localize("tray.server_port_dialog_cancel")).Clicked() {
				d.window.Close()
			}
			if ui.PrimaryButton(c, i18n.Localize("tray.server_port_dialog_save")).Clicked() {
				d.save()
			}
		})
	})
}

func (d *portDialog) save() {
	port, err := strconv.Atoi(strings.TrimSpace(d.port))
	if err != nil {
		d.errorText = i18n.Localize("tray.server_port_dialog_invalid")
		d.window.Invalidate()
		d.input.Focus()
		return
	}
	if err := validateServerPort(port); err != nil {
		d.errorText = i18n.Localize("tray.server_port_dialog_invalid")
		d.window.Invalidate()
		d.input.Focus()
		return
	}
	if err := setServerPort(d.configPath, port); err != nil {
		d.errorText = fmt.Sprintf("%s: %v", i18n.Localize("tray.server_port_dialog_save_error"), err)
		d.window.Invalidate()
		d.input.Focus()
		return
	}

	d.saved = true
	d.errorText = ""
	d.window.SetTitle(i18n.Localize("tray.server_port_dialog_title"))
	d.window.Invalidate()
}

func (d *portDialog) savedView(c *ui.Context) {
	ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
		ui.Text(c, i18n.Localize("tray.server_port_dialog_saved")).FontSize(13)
		ui.Row(c).Gap(8).Children(func() {
			ui.Spacer(c)
			if ui.PrimaryButton(c, i18n.Localize("common.button.close")).Clicked() {
				d.window.Close()
			}
		})
	})
}
