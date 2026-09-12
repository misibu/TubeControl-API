from pathlib import Path

main_path = Path("windows-v3/MainForm.cs")
api_path = Path("windows-v3/ApiClient.cs")

s = main_path.read_text(encoding="utf-8")
a = api_path.read_text(encoding="utf-8")

# v3.8 — recover gracefully when a previously stored Windows device token is no longer valid.

# v3.4 owns the final InitialReload implementation. On HTTP 401 do not leave the
# user at a dead-end error; immediately open the in-app activation window.
old_initial = '''        catch (TubeControlApiException ex)
        {
            _state.Text = ex.IsTemporary
                ? "Сервер временно недоступен · нажмите ↻"
                : "Ошибка подключения к серверу";
            _state.ForeColor = ex.IsTemporary ? UiPalette.Yellow : UiPalette.Red;
        }'''
new_initial = '''        catch (TubeControlApiException ex) when (ex.StatusCode == 401)
        {
            await ReauthorizeWindows();
        }
        catch (TubeControlApiException ex)
        {
            _state.Text = ex.IsTemporary
                ? "Сервер временно недоступен · нажмите ↻"
                : "Ошибка подключения к серверу";
            _state.ForeColor = ex.IsTemporary ? UiPalette.Yellow : UiPalette.Red;
        }'''
if old_initial not in s:
    raise SystemExit("InitialReload catch block not found")
s = s.replace(old_initial, new_initial, 1)

# Any later command can discover an expired/revoked device too.
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

# Reusable native Windows activation recovery. Theme, overdue settings, server and
# other preferences are preserved; only the invalid device credentials are reset.
marker = "    private async Task InitialReload()\n"
reauth = r'''    private async Task<bool> ReauthorizeWindows()
    {
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

# Add an explicit recovery entry in Settings. A C# local function can be called
# before its declaration, so CenteredButton remains the single source of geometry.
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
        var reactivate = DialogButton("Повторная активация Windows");
        reactivate.Width = 320;
        reactivate.Height = 42;
        reactivate.Radius = 20;
        reactivate.BorderWidth = 1;
        reactivate.Margin = new Padding(35, 0, 35, 18);
        reactivate.Click += async (_, _) =>
        {
            f.Close();
            await ReauthorizeWindows();
        };
        p.Controls.Add(reactivate);

'''
if android_label not in s:
    raise SystemExit("Android settings marker not found")
s = s.replace(android_label, windows_section + android_label, 1)

# Accurate fallback if a 401 is ever surfaced outside the recovery path.
a = a.replace(
    'throw new TubeControlApiException("Устройство не авторизовано. Откройте «Настройки» и проверьте подключение устройства.", code);',
    'throw new TubeControlApiException("Сохранённая активация Windows больше не действительна. Требуется повторная активация.", code);',
)

main_path.write_text(s, encoding="utf-8", newline="\n")
api_path.write_text(a, encoding="utf-8", newline="\n")
