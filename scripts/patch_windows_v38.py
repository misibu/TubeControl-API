from pathlib import Path

main_path = Path("windows-v3/MainForm.cs")
api_path = Path("windows-v3/ApiClient.cs")

s = main_path.read_text(encoding="utf-8")
a = api_path.read_text(encoding="utf-8")

# v3.8 — recover gracefully when a previously stored Windows device token is no longer valid.
# The app must not show a dead-end 401 dialog telling the user to check Settings.

# Initial synchronization: immediately offer the native activation dialog again on 401.
old_initial = '''        catch (TubeControlApiException ex)
        {
            _state.Text = ex.IsTemporary
                ? "Сервер временно недоступен — нажмите ↻ для повтора"
                : "Ошибка подключения к серверу";
            _state.ForeColor = ex.IsTemporary ? UiPalette.Yellow : UiPalette.Red;
            MessageBox.Show(this, ex.Message, "TubeControl — подключение к серверу",
                MessageBoxButtons.OK, ex.IsTemporary ? MessageBoxIcon.Warning : MessageBoxIcon.Error);
        }'''
new_initial = '''        catch (TubeControlApiException ex) when (ex.StatusCode == 401)
        {
            await ReauthorizeWindows();
        }
        catch (TubeControlApiException ex)
        {
            _state.Text = ex.IsTemporary
                ? "Сервер временно недоступен — нажмите ↻ для повтора"
                : "Ошибка подключения к серверу";
            _state.ForeColor = ex.IsTemporary ? UiPalette.Yellow : UiPalette.Red;
            MessageBox.Show(this, ex.Message, "TubeControl — подключение к серверу",
                MessageBoxButtons.OK, ex.IsTemporary ? MessageBoxIcon.Warning : MessageBoxIcon.Error);
        }'''
if old_initial not in s:
    raise SystemExit("InitialReload catch block not found")
s = s.replace(old_initial, new_initial, 1)

# Any later operation can also discover that the server revoked/forgot the device.
old_safe = '''        catch (Exception ex) { ShowError(ex); }
        finally { if (disable is not null) disable.Enabled = true; }'''
new_safe = '''        catch (TubeControlApiException ex) when (ex.StatusCode == 401)
        {
            await ReauthorizeWindows();
        }
        catch (Exception ex) { ShowError(ex); }
        finally { if (disable is not null) disable.Enabled = true; }'''
if old_safe not in s:
    raise SystemExit("Safe catch block not found")
s = s.replace(old_safe, new_safe, 1)

# Insert a reusable Windows reactivation flow before InitialReload.
marker = "    private async Task InitialReload()\n"
reauth = r'''    private async Task<bool> ReauthorizeWindows()
    {
        // The stored token was rejected by the server. Keep all user preferences,
        // but remove only the obsolete Windows-device credentials.
        _config.DeviceToken = string.Empty;
        _config.DeviceId = string.Empty;
        _store.Save(_config);

        _state.Text = "Требуется активация Windows";
        _state.ForeColor = UiPalette.Yellow;

        using var activation = new ActivationForm(_store, _config, _api)
        {
            StartPosition = FormStartPosition.CenterParent
        };

        if (activation.ShowDialog(this) != DialogResult.OK)
        {
            _state.Text = "Windows не активирован";
            _state.ForeColor = UiPalette.Yellow;
            return false;
        }

        _state.Text = "Проверяем новое подключение…";
        try
        {
            await Reload();
            return true;
        }
        catch (TubeControlApiException ex) when (ex.StatusCode == 401)
        {
            _state.Text = "Новая активация не принята сервером";
            _state.ForeColor = UiPalette.Red;
            return false;
        }
        catch (Exception ex)
        {
            ShowError(ex);
            return false;
        }
    }
'''
if "private async Task<bool> ReauthorizeWindows()" not in s:
    if marker not in s:
        raise SystemExit("InitialReload marker not found")
    s = s.replace(marker, reauth + "\n" + marker, 1)

# Add an explicit Windows activation section to Settings so recovery is always reachable.
android_label = '''        p.Controls.Add(new Label
        {
            Text = "Android устройства",
            Width = 390,'''
windows_section = r'''        p.Controls.Add(new Label
        {
            Text = "Windows устройство",
            Width = 390,
            Height = 28,
            TextAlign = ContentAlignment.MiddleLeft,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            Margin = new Padding(0, 2, 0, 8)
        });

        var reactivate = CenteredButton("Повторная активация Windows");
        reactivate.Margin = new Padding(35, 0, 35, 18);
        reactivate.Click += async (_, _) =>
        {
            f.Close();
            await ReauthorizeWindows();
        };
        p.Controls.Add(reactivate);

'''
# CenteredButton local function is declared later in v3.6, so we cannot call it before
# its declaration in C#. Insert the section after the local function declaration block instead.
insert_after = '''        GlowButton CenteredButton(string text)
        {
            var b = DialogButton(text);
            b.Width = 320;
            b.Height = 42;
            b.Radius = 20;
            b.BorderWidth = 1;
            b.Margin = new Padding(35, 0, 35, 10);
            return b;
        }

'''
windows_after_helper = r'''        p.Controls.Add(new Label
        {
            Text = "Windows устройство",
            Width = 390,
            Height = 28,
            TextAlign = ContentAlignment.MiddleLeft,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            Margin = new Padding(0, 2, 0, 8)
        });
        var reactivate = CenteredButton("Повторная активация Windows");
        reactivate.Margin = new Padding(35, 0, 35, 18);
        reactivate.Click += async (_, _) =>
        {
            f.Close();
            await ReauthorizeWindows();
        };
        p.Controls.Add(reactivate);

'''
if insert_after not in s:
    raise SystemExit("CenteredButton helper not found")
s = s.replace(insert_after, insert_after + windows_after_helper, 1)

# Make the API fallback text accurate as well; this should rarely surface now.
a = a.replace(
    'throw new TubeControlApiException("Устройство не авторизовано. Откройте «Настройки» и проверьте подключение устройства.", code);',
    'throw new TubeControlApiException("Сохранённая активация Windows больше не действительна. Требуется повторная активация.", code);',
)

main_path.write_text(s, encoding="utf-8", newline="\n")
api_path.write_text(a, encoding="utf-8", newline="\n")
