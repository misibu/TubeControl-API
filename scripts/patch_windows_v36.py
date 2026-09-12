from pathlib import Path

main_path = Path("windows-v3/MainForm.cs")
ui_path = Path("windows-v3/UiKit.cs")

s = main_path.read_text(encoding="utf-8")
u = ui_path.read_text(encoding="utf-8")


def replace_between(text: str, start_marker: str, end_marker: str, replacement: str) -> str:
    start = text.index(start_marker)
    end = text.index(end_marker, start)
    return text[:start] + replacement + "\n\n" + text[end:]

# ---------------------------------------------------------------------------
# v3.6 UI cleanup
# ---------------------------------------------------------------------------
# Final brand subtitle requested by the user.
s = s.replace(
    'Text = "УЧЁТ  ·  ВОЗВРАТ  ·  ПОД КОНТРОЛЕМ",',
    'Text = "Учет и Контроль",',
)

# Exact, stable button geometry. Do not auto-grow buttons based on measured text;
# it made neighboring controls look uneven at different Windows DPI settings.
action = r'''    private GlowButton Action(string text, Func<Task> fn, int width)
    {
        var b = new GlowButton
        {
            Text = text,
            Width = width,
            Height = 42,
            Font = new Font("Segoe UI", 10f, FontStyle.Regular),
            Margin = new Padding(0, 0, 10, 0),
            Radius = 20,
            BorderWidth = 1,
            BorderColor = UiPalette.Emerald
        };
        b.Click += async (_, _) => await Safe(fn, b);
        return b;
    }'''
s = replace_between(s, "    private GlowButton Action(string text, Func<Task> fn, int width)\n", "    private GlowButton Action(string text, Action fn, int width)", action)

# Main toolbar: fixed widths and a common 42px height/radius.
toolbar = r'''    private Control BuildMinimalToolbar()
    {
        var host = new FlowLayoutPanel
        {
            Dock = DockStyle.Fill,
            FlowDirection = FlowDirection.LeftToRight,
            WrapContents = false,
            Padding = new Padding(0, 7, 0, 5),
            Margin = Padding.Empty
        };

        var import = Action("Импорт Excel", async () => await ImportExcel(), 140);
        var add = Action("Добавить THU", async () => await AddThu(), 145);
        var ret = Action("Зарегистрировать возврат", async () => await ManualReturn(), 210);
        var status = Action("Изменить статус", EditSelectedStatus, 160);
        var export = Action("Выгрузить", Export, 120);
        foreach (var b in new[] { import, add, ret, status, export })
        {
            b.Height = 42;
            b.Radius = 20;
            b.BorderWidth = 1;
            b.Margin = new Padding(0, 0, 10, 0);
            host.Controls.Add(b);
        }
        return host;
    }'''
s = replace_between(s, "    private Control BuildMinimalToolbar()\n", "    private Control BuildMinimalSearchRow()\n", toolbar)

# Search/filter row follows the same 42px / 20px geometry as the buttons.
search_row = r'''    private Control BuildMinimalSearchRow()
    {
        var host = new Panel { Dock = DockStyle.Fill, Margin = Padding.Empty };
        var searchCard = new RoundedPanel
        {
            Dock = DockStyle.Left,
            Width = 560,
            Height = 42,
            Padding = new Padding(16, 8, 12, 5),
            Radius = 20,
            BorderWidth = 1,
            BorderColor = UiPalette.Emerald,
            Margin = Padding.Empty
        };
        _search.BorderStyle = BorderStyle.None;
        _search.Font = new Font("Segoe UI", 10.5f);
        _search.Dock = DockStyle.Fill;
        _search.TextChanged += (_, _) => Render();
        searchCard.Controls.Add(_search);
        host.Controls.Add(searchCard);

        _filter.Width = 180;
        _filter.Height = 42;
        _filter.Font = new Font("Segoe UI", 10f);
        _filter.FlatStyle = FlatStyle.Flat;
        if (_filter.Items.Count == 0)
        {
            _filter.Items.AddRange(new object[] { "Все статусы", "Отправлен", "В пути", "Возвращён", "Потерян", "Просрочен" });
            _filter.SelectedIndex = 0;
            _filter.SelectedIndexChanged += (_, _) => Render();
        }
        _filter.Location = new Point(574, 0);
        host.Controls.Add(_filter);

        var refresh = Action("↻", Reload, 48);
        refresh.Radius = 20;
        refresh.Anchor = AnchorStyles.Top | AnchorStyles.Right;
        refresh.Location = new Point(Math.Max(0, host.Width - refresh.Width), 0);
        host.Controls.Add(refresh);
        host.Resize += (_, _) => refresh.Left = host.Width - refresh.Width;
        return host;
    }'''
