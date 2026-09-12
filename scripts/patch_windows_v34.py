from pathlib import Path

p = Path("windows-v3/MainForm.cs")
s = p.read_text(encoding="utf-8")


def replace_between(text: str, start_marker: str, end_marker: str, replacement: str) -> str:
    start = text.index(start_marker)
    end = text.index(end_marker, start)
    return text[:start] + replacement + "\n\n" + text[end:]

# Minimal chrome: only window controls, no duplicate mini-brand in the title area.
chrome = r'''    private Control BuildChrome()
    {
        _chrome.BackColor = Color.FromArgb(2, 11, 8);
        _chrome.MouseDown += (_, e) =>
        {
            if (e.Button == MouseButtons.Left)
            {
                ReleaseCapture();
                SendMessage(Handle, 0xA1, 0x2, 0);
            }
        };
        _chrome.DoubleClick += (_, _) => ToggleMaximize();

        var right = new FlowLayoutPanel
        {
            Dock = DockStyle.Right,
            AutoSize = true,
            WrapContents = false,
            FlowDirection = FlowDirection.LeftToRight,
            Margin = Padding.Empty,
            Padding = Padding.Empty
        };
        right.Controls.Add(WindowButton("—", (_, _) => WindowState = FormWindowState.Minimized));
        _maxButton = WindowButton("□", (_, _) => ToggleMaximize());
        right.Controls.Add(_maxButton);
        right.Controls.Add(WindowButton("×", (_, _) => Close(), true));
        _chrome.Controls.Add(right);
        return _chrome;
    }'''
s = replace_between(s, "    private Control BuildChrome()\n", "    private Button WindowButton", chrome)

