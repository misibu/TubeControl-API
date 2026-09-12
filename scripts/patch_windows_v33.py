from pathlib import Path

p = Path("windows-v3/MainForm.cs")
s = p.read_text(encoding="utf-8")


def replace_between(text: str, start_marker: str, end_marker: str, replacement: str) -> str:
    start = text.index(start_marker)
    end = text.index(end_marker, start)
    return text[:start] + replacement + "\n\n" + text[end:]

# Do not let a temporary 502/503/504 from the hosting provider terminate the app
# during the first synchronization.
old_shown = '''        Shown += async (_, _) =>
        {
            _restoreBounds = new Rectangle(80, 60, 1420, 860);
            MaximizeToWorkArea();
            await Reload();
        };'''
new_shown = '''        Shown += async (_, _) =>
        {
            _restoreBounds = new Rectangle(80, 60, 1420, 860);
            MaximizeToWorkArea();
            await InitialReload();
        };'''
s = s.replace(old_shown, new_shown)

brand = r'''    private Control BuildBrandHeader()
    {
        var p = new Panel { Dock = DockStyle.Fill, Padding = new Padding(18, 12, 8, 4) };

        var wordmark = new FlowLayoutPanel
        {
            Location = new Point(18, 13),
            Width = 205,
            Height = 48,
            FlowDirection = FlowDirection.LeftToRight,
            WrapContents = false,
            Margin = Padding.Empty,
            Padding = Padding.Empty
        };
        var tube = new Label
        {
            Text = "Tube",
            AutoSize = true,
            Font = new Font("Segoe UI", 22, FontStyle.Bold),
            Margin = Padding.Empty,
            Padding = Padding.Empty
        };
        var control = new Label
        {
            Text = "Control",
            AutoSize = true,
            Font = new Font("Segoe UI", 22, FontStyle.Bold),
            ForeColor = UiPalette.EmeraldBright,
            Margin = Padding.Empty,
            Padding = Padding.Empty
        };
        wordmark.Controls.Add(tube);
        wordmark.Controls.Add(control);

        var tag = new Label
        {
            Text = "TRACK  ·  RETURN  ·  UNDER CONTROL",
            AutoSize = true,
            Font = new Font("Segoe UI", 6.2f, FontStyle.Bold),
            ForeColor = Color.FromArgb(135, 161, 151),
            Location = new Point(20, 64)
        };
        p.Controls.Add(wordmark);
        p.Controls.Add(tag);
        return p;
    }'''
s = replace_between(s, "    private Control BuildBrandHeader()\n", "    private Control BuildTopHeader()\n", brand)

top = r'''    private Control BuildTopHeader()
    {
        var host = new Panel { Dock = DockStyle.Fill, Padding = new Padding(12, 12, 8, 8) };

        var actions = new FlowLayoutPanel
        {
            Dock = DockStyle.Bottom,
            Height = 49,
            FlowDirection = FlowDirection.LeftToRight,
            WrapContents = false,
            AutoScroll = false,
            Padding = Padding.Empty,
            Margin = Padding.Empty
        };
        actions.Controls.Add(Action("＋  Добавить THU вручную", async () => await AddThu(), 218));
        actions.Controls.Add(Action("↶  Зарегистрировать возврат", async () => await ManualReturn(), 246));
        actions.Controls.Add(Action("↓  Экспорт", Export, 132));
        actions.Controls.Add(Action("⚙  Настройки", ShowSettings, 150));
        actions.Controls.Add(Action("↻", Reload, 52));
        host.Controls.Add(actions);

        var statusLine = new FlowLayoutPanel
        {
            Dock = DockStyle.Top,
            Height = 38,
            FlowDirection = FlowDirection.RightToLeft,
            WrapContents = false,
            Padding = Padding.Empty
        };
        _state.Margin = new Padding(6, 9, 6, 0);
        statusLine.Controls.Add(_state);
        host.Controls.Add(statusLine);
        return host;
    }'''
s = replace_between(s, "    private Control BuildTopHeader()\n", "    private Control BuildAccountHeader()\n", top)

action = r'''    private GlowButton Action(string text, Func<Task> fn, int width)
    {
        var font = new Font("Segoe UI", 9);
        var measured = TextRenderer.MeasureText(text, font, new Size(int.MaxValue, 42), TextFormatFlags.NoPadding);
        var b = new GlowButton
        {
            Text = text,
            Width = Math.Max(width, measured.Width + 38),
            Height = 42,
            Font = font,
            Margin = new Padding(0, 0, 8, 0),
            Radius = 10
        };
        b.Click += async (_, _) => await Safe(fn, b);
        return b;
    }'''
s = replace_between(s, "    private GlowButton Action(string text, Func<Task> fn, int width)\n", "    private GlowButton Action(string text, Action fn, int width)", action)

# Make temporary server outages visible but non-fatal. The app stays open and the
# existing refresh button can retry the request.
initial_reload = r'''    private async Task InitialReload()
    {
        try
        {
            await Reload();
        }
        catch (TubeControlApiException ex)
        {
            _state.Text = ex.IsTemporary
                ? "Сервер временно недоступен — нажмите ↻ для повтора"
                : "Ошибка подключения к серверу";
            _state.ForeColor = ex.IsTemporary ? UiPalette.Yellow : UiPalette.Red;
            MessageBox.Show(this, ex.Message, "TubeControl — подключение к серверу",
                MessageBoxButtons.OK, ex.IsTemporary ? MessageBoxIcon.Warning : MessageBoxIcon.Error);
        }
        catch (Exception ex)
        {
            _state.Text = "Ошибка подключения к серверу";
            _state.ForeColor = UiPalette.Red;
            ShowError(ex);
        }
    }
'''
marker = "    private async Task Reload()\n"
if "private async Task InitialReload()" not in s:
    s = s.replace(marker, initial_reload + "\n" + marker)

# For later operations, keep the friendly API error text and do not show raw HTML.
show_error_old = '''    private void ShowError(Exception ex) =>
        MessageBox.Show(this, ex.Message, "TubeControl", MessageBoxButtons.OK, MessageBoxIcon.Error);'''
show_error_new = '''    private void ShowError(Exception ex)
    {
        var icon = ex is TubeControlApiException api && api.IsTemporary
            ? MessageBoxIcon.Warning
            : MessageBoxIcon.Error;
        if (ex is TubeControlApiException apiEx && apiEx.IsTemporary)
        {
            _state.Text = "Сервер временно недоступен — нажмите ↻ для повтора";
            _state.ForeColor = UiPalette.Yellow;
        }
        MessageBox.Show(this, ex.Message, "TubeControl", MessageBoxButtons.OK, icon);
    }'''
s = s.replace(show_error_old, show_error_new)

# Restore the muted status color after a successful refresh.
reload_old = '''        Render();
        _state.Text = $"Записей: {_all.Count}";'''
reload_new = '''        Render();
        _state.Text = $"Записей: {_all.Count}";
        _state.ForeColor = Muted;'''
s = s.replace(reload_old, reload_new)

p.write_text(s, encoding="utf-8", newline="\n")