s = replace_between(s, "    private Control BuildMinimalSearchRow()\n", "    private Control BuildBrandHeader()\n", search_row)

# Settings window: a centered 390px content column. All action buttons are centered
# and use the same dimensions instead of inheriting FlowLayoutPanel left alignment.
settings = r'''    private void ShowSettings()
    {
        using var f = CreateDialog("Настройки", 540, 660);

        var outer = new TableLayoutPanel
        {
            Dock = DockStyle.Fill,
            ColumnCount = 3,
            RowCount = 1,
            Padding = new Padding(0, 18, 0, 18),
            Margin = Padding.Empty
        };
        outer.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 50));
        outer.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute, 390));
        outer.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 50));

        var p = new FlowLayoutPanel
        {
            Dock = DockStyle.Fill,
            FlowDirection = FlowDirection.TopDown,
            WrapContents = false,
            AutoScroll = true,
            Padding = Padding.Empty,
            Margin = Padding.Empty
        };

        var title = new Label
        {
            Text = "Настройки TubeControl",
            Width = 390,
            Height = 42,
            TextAlign = ContentAlignment.MiddleCenter,
            Font = new Font("Segoe UI", 16, FontStyle.Bold),
            Margin = new Padding(0, 0, 0, 12)
        };
        p.Controls.Add(title);

        p.Controls.Add(new Label
        {
            Text = "Оформление",
            Width = 390,
            Height = 28,
            TextAlign = ContentAlignment.MiddleLeft,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            Margin = new Padding(0, 2, 0, 8)
        });

        var themes = new FlowLayoutPanel
        {
            Width = 390,
            Height = 46,
            FlowDirection = FlowDirection.LeftToRight,
            WrapContents = false,
            Padding = Padding.Empty,
            Margin = new Padding(0, 0, 0, 14)
        };
        var dark = DialogButton("Тёмная");
        var light = DialogButton("Светлая");
        dark.Width = 190;
        light.Width = 190;
        dark.Height = light.Height = 42;
        dark.Radius = light.Radius = 20;
        dark.Margin = new Padding(0, 0, 10, 0);
        light.Margin = Padding.Empty;
        dark.Click += (_, _) => { SetTheme("dark"); ApplyThemeTo(f); };
        light.Click += (_, _) => { SetTheme("light"); ApplyThemeTo(f); };
        themes.Controls.Add(dark);
        themes.Controls.Add(light);
        p.Controls.Add(themes);

        p.Controls.Add(new Label
        {
            Text = "Просрочено",
            Width = 390,
            Height = 28,
            TextAlign = ContentAlignment.MiddleLeft,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            Margin = new Padding(0, 2, 0, 4)
        });
        p.Controls.Add(new Label
        {
            Text = "Через сколько дней после отправки тубус автоматически отмечается как просроченный:",
            Width = 390,
            Height = 42,
            AutoSize = false,
            Font = new Font("Segoe UI", 9f),
            Margin = new Padding(0, 0, 0, 6)
        });

        var overdueHost = new Panel { Width = 390, Height = 40, Margin = new Padding(0, 0, 0, 14) };
        var overdue = new NumericUpDown
        {
            Width = 150,
            Height = 34,
            Minimum = 1,
            Maximum = 365,
            Value = Math.Clamp(_config.OverdueDays, 1, 365),
            Font = new Font("Segoe UI", 10),
            Location = new Point((390 - 150) / 2, 2)
        };
        overdueHost.Controls.Add(overdue);
        p.Controls.Add(overdueHost);

        p.Controls.Add(new Label
        {
            Text = "Android устройства",
            Width = 390,
            Height = 28,
            TextAlign = ContentAlignment.MiddleLeft,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            Margin = new Padding(0, 2, 0, 8)
        });

        GlowButton CenteredButton(string text)
        {
            var b = DialogButton(text);
            b.Width = 320;
            b.Height = 42;
            b.Radius = 20;
            b.BorderWidth = 1;
            b.Margin = new Padding(35, 0, 35, 10);
            return b;
        }

        var pair = CenteredButton("Подключить Android по QR-коду");
        var devices = CenteredButton("Управление подключёнными устройствами");
        devices.Margin = new Padding(35, 0, 35, 18);
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

        var save = CenteredButton("Сохранить настройки");
        save.Tag = "primary";
        save.FillColor = UiPalette.EmeraldBright;
        save.HoverFillColor = Color.FromArgb(45, 245, 177);
        save.PressedFillColor = UiPalette.Emerald;
        save.BorderColor = UiPalette.Mint;
        save.ForeColor = Color.FromArgb(0, 28, 18);
        save.Margin = new Padding(35, 0, 35, 0);
        save.Click += (_, _) =>
        {
            _config.OverdueDays = (int)overdue.Value;
            _store.Save(_config);
            Render();
            f.Close();
        };
        p.Controls.Add(save);

        outer.Controls.Add(p, 1, 0);
        f.Controls.Add(outer);
        ApplyThemeTo(f);
        f.ShowDialog(this);
    }'''