# Entire body is rebuilt: no left menu, no ad/banner, no right details/account block.
body = r'''    private Control BuildBody()
    {
        var shell = new TableLayoutPanel
        {
            Dock = DockStyle.Fill,
            ColumnCount = 1,
            RowCount = 2,
            Padding = new Padding(28, 18, 28, 22),
            Margin = Padding.Empty
        };
        shell.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 100));
        shell.RowStyles.Add(new RowStyle(SizeType.Absolute, 112));
        shell.RowStyles.Add(new RowStyle(SizeType.Percent, 100));
        shell.Controls.Add(BuildMinimalHeader(), 0, 0);
        shell.Controls.Add(BuildMinimalCenter(), 0, 1);
        return shell;
    }

    private Control BuildMinimalHeader()
    {
        var host = new Panel { Dock = DockStyle.Fill, Margin = Padding.Empty };

        var wordmark = new FlowLayoutPanel
        {
            Location = new Point(0, 6),
            Width = 430,
            Height = 58,
            FlowDirection = FlowDirection.LeftToRight,
            WrapContents = false,
            Margin = Padding.Empty,
            Padding = Padding.Empty
        };
        wordmark.Controls.Add(new Label
        {
            Text = "Tube",
            AutoSize = true,
            Font = new Font("Segoe UI", 31, FontStyle.Bold),
            Margin = Padding.Empty,
            Padding = Padding.Empty
        });
        wordmark.Controls.Add(new Label
        {
            Text = "Control",
            AutoSize = true,
            Font = new Font("Segoe UI", 31, FontStyle.Bold),
            ForeColor = UiPalette.EmeraldBright,
            Margin = Padding.Empty,
            Padding = Padding.Empty
        });
        host.Controls.Add(wordmark);

        var subtitle = new Label
        {
            Text = "УЧЁТ  ·  ВОЗВРАТ  ·  ПОД КОНТРОЛЕМ",
            AutoSize = true,
            Font = new Font("Segoe UI", 8.2f, FontStyle.Bold),
            ForeColor = Color.FromArgb(138, 174, 160),
            Location = new Point(3, 68)
        };
        host.Controls.Add(subtitle);

        var settings = Action("⚙  Настройки", ShowSettings, 150);
        settings.Radius = 21;
        settings.Anchor = AnchorStyles.Top | AnchorStyles.Right;
        settings.Location = new Point(Math.Max(0, host.Width - settings.Width), 14);
        host.Controls.Add(settings);

        _state.AutoSize = true;
        _state.TextAlign = ContentAlignment.MiddleRight;
        _state.Anchor = AnchorStyles.Top | AnchorStyles.Right;
        _state.Location = new Point(Math.Max(0, host.Width - 360), 68);
        host.Controls.Add(_state);

        host.Resize += (_, _) =>
        {
            settings.Left = host.Width - settings.Width;
            _state.Left = Math.Max(450, host.Width - _state.Width);
        };
        return host;
    }

    private Control BuildMinimalCenter()
    {
        var center = new TableLayoutPanel
        {
            Dock = DockStyle.Fill,
            RowCount = 4,
            ColumnCount = 1,
            Padding = Padding.Empty,
            Margin = Padding.Empty
        };
        center.RowStyles.Add(new RowStyle(SizeType.Absolute, 58));
        center.RowStyles.Add(new RowStyle(SizeType.Absolute, 58));
        center.RowStyles.Add(new RowStyle(SizeType.Percent, 100));
        center.RowStyles.Add(new RowStyle(SizeType.Absolute, 54));
        center.Controls.Add(BuildMinimalToolbar(), 0, 0);
        center.Controls.Add(BuildMinimalSearchRow(), 0, 1);
        center.Controls.Add(BuildGridCard(), 0, 2);
        center.Controls.Add(BuildPager(), 0, 3);

        _grid.CellDoubleClick += async (_, e) =>
        {
            if (e.RowIndex >= 0) await Safe(EditSelectedStatus);
        };
        return center;
    }

    private Control BuildMinimalToolbar()
    {
        var host = new FlowLayoutPanel
        {
            Dock = DockStyle.Fill,
            FlowDirection = FlowDirection.LeftToRight,
            WrapContents = false,
            Padding = new Padding(0, 6, 0, 4),
            Margin = Padding.Empty
        };

        var import = Action("Импорт Excel", async () => await ImportExcel(), 138);
        var add = Action("Добавить THU", async () => await AddThu(), 142);
        var ret = Action("Зарегистрировать возврат", async () => await ManualReturn(), 196);
        var status = Action("Изменить статус", EditSelectedStatus, 158);
        var export = Action("Выгрузить", Export, 118);
        foreach (var b in new[] { import, add, ret, status, export })
        {
            b.Radius = 21;
            b.Height = 42;
            b.Margin = new Padding(0, 0, 10, 0);
            host.Controls.Add(b);
        }
        return host;
    }

    private Control BuildMinimalSearchRow()
    {
        var host = new Panel { Dock = DockStyle.Fill, Margin = Padding.Empty };
        var searchCard = new RoundedPanel
        {
            Dock = DockStyle.Left,
            Width = 560,
            Height = 44,
            Padding = new Padding(16, 8, 12, 5),
            Radius = 22,
            Margin = Padding.Empty
        };
        _search.BorderStyle = BorderStyle.None;
        _search.Font = new Font("Segoe UI", 10.5f);
        _search.Dock = DockStyle.Fill;
        _search.TextChanged += (_, _) => Render();
        searchCard.Controls.Add(_search);
        host.Controls.Add(searchCard);

        _filter.Width = 180;
        _filter.Height = 40;
        if (_filter.Items.Count == 0)
        {
            _filter.Items.AddRange(new object[] { "Все статусы", "Отправлен", "В пути", "Возвращён", "Потерян", "Просрочен" });
            _filter.SelectedIndex = 0;
            _filter.SelectedIndexChanged += (_, _) => Render();
        }
        _filter.Location = new Point(574, 2);
        host.Controls.Add(_filter);

        var refresh = Action("↻", Reload, 48);
        refresh.Radius = 21;
        refresh.Anchor = AnchorStyles.Top | AnchorStyles.Right;
        refresh.Location = new Point(Math.Max(0, host.Width - refresh.Width), 1);
        host.Controls.Add(refresh);
        host.Resize += (_, _) => refresh.Left = host.Width - refresh.Width;
        return host;
    }'''
