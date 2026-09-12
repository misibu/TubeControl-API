from pathlib import Path

p = Path("windows-v3/MainForm.cs")
s = p.read_text(encoding="utf-8")


def replace_between(text: str, start_marker: str, end_marker: str, replacement: str) -> str:
    start = text.index(start_marker)
    end = text.index(end_marker, start)
    return text[:start] + replacement + "\n\n" + text[end:]

# v3.8.1 — compact settings dialog that fits on typical 768p desktops,
# with all controls centered, and a wider Edit button.

# Widen the toolbar Edit button so the full Russian label is always visible.
s = s.replace(
    'var status = Action("Редактировать", EditSelectedStatus, 132);',
    'var status = Action("Редактировать", EditSelectedStatus, 178);',
)

settings = r'''    private void ShowSettings()
    {
        // Keep the dialog comfortably inside a common 1366x768 working area.
        using var f = CreateDialog("Настройки", 620, 700);
        f.MinimumSize = new Size(620, 700);

        var outer = new TableLayoutPanel
        {
            Dock = DockStyle.Fill,
            ColumnCount = 3,
            RowCount = 2,
            Padding = new Padding(0, 14, 0, 14),
            Margin = Padding.Empty
        };
        outer.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 50));
        outer.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute, 420));
        outer.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 50));
        outer.RowStyles.Add(new RowStyle(SizeType.Percent, 100));
        outer.RowStyles.Add(new RowStyle(SizeType.Absolute, 58));

        var p = new TableLayoutPanel
        {
            Dock = DockStyle.Fill,
            ColumnCount = 1,
            RowCount = 15,
            Padding = Padding.Empty,
            Margin = Padding.Empty
        };
        p.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 100));
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 48));  // title
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 30));  // appearance label
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 48));  // themes
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 12));  // gap
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 30));  // overdue label
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 48));  // overdue description
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 42));  // overdue number
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 12));  // gap
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 30));  // windows label
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 48));  // windows action
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 12));  // gap
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 30));  // android label
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 48));  // pair
        p.RowStyles.Add(new RowStyle(SizeType.Absolute, 48));  // devices
        p.RowStyles.Add(new RowStyle(SizeType.Percent, 100));

        Label Section(string text) => new()
        {
            Text = text,
            Dock = DockStyle.Fill,
            TextAlign = ContentAlignment.MiddleLeft,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            Margin = new Padding(0)
        };

        GlowButton CenteredButton(string text, int width = 340)
        {
            var b = DialogButton(text);
            b.Width = width;
            b.Height = 42;
            b.Radius = 20;
            b.BorderWidth = 1;
            b.Anchor = AnchorStyles.None;
            b.Margin = Padding.Empty;
            return b;
        }

        var title = new Label
        {
            Text = "Настройки TubeControl",
            Dock = DockStyle.Fill,
            TextAlign = ContentAlignment.MiddleCenter,
            Font = new Font("Segoe UI", 16, FontStyle.Bold),
            Margin = Padding.Empty
        };
        p.Controls.Add(title, 0, 0);

        p.Controls.Add(Section("Оформление"), 0, 1);
        var themes = new TableLayoutPanel
        {
            Dock = DockStyle.Fill,
            ColumnCount = 2,
            RowCount = 1,
            Padding = Padding.Empty,
            Margin = Padding.Empty
        };
        themes.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 50));
        themes.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 50));
        var dark = CenteredButton("Тёмная", 190);
        var light = CenteredButton("Светлая", 190);
        dark.Click += (_, _) => { SetTheme("dark"); ApplyThemeTo(f); };
        light.Click += (_, _) => { SetTheme("light"); ApplyThemeTo(f); };
        themes.Controls.Add(dark, 0, 0);
        themes.Controls.Add(light, 1, 0);
        p.Controls.Add(themes, 0, 2);

        p.Controls.Add(Section("Просрочено"), 0, 4);
        p.Controls.Add(new Label
        {
            Text = "Через сколько дней после отправки тубус автоматически отмечается как просроченный:",
            Dock = DockStyle.Fill,
            TextAlign = ContentAlignment.MiddleLeft,
            Font = new Font("Segoe UI", 9f),
            Margin = Padding.Empty
        }, 0, 5);

        var overdueHost = new Panel { Dock = DockStyle.Fill, Margin = Padding.Empty };
        var overdue = new NumericUpDown
        {
            Width = 150,
            Height = 34,
            Minimum = 1,
            Maximum = 365,
            Value = Math.Clamp(_config.OverdueDays, 1, 365),
            Font = new Font("Segoe UI", 10),
            Anchor = AnchorStyles.None
        };
        overdueHost.Controls.Add(overdue);
        overdueHost.Resize += (_, _) =>
        {
            overdue.Left = (overdueHost.ClientSize.Width - overdue.Width) / 2;
            overdue.Top = (overdueHost.ClientSize.Height - overdue.Height) / 2;
        };
        p.Controls.Add(overdueHost, 0, 6);

        p.Controls.Add(Section("Windows устройство"), 0, 8);
        var reactivate = CenteredButton("Повторная активация Windows");
        reactivate.Click += async (_, _) =>
        {
            f.Close();
            await ReauthorizeWindows();
        };
        p.Controls.Add(reactivate, 0, 9);

        p.Controls.Add(Section("Android устройства"), 0, 11);
        var pair = CenteredButton("Подключить Android по QR-коду");
        var devices = CenteredButton("Управление подключёнными устройствами");
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
        p.Controls.Add(pair, 0, 12);
        p.Controls.Add(devices, 0, 13);

        outer.Controls.Add(p, 1, 0);

        // Save is outside the content stack, so it is always visible and never scrolls away.
        var saveHost = new Panel { Dock = DockStyle.Fill, Margin = Padding.Empty };
        var save = CenteredButton("Сохранить настройки", 340);
        save.Tag = "primary";
        save.FillColor = UiPalette.EmeraldBright;
        save.HoverFillColor = Color.FromArgb(45, 245, 177);
        save.PressedFillColor = UiPalette.Emerald;
        save.BorderColor = UiPalette.Mint;
        save.ForeColor = Color.FromArgb(0, 28, 18);
        save.Click += (_, _) =>
        {
            _config.OverdueDays = (int)overdue.Value;
            _store.Save(_config);
            Render();
            f.Close();
        };
        saveHost.Controls.Add(save);
        saveHost.Resize += (_, _) =>
        {
            save.Left = (saveHost.ClientSize.Width - save.Width) / 2;
            save.Top = (saveHost.ClientSize.Height - save.Height) / 2;
        };
        outer.Controls.Add(saveHost, 1, 1);

        f.Controls.Add(outer);
        ApplyThemeTo(f);
        f.ShowDialog(this);
    }'''

s = replace_between(s, "    private void ShowSettings()\n", "    private string? Prompt", settings)

p.write_text(s, encoding="utf-8", newline="\n")