s = replace_between(s, "    private void ShowSettings()\n", "    private string? Prompt", settings)

# Preserve a primary emerald button in dialogs instead of recoloring every GlowButton
# to the ordinary panel background during recursive theme application.
old_theme = '''            else if (gb != _saveStatus && gb != _themeDark && gb != _themeLight)
            {
                gb.FillColor = Panel;
                gb.HoverFillColor = Panel2;
                gb.PressedFillColor = DarkTheme ? Color.FromArgb(11, 67, 50) : Color.FromArgb(210, 238, 226);
                gb.BorderColor = UiPalette.Emerald;
                gb.ForeColor = Fg;
            }'''
new_theme = '''            else if (gb.Tag is string buttonTag && buttonTag == "primary")
            {
                gb.FillColor = UiPalette.EmeraldBright;
                gb.HoverFillColor = Color.FromArgb(45, 245, 177);
                gb.PressedFillColor = UiPalette.Emerald;
                gb.BorderColor = UiPalette.Mint;
                gb.ForeColor = Color.FromArgb(0, 28, 18);
                gb.BorderWidth = 1;
            }
            else if (gb != _saveStatus && gb != _themeDark && gb != _themeLight)
            {
                gb.FillColor = Panel;
                gb.HoverFillColor = Panel2;
                gb.PressedFillColor = DarkTheme ? Color.FromArgb(11, 67, 50) : Color.FromArgb(210, 238, 226);
                gb.BorderColor = UiPalette.Emerald;
                gb.ForeColor = Fg;
                gb.BorderWidth = 1;
            }'''
s = s.replace(old_theme, new_theme)