s = replace_between(s, "    private Control BuildBody()\n", "    private Control BuildBrandHeader()\n", body)

# Make the status card editor available as a small modal instead of the removed right panel.
edit_status = r'''    private async Task EditSelectedStatus()
    {
        if (_grid.SelectedRows.Count == 0 || _grid.SelectedRows[0].Tag is not TubeItem t)
        {
            MessageBox.Show(this, "Сначала выберите тубус в таблице.", "TubeControl", MessageBoxButtons.OK, MessageBoxIcon.Information);
            return;
        }

        using var f = CreateDialog("Изменить статус", 470, 420);
        var stack = DialogStack();
        stack.Padding = new Padding(30, 24, 30, 24);
        stack.Controls.Add(new Label
        {
            Text = t.Thu,
            AutoSize = true,
            Font = new Font("Segoe UI", 17, FontStyle.Bold),
            Margin = new Padding(0, 0, 0, 14)
        });
        stack.Controls.Add(LabelSmall("Статус"));
        var status = new ComboBox
        {
            Width = 350,
            Height = 36,
            DropDownStyle = ComboBoxStyle.DropDownList,
            Font = new Font("Segoe UI", 10),
            Margin = new Padding(0, 4, 0, 14)
        };
        status.Items.AddRange(new object[] { "Автоматически", "Отправлен", "В пути", "Возвращён", "Потерян" });
        status.SelectedItem = string.IsNullOrWhiteSpace(t.ManualStatus) ? "Автоматически" : StatusRuFromCode(t.ManualStatus);
        if (status.SelectedIndex < 0) status.SelectedIndex = 0;
        stack.Controls.Add(status);

        stack.Controls.Add(LabelSmall("Комментарий"));
        var comment = new TextBox
        {
            Width = 350,
            Height = 86,
            Multiline = true,
            MaxLength = 500,
            Text = t.StatusComment ?? string.Empty,
            BorderStyle = BorderStyle.FixedSingle,
            Font = new Font("Segoe UI", 10),
            Margin = new Padding(0, 4, 0, 16)
        };
        stack.Controls.Add(comment);
        var save = DialogButton("Сохранить");
        save.Width = 350;
        save.Radius = 20;
        save.Click += (_, _) => f.DialogResult = DialogResult.OK;
        stack.Controls.Add(save);
        f.Controls.Add(stack);
        ApplyThemeTo(f);

        if (f.ShowDialog(this) != DialogResult.OK) return;
        var code = StatusCode(status.SelectedItem?.ToString() ?? "Автоматически");
        await _api.UpdateStatusAsync(_config.DeviceToken, t.Thu, code, comment.Text.Trim());
        await Reload();
    }
'''
marker = "    private async Task SaveStatus()\n"
if "private async Task EditSelectedStatus()" not in s:
    s = s.replace(marker, edit_status + "\n" + marker)

# On a temporary hosting outage just leave the main UI open and show a concise status.
initial = r'''    private async Task InitialReload()
    {
        try
        {
            await Reload();
        }
        catch (TubeControlApiException ex)
        {
            _state.Text = ex.IsTemporary
                ? "Сервер временно недоступен · нажмите ↻"
                : "Ошибка подключения к серверу";
            _state.ForeColor = ex.IsTemporary ? UiPalette.Yellow : UiPalette.Red;
        }
        catch
        {
            _state.Text = "Не удалось подключиться к серверу";
            _state.ForeColor = UiPalette.Red;
        }
    }'''
s = replace_between(s, "    private async Task InitialReload()\n", "    private async Task Reload()\n", initial)

# v3.4 visual tweaks for the table card.
s = s.replace("_grid.ColumnHeadersHeight = 46;", "_grid.ColumnHeadersHeight = 44;")
s = s.replace("_grid.RowTemplate.Height = 43;", "_grid.RowTemplate.Height = 42;")
s = s.replace("row.MinimumHeight = 43;", "row.MinimumHeight = 42;")

p.write_text(s, encoding="utf-8", newline="\n")
