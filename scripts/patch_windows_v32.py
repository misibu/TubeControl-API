from pathlib import Path

p = Path("windows-v3/MainForm.cs")
s = p.read_text(encoding="utf-8")


def replace_between(text: str, start_marker: str, end_marker: str, replacement: str) -> str:
    start = text.index(start_marker)
    end = text.index(end_marker, start)
    return text[:start] + replacement + "\n\n" + text[end:]

# Compile-time type disambiguation without ever round-tripping the C# file through
# Windows PowerShell's legacy ANSI decoding. This is what caused the Russian
# mojibake in previous Windows builds.
s = s.replace(
    "root is Panel or FlowLayoutPanel or TableLayoutPanel",
    "root is System.Windows.Forms.Panel or FlowLayoutPanel or TableLayoutPanel",
)

# Give the brand/sidebar enough room so the TubeControl wordmark is not clipped.
s = s.replace(
    "shell.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute, 205));",
    "shell.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute, 230));",
)
s = s.replace("            Width = 205,\n", "            Width = 230,\n", 1)

build_top = r'''    private Control BuildTopHeader()
    {
        var host = new Panel { Dock = DockStyle.Fill, Padding = new Padding(12, 12, 8, 8) };

        var actions = new FlowLayoutPanel
        {
            Dock = DockStyle.Bottom,
            Height = 49,
            FlowDirection = FlowDirection.LeftToRight,
            WrapContents = false,
            Padding = new Padding(0),
            Margin = Padding.Empty
        };
        actions.Controls.Add(Action("＋  Добавить THU вручную", async () => await AddThu(), 184));
        actions.Controls.Add(Action("↶  Зарегистрировать возврат", async () => await ManualReturn(), 205));
        actions.Controls.Add(Action("⇩  Экспорт", Export, 118));
        actions.Controls.Add(Action("⚙  Настройки", ShowSettings, 126));
        actions.Controls.Add(Action("⟳", Reload, 48));
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
s = replace_between(s, "    private Control BuildTopHeader()\n", "    private Control BuildAccountHeader()\n", build_top)

build_account = r'''    private Control BuildAccountHeader()
    {
        var p = new Panel { Dock = DockStyle.Fill, Padding = new Padding(16, 18, 15, 8) };
        _userName.Text = string.IsNullOrWhiteSpace(_config.DeviceName) ? Environment.MachineName : _config.DeviceName;
        _userName.Location = new Point(18, 18);
        _license.Location = new Point(18, 43);
        p.Controls.Add(_userName);
        p.Controls.Add(_license);
        return p;
    }'''
s = replace_between(s, "    private Control BuildAccountHeader()\n", "    private Control BuildSidebar()\n", build_account)

# Device pairing/management is now consolidated under Settings.
s = s.replace('        menu.Controls.Add(Nav("⌁   Устройства", false, async () => await Devices()));\n', '')

render = r'''    private void Render()
    {
        if (_grid.Columns.Count == 0) return;
        var q = _search.Text.Trim().ToLowerInvariant();
        var sf = FilterCode(_filter.SelectedItem?.ToString() ?? "Все статусы");
        _visible.Clear();
        _visible.AddRange(_all.Where(t =>
            (q.Length == 0 || $"{t.Thu} {t.StoreCode} {t.OrderNumber}".ToLowerInvariant().Contains(q))
            && (sf == "all" || EffectiveStatusCode(t) == sf)));

        _grid.Rows.Clear();
        foreach (var t in _visible)
        {
            var displayStatus = EffectiveStatusCode(t);
            var idx = _grid.Rows.Add(false, t.Thu, t.OrderNumber, t.StoreCode, t.ShippedAt, ExcelService.StatusRu(displayStatus));
            var row = _grid.Rows[idx];
            row.Tag = t;
            row.MinimumHeight = 43;
        }

        _recordCount.Text = $"Показано {_visible.Count} из {_all.Count}";
        ShowSelected();
    }'''
s = replace_between(s, "    private void Render()\n", "    private void GridCellPainting", render)

show_selected = r'''    private void ShowSelected()
    {
        if (_grid.SelectedRows.Count == 0 || _grid.SelectedRows[0].Tag is not TubeItem t)
        {
            foreach (DataGridViewRow row in _grid.Rows)
                if (row.Cells.Count > 0) row.Cells[0].Value = false;
            _detailThu.Text = "THU —";
            _orderValue.Text = "—";
            _storeValue.Text = "—";
            _dateValue.Text = "—";
            _currentStatus.Text = "—";
            _saveStatus.Enabled = false;
            return;
        }

        foreach (DataGridViewRow row in _grid.Rows)
            if (row.Cells.Count > 0) row.Cells[0].Value = row == _grid.SelectedRows[0];

        _saveStatus.Enabled = true;
        _detailThu.Text = t.Thu;
        _orderValue.Text = string.IsNullOrWhiteSpace(t.OrderNumber) ? "—" : t.OrderNumber;
        _storeValue.Text = string.IsNullOrWhiteSpace(t.StoreCode) ? "—" : t.StoreCode;
        _dateValue.Text = string.IsNullOrWhiteSpace(t.ShippedAt) ? "—" : t.ShippedAt;
        var displayStatus = EffectiveStatusCode(t);
        _currentStatus.Text = "●  " + ExcelService.StatusRu(displayStatus);
        _currentStatus.ForeColor = UiDrawing.StatusColor(displayStatus);

        _status.SelectedItem = string.IsNullOrWhiteSpace(t.ManualStatus)
            ? "Автоматически"
            : StatusRuFromCode(t.ManualStatus);
        if (_status.SelectedIndex < 0) _status.SelectedIndex = 0;
        _comment.Text = t.StatusComment ?? "";
    }'''
s = replace_between(s, "    private void ShowSelected()\n", "    private async Task SaveStatus()\n", show_selected)

export_method = r'''    private void Export()
    {
        var rows = _visible
            .Where(t => EffectiveStatusCode(t) != "returned")
            .Select(t => new TubeItem
            {
                Thu = t.Thu,
                ShippedAt = t.ShippedAt,
                StoreCode = t.StoreCode,
                OrderNumber = t.OrderNumber,
                ReturnedAt = t.ReturnedAt,
                ManualStatus = t.ManualStatus,
                Status = EffectiveStatusCode(t),
                StatusComment = t.StatusComment,
                UpdatedAt = t.UpdatedAt
            })
            .ToList();
        using var dlg = new SaveFileDialog
        {
            Filter = "Excel (*.xlsx)|*.xlsx",
            FileName = $"TubeControl_не_возвращены_{DateTime.Now:yyyyMMdd_HHmm}.xlsx"
        };
        if (dlg.ShowDialog(this) != DialogResult.OK) return;
        ExcelService.Export(dlg.FileName, rows);
        MessageBox.Show(this, $"Выгружено: {rows.Count}", "TubeControl");
    }'''
s = replace_between(s, "    private void Export()\n", "    private async Task AddThu()\n", export_method)

settings = r'''    private void ShowSettings()
    {
        using var f = CreateDialog("Настройки", 500, 590);
        var p = DialogStack();
        p.Padding = new Padding(30, 24, 30, 24);

        var title = new Label
        {
            Text = "Настройки TubeControl",
            AutoSize = true,
            Font = new Font("Segoe UI", 15, FontStyle.Bold),
            Margin = new Padding(0, 0, 0, 14)
        };
        p.Controls.Add(title);

        p.Controls.Add(new Label
        {
            Text = "Оформление",
            AutoSize = true,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            Margin = new Padding(0, 5, 0, 7)
        });
        var themes = new FlowLayoutPanel
        {
            Width = 390,
            Height = 48,
            FlowDirection = FlowDirection.LeftToRight,
            WrapContents = false,
            Margin = new Padding(0, 0, 0, 12)
        };
        var dark = DialogButton("Тёмная");
        var light = DialogButton("Светлая");
        dark.Width = 188;
        light.Width = 188;
        dark.Margin = new Padding(0, 0, 8, 0);
        light.Margin = Padding.Empty;
        dark.Click += (_, _) => { SetTheme("dark"); ApplyThemeTo(f); };
        light.Click += (_, _) => { SetTheme("light"); ApplyThemeTo(f); };
        themes.Controls.Add(dark);
        themes.Controls.Add(light);
        p.Controls.Add(themes);

        p.Controls.Add(new Label
        {
            Text = "Просрочено",
            AutoSize = true,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            Margin = new Padding(0, 5, 0, 5)
        });
        p.Controls.Add(new Label
        {
            Text = "Через сколько дней после отправки тубус автоматически отмечается как просроченный:",
            Width = 390,
            Height = 38,
            AutoSize = false,
            Font = new Font("Segoe UI", 8.8f),
            Margin = new Padding(0, 0, 0, 5)
        });
        var overdue = new NumericUpDown
        {
            Width = 150,
            Height = 34,
            Minimum = 1,
            Maximum = 365,
            Value = Math.Clamp(_config.OverdueDays, 1, 365),
            Font = new Font("Segoe UI", 10),
            Margin = new Padding(0, 0, 0, 14)
        };
        p.Controls.Add(overdue);

        p.Controls.Add(new Label
        {
            Text = "Android устройства",
            AutoSize = true,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            Margin = new Padding(0, 5, 0, 7)
        });
        var pair = DialogButton("Подключить Android по QR-коду");
        var devices = DialogButton("Управление подключёнными устройствами");
        pair.Margin = new Padding(0, 0, 0, 7);
        devices.Margin = new Padding(0, 0, 0, 14);
        pair.Click += async (_, _) =>
        {
            f.Close();
            await Safe(PairAndroid);
        };
        devices.Click += async (_, _) =>
        {
            f.Close();
            await Safe(Devices);
        };
        p.Controls.Add(pair);
        p.Controls.Add(devices);

        var save = DialogButton("Сохранить настройки");
        save.FillColor = UiPalette.EmeraldBright;
        save.HoverFillColor = Color.FromArgb(45, 245, 177);
        save.ForeColor = Color.FromArgb(0, 28, 18);
        save.Click += (_, _) =>
        {
            _config.OverdueDays = (int)overdue.Value;
            _store.Save(_config);
            Render();
            f.Close();
        };
        p.Controls.Add(save);

        f.Controls.Add(p);
        ApplyThemeTo(f);
        f.ShowDialog(this);
    }'''
s = replace_between(s, "    private void ShowSettings()\n", "    private string? Prompt", settings)

# Automatic overdue status is a Windows presentation rule. Returned/lost states always
# win; sent/in-transit tubes become overdue once the configured number of full days passes.
effective_status = r'''    private string EffectiveStatusCode(TubeItem t)
    {
        var status = (t.Status ?? string.Empty).Trim().ToLowerInvariant();
        if (status is "returned" or "lost" or "overdue") return status;

        if (DateTime.TryParse(t.ShippedAt, out var shipped))
        {
            var ageDays = (DateTime.Today - shipped.Date).TotalDays;
            if (ageDays >= Math.Clamp(_config.OverdueDays, 1, 365)) return "overdue";
        }
        return string.IsNullOrWhiteSpace(status) ? "transit" : status;
    }
'''
marker = "    private static string FilterCode(string ru) => ru switch\n"
if "private string EffectiveStatusCode(TubeItem t)" not in s:
    s = s.replace(marker, effective_status + "\n" + marker)

p.write_text(s, encoding="utf-8", newline="\n")