# ---------------------------------------------------------------------------
# Smooth custom button rendering.  No Region clipping: we explicitly repaint the
# parent's solid background, then draw one anti-aliased 1px path inside it. This
# removes the uneven/double outline and the small white/light artifacts seen at
# rounded corners on Windows.
# ---------------------------------------------------------------------------
glow = r'''internal sealed class GlowButton : Control
{
    private int _radius = 20;
    private bool _hover;
    private bool _pressed;

    public int Radius
    {
        get => _radius;
        set { _radius = Math.Max(2, value); Invalidate(); }
    }

    public Color BorderColor { get; set; } = UiPalette.Emerald;
    public Color FillColor { get; set; } = UiPalette.DarkPanel;
    public Color HoverFillColor { get; set; } = UiPalette.DarkPanel2;
    public Color PressedFillColor { get; set; } = Color.FromArgb(12, 70, 53);
    public int BorderWidth { get; set; } = 1;
    public ContentAlignment TextAlign { get; set; } = ContentAlignment.MiddleCenter;

    public GlowButton()
    {
        SetStyle(ControlStyles.UserPaint |
                 ControlStyles.AllPaintingInWmPaint |
                 ControlStyles.OptimizedDoubleBuffer |
                 ControlStyles.ResizeRedraw |
                 ControlStyles.Selectable, true);
        ForeColor = Color.White;
        Cursor = Cursors.Hand;
        TabStop = true;
        Font = new Font("Segoe UI", 10f, FontStyle.Regular);
    }

    protected override void OnPaintBackground(PaintEventArgs e)
    {
        using var bg = new SolidBrush(Parent?.BackColor ?? UiPalette.Dark);
        e.Graphics.FillRectangle(bg, ClientRectangle);
    }

    protected override void OnMouseEnter(EventArgs e) { _hover = true; Invalidate(); base.OnMouseEnter(e); }
    protected override void OnMouseLeave(EventArgs e) { _hover = false; _pressed = false; Invalidate(); base.OnMouseLeave(e); }

    protected override void OnMouseDown(MouseEventArgs e)
    {
        if (e.Button == MouseButtons.Left && Enabled)
        {
            _pressed = true;
            Capture = true;
            Invalidate();
        }
        base.OnMouseDown(e);
    }

    protected override void OnMouseUp(MouseEventArgs e)
    {
        var fire = _pressed && e.Button == MouseButtons.Left && ClientRectangle.Contains(e.Location) && Enabled;
        _pressed = false;
        Capture = false;
        Invalidate();
        base.OnMouseUp(e);
        if (fire) OnClick(EventArgs.Empty);
    }

    protected override void OnKeyDown(KeyEventArgs e)
    {
        if (Enabled && (e.KeyCode == Keys.Space || e.KeyCode == Keys.Enter))
        {
            _pressed = true;
            Invalidate();
            e.Handled = true;
        }
        base.OnKeyDown(e);
    }

    protected override void OnKeyUp(KeyEventArgs e)
    {
        if (_pressed && Enabled && (e.KeyCode == Keys.Space || e.KeyCode == Keys.Enter))
        {
            _pressed = false;
            Invalidate();
            OnClick(EventArgs.Empty);
            e.Handled = true;
        }
        base.OnKeyUp(e);
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        g.PixelOffsetMode = PixelOffsetMode.HighQuality;
        g.CompositingQuality = CompositingQuality.HighQuality;

        using (var bg = new SolidBrush(Parent?.BackColor ?? UiPalette.Dark))
            g.FillRectangle(bg, ClientRectangle);

        var rect = new RectangleF(1.5f, 1.5f, Math.Max(1f, Width - 3f), Math.Max(1f, Height - 3f));
        var radius = Math.Min(Radius, (int)(rect.Height / 2f));
        using var path = RoundedFloat(rect, radius);

        var fillColor = !Enabled
            ? Color.FromArgb(35, 45, 42)
            : _pressed ? PressedFillColor : _hover ? HoverFillColor : FillColor;
        using var fill = new SolidBrush(fillColor);
        g.FillPath(fill, path);

        if (BorderWidth > 0)
        {
            using var pen = new Pen(Enabled ? BorderColor : Color.FromArgb(80, 90, 86), 1f)
            {
                Alignment = PenAlignment.Inset,
                LineJoin = LineJoin.Round
            };
            g.DrawPath(pen, path);
        }

        var textRect = Rectangle.Round(RectangleF.Inflate(rect, -14f, 0f));
        var flags = TextFormatFlags.VerticalCenter | TextFormatFlags.EndEllipsis | TextFormatFlags.NoPadding;
        if (TextAlign is ContentAlignment.MiddleLeft or ContentAlignment.TopLeft or ContentAlignment.BottomLeft)
            flags |= TextFormatFlags.Left;
        else if (TextAlign is ContentAlignment.MiddleRight or ContentAlignment.TopRight or ContentAlignment.BottomRight)
            flags |= TextFormatFlags.Right;
        else
            flags |= TextFormatFlags.HorizontalCenter;
        TextRenderer.DrawText(g, Text, Font, textRect, Enabled ? ForeColor : Color.Gray, flags);
    }

    private static GraphicsPath RoundedFloat(RectangleF r, float radius)
    {
        var p = new GraphicsPath();
        var rr = Math.Max(1f, Math.Min(radius, Math.Min(r.Width, r.Height) / 2f));
        var d = rr * 2f;
        p.AddArc(r.X, r.Y, d, d, 180, 90);
        p.AddArc(r.Right - d, r.Y, d, d, 270, 90);
        p.AddArc(r.Right - d, r.Bottom - d, d, d, 0, 90);
        p.AddArc(r.X, r.Bottom - d, d, d, 90, 90);
        p.CloseFigure();
        return p;
    }
}'''
u = replace_between(u, "internal sealed class GlowButton : Control\n", "internal sealed class TubeIllustration", glow)

main_path.write_text(s, encoding="utf-8", newline="\n")
ui_path.write_text(u, encoding="utf-8", newline="\n")
