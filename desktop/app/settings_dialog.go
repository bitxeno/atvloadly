package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bitxeno/atvloadly/internal/i18n"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type settingsDialog struct {
	window     *mygo.Window
	configPath string
	port       string
	allowLAN   bool
	errorText  string
	saved      bool
	input      ui.Handle
}

var activeSettingsDialog *settingsDialog

// openSettingsDialog opens the native settings page for the embedded service.
func openSettingsDialog(configPath string) {
	if activeSettingsDialog != nil && activeSettingsDialog.window != nil && !activeSettingsDialog.window.IsDestroyed() {
		activeSettingsDialog.window.Show()
		if !activeSettingsDialog.saved {
			activeSettingsDialog.input.Focus()
		}
		return
	}

	settings, err := loadDesktopSettings(configPath)
	if err != nil {
		showSettingsError(err)
		return
	}

	d := &settingsDialog{
		configPath: configPath,
		port:       strconv.Itoa(settings.Port),
		allowLAN:   settings.allowLAN(),
	}
	d.window = mygo.NewWindow(mygo.WindowOptions{
		Title:             i18n.Localize("tray.settings_title"),
		Width:             440,
		Height:            300,
		UseContentSize:    true,
		DisableResize:     true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		Content:           ui.View(d.view),
	})
	d.window.OnClosed(func() {
		if activeSettingsDialog == d {
			activeSettingsDialog = nil
		}
	})
	activeSettingsDialog = d
	d.input.Focus()
}

func closeSettingsDialog() {
	if activeSettingsDialog != nil && activeSettingsDialog.window != nil && !activeSettingsDialog.window.IsDestroyed() {
		activeSettingsDialog.window.Destroy()
	}
	activeSettingsDialog = nil
}

func (d *settingsDialog) view(c *ui.Context) {
	if d.saved {
		d.savedView(c)
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		d.window.Close()
	}

	t := c.Theme()
	ui.Column(c).Fill().Padding(20).Gap(14).Children(func() {
		ui.Text(c, i18n.Localize("tray.settings_description")).
			FontSize(12).TextColor(t.TextMuted)

		var portInput ui.Element
		ui.Field(c, i18n.Localize("tray.settings_port_label"), func() {
			portInput = ui.TextInput(c, &d.port).Bind(&d.input).Placeholder("5533").Width(360).
				Label(i18n.Localize("tray.settings_port_label"))
		}).Error(d.errorText)
		if portInput.Submitted() {
			d.save()
		}

		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Switch(c, &d.allowLAN).Label(i18n.Localize("tray.settings_lan_label"))
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				ui.Text(c, i18n.Localize("tray.settings_lan_label")).FontSize(13)
				ui.Text(c, i18n.Localize("tray.settings_lan_help")).FontSize(11).TextColor(t.TextMuted)
			})
		})

		ui.Spacer(c)
		ui.Row(c).Gap(8).Children(func() {
			ui.Spacer(c)
			if ui.Button(c, i18n.Localize("tray.settings_cancel")).Clicked() {
				d.window.Close()
			}
			if ui.PrimaryButton(c, i18n.Localize("tray.settings_save")).Clicked() {
				d.save()
			}
		})
	})
}

func (d *settingsDialog) save() {
	port, err := strconv.Atoi(strings.TrimSpace(d.port))
	if err != nil {
		d.fail(i18n.Localize("tray.settings_invalid_port"))
		return
	}
	if err := validateServerPort(port); err != nil {
		d.fail(i18n.Localize("tray.settings_invalid_port"))
		return
	}

	listenAddr := loopbackListenAddr
	if d.allowLAN {
		listenAddr = lanListenAddr
	}
	settings := desktopSettings{Port: port, ListenAddr: listenAddr}
	if err := saveDesktopSettings(d.configPath, settings); err != nil {
		d.fail(fmt.Sprintf("%s: %v", i18n.Localize("tray.settings_save_error"), err))
		return
	}

	d.saved = true
	d.errorText = ""
	d.window.Invalidate()
}

func (d *settingsDialog) fail(message string) {
	d.errorText = message
	d.window.Invalidate()
	d.input.Focus()
}

func (d *settingsDialog) savedView(c *ui.Context) {
	ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
		ui.Text(c, i18n.Localize("tray.settings_saved")).FontSize(13)
		ui.Row(c).Gap(8).Children(func() {
			ui.Spacer(c)
			if ui.PrimaryButton(c, i18n.Localize("common.button.close")).Clicked() {
				d.window.Close()
			}
		})
	})
}

// showSettingsError reports a settings file problem in a native window.
func showSettingsError(err error) {
	d := &settingsDialog{errorText: err.Error()}
	d.window = mygo.NewWindow(mygo.WindowOptions{
		Title:             i18n.Localize("tray.settings_title"),
		Width:             440,
		Height:            180,
		UseContentSize:    true,
		DisableResize:     true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		Content: ui.View(func(c *ui.Context) {
			ui.Column(c).Fill().Padding(20).Gap(16).Children(func() {
				ui.Text(c, fmt.Sprintf("%s: %v", i18n.Localize("tray.settings_save_error"), err)).FontSize(13)
				ui.Row(c).Gap(8).Children(func() {
					ui.Spacer(c)
					if ui.PrimaryButton(c, i18n.Localize("common.button.close")).Clicked() {
						d.window.Close()
					}
				})
			})
		}),
	})
	d.window.OnClosed(func() {
		if activeSettingsDialog == d {
			activeSettingsDialog = nil
		}
	})
	activeSettingsDialog = d
}
