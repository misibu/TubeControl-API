using QRCoder;
using System.Drawing.Drawing2D;
using System.Runtime.InteropServices;
using System.Text.Json;

namespace TubeControl.Windows;

public sealed class MainForm : Form
{
    private readonly ConfigStore _store;
    private readonly AppConfig _config;
    private readonly ApiClient _api;
    private readonly List<TubeItem> _all = new();
    private readonly List<TubeItem> _visible = new();

    private readonly DataGridView _grid = new();
    private readonly TextBox _search = new() { PlaceholderText = "Поиск по THU, заказу или магазину…" };
    private readonly ComboBox _filter = new() { DropDownStyle = ComboBoxStyle.DropDownList };
    private readonly Label _state = new() { AutoSize = true };
    private readonly Label _recordCount = new() { AutoSize = true };

    private readonly RoundedPanel _details = new() { Dock = DockStyle.Fill, Margin = new Padding(8, 12, 12, 12), Padding = new Padding(18) };
    private readonly Label _detailThu = new() { AutoSize = true, Font = new Font("Segoe UI", 19, FontStyle.Bold) };
    private readonly Label _orderValue = new() { AutoSize = true };
    private readonly Label _storeValue = new() { AutoSize = true };
    private readonly Label _dateValue = new() { AutoSize = true };
    private readonly Label _currentStatus = new() { AutoSize = true, Font = new Font("Segoe UI", 10, FontStyle.Bold) };
    private readonly ComboBox _status = new() { DropDownStyle = ComboBoxStyle.DropDownList, DrawMode = DrawMode.OwnerDrawFixed, ItemHeight = 28 };
    private readonly TextBox _comment = new() { Multiline = true, PlaceholderText = "Добавить комментарий…", MaxLength = 500, BorderStyle = BorderStyle.FixedSingle };
    private readonly Label _commentCount = new() { AutoSize = true, Text = "0/500" };
    private readonly GlowButton _saveStatus = new() { Text = "Сохранить изменения", Height = 44, Dock = DockStyle.Top, Font = new Font("Segoe UI", 10, FontStyle.Bold) };
    private readonly TubeIllustration _tubeArt = new() { Width = 126, Height = 94, Anchor = AnchorStyles.Top | AnchorStyles.Right };
    private readonly GlowButton _themeLight = new() { Text = "☀  Светлая", Width = 108, Height = 36, Radius = 18 };
    private readonly GlowButton _themeDark = new() { Text = "☾  Тёмная", Width = 108, Height = 36, Radius = 18 };
    private readonly Label _userName = new() { AutoSize = true, Font = new Font("Segoe UI", 9, FontStyle.Bold) };
    private readonly Label _license = new() { AutoSize = true, Text = "Устройство активировано", Font = new Font("Segoe UI", 8) };
    private readonly Panel _chrome = new() { Dock = DockStyle.Top, Height = 38 };

    private Rectangle _restoreBounds;
    private bool _maximized = true;
    private Button? _maxButton;

    private bool DarkTheme => _config.Theme != "light";
    private Color Bg => DarkTheme ? UiPalette.Dark : UiPalette.Light;
    private Color Panel => DarkTheme ? UiPalette.DarkPanel : UiPalette.LightPanel;
    private Color Panel2 => DarkTheme ? UiPalette.DarkPanel2 : Color.FromArgb(234, 244, 239);
    private Color Border => DarkTheme ? UiPalette.DarkBorder : UiPalette.LightBorder;
    private Color Fg => DarkTheme ? Color.White : Color.FromArgb(24, 45, 38);
    private Color Muted => DarkTheme ? Color.FromArgb(164, 184, 177) : Color.FromArgb(78, 101, 92);

    public MainForm(ConfigStore store, AppConfig config, ApiClient api)
    {
        _store = store;
        _config = config;
        _api = api;

        Text = "TubeControl";
        MinimumSize = new Size(1180, 720);
        StartPosition = FormStartPosition.Manual;
        FormBorderStyle = FormBorderStyle.None;
        BackColor = UiPalette.Dark;
        Icon = Icon.ExtractAssociatedIcon(Application.ExecutablePath);
        DoubleBuffered = true;

        BuildUi();
        ApplyTheme();

        Shown += async (_, _) =>
        {
            _restoreBounds = new Rectangle(80, 60, 1420, 860);
            MaximizeToWorkArea();
            await Reload();
        };
    }

    private void BuildUi()
    {
        Controls.Add(BuildBody());
        Controls.Add(BuildChrome());
    }

    private Control BuildChrome()
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

        var brand = new Label
        {
            Text = "TubeControl",
            AutoSize = true,
            Font = new Font("Segoe UI", 9, FontStyle.Bold),
            ForeColor = Color.FromArgb(100, 145, 130),
            Location = new Point(16, 10)
        };
        _chrome.Controls.Add(brand);
        return _chrome;
    }

    private Button WindowButton(string text, EventHandler click, bool close = false)
    {
        var b = new Button
        {
            Text = text,
            Width = 46,
            Height = 38,
            FlatStyle = FlatStyle.Flat,
            BackColor = Color.Transparent,
            ForeColor = Color.FromArgb(190, 210, 203),
            Font = new Font("Segoe UI", 12),
            TabStop = false,
            Margin = Padding.Empty
        };
        b.FlatAppearance.BorderSize = 0;
        b.FlatAppearance.MouseOverBackColor = close ? Color.FromArgb(180, 45, 55) : Color.FromArgb(18, 45, 37);
        b.FlatAppearance.MouseDownBackColor = close ? Color.FromArgb(205, 45, 55) : Color.FromArgb(23, 58, 47);
        b.Click += click;
        return b;
    }

    private Control BuildBody()
    {
        var shell = new TableLayoutPanel
        {
            Dock = DockStyle.Fill,
            ColumnCount = 3,
            RowCount = 2,
            Padding = Padding.Empty,
            Margin = Padding.Empty
        };
        shell.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute, 205));
        shell.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 100));
        shell.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute, 350));
        shell.RowStyles.Add(new RowStyle(SizeType.Absolute, 104));
        shell.RowStyles.Add(new RowStyle(SizeType.Percent, 100));

        shell.Controls.Add(BuildBrandHeader(), 0, 0);
        shell.Controls.Add(BuildTopHeader(), 1, 0);
        shell.Controls.Add(BuildAccountHeader(), 2, 0);
        shell.Controls.Add(BuildSidebar(), 0, 1);
        shell.Controls.Add(BuildCenter(), 1, 1);
        shell.Controls.Add(BuildDetails(), 2, 1);
        return shell;
    }

    private Control BuildBrandHeader()
    {
        var p = new Panel { Dock = DockStyle.Fill, Padding = new Padding(22, 15, 8, 4) };

        var tube = new Label
        {
            Text = "Tube",
            AutoSize = true,
            Font = new Font("Segoe UI", 25, FontStyle.Bold),
            Location = new Point(20, 16)
        };
        var control = new Label
        {
            Text = "Control",
            AutoSize = true,
            Font = new Font("Segoe UI", 25, FontStyle.Bold),
            ForeColor = UiPalette.EmeraldBright,
            Location = new Point(91, 16)
        };
        var tag = new Label
        {
            Text = "T R A C K   ·   R E T U R N   ·   U N D E R   C O N T R O L",
            AutoSize = true,
            Font = new Font("Segoe UI", 6.4f, FontStyle.Bold),
            ForeColor = Color.FromArgb(135, 161, 151),
            Location = new Point(23, 66)
        };
        p.Controls.AddRange(new Control[] { tube, control, tag });
        return p;
    }

    private Control BuildTopHeader()
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
        actions.Controls.Add(Action("⌁  Подключить Android", async () => await PairAndroid(), 174));
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
    }

    private Control BuildAccountHeader()
    {
        var p = new Panel { Dock = DockStyle.Fill, Padding = new Padding(6, 13, 15, 8) };

        var themes = new FlowLayoutPanel
        {
            Dock = DockStyle.Top,
            Height = 42,
            FlowDirection = FlowDirection.LeftToRight,
            WrapContents = false,
            Padding = Padding.Empty
        };
        _themeLight.Click += (_, _) => SetTheme("light");
        _themeDark.Click += (_, _) => SetTheme("dark");
        themes.Controls.Add(_themeLight);
        themes.Controls.Add(_themeDark);
        p.Controls.Add(themes);

        _userName.Text = string.IsNullOrWhiteSpace(_config.DeviceName) ? Environment.MachineName : _config.DeviceName;
        _userName.Location = new Point(232, 16);
        _license.Location = new Point(232, 39);
        p.Controls.Add(_userName);
        p.Controls.Add(_license);
        return p;
    }

    private Control BuildSidebar()
    {
        var side = new Panel { Dock = DockStyle.Fill, Padding = new Padding(0, 22, 0, 12) };
        var menu = new FlowLayoutPanel
        {
            Dock = DockStyle.Top,
            Height = 310,
            FlowDirection = FlowDirection.TopDown,
            WrapContents = false,
            Padding = Padding.Empty
        };

        menu.Controls.Add(Nav("◇   Трубы (THU)", true, () => { _filter.SelectedIndex = 0; Render(); }));
        menu.Controls.Add(Nav("↶   Возвраты", false, () => { _filter.SelectedItem = "Возвращён"; }));
        menu.Controls.Add(Nav("▥   Отчёты", false, Export));
        menu.Controls.Add(Nav("⌁   Устройства", false, async () => await Devices()));
        menu.Controls.Add(Nav("⚙   Настройки", false, () => ShowSettings()));
        side.Controls.Add(menu);

        var slogan = new Panel { Dock = DockStyle.Bottom, Height = 120, Padding = new Padding(20, 12, 15, 10) };
        var leaf = new Label { Text = "♧", AutoSize = true, Font = new Font("Segoe UI Symbol", 28), ForeColor = UiPalette.Mint, Location = new Point(20, 2) };
        var txt = new Label { Text = "М Е Н Ь Ш Е   О Т Х О Д О В .\nБ О Л Ь Ш Е   К О Н Т Р О Л Я .", AutoSize = true, Font = new Font("Segoe UI", 7.2f), Location = new Point(20, 53) };
        slogan.Controls.Add(leaf);
        slogan.Controls.Add(txt);
        side.Controls.Add(slogan);
        return side;
    }

    private GlowButton Nav(string text, bool active, Action click)
    {
        var b = new GlowButton
        {
            Text = text,
            Width = 205,
            Height = 54,
            Radius = 0,
            BorderWidth = active ? 1 : 0,
            BorderColor = active ? UiPalette.EmeraldBright : Color.Transparent,
            FillColor = active ? Color.FromArgb(9, 55, 42) : Color.Transparent,
            HoverFillColor = Color.FromArgb(9, 47, 37),
            ForeColor = active ? Color.White : Color.FromArgb(205, 220, 214),
            TextAlign = ContentAlignment.MiddleLeft,
            Font = new Font("Segoe UI", 10, active ? FontStyle.Bold : FontStyle.Regular),
            Margin = Padding.Empty
        };
        b.Tag = active ? "nav-active" : "nav";
        b.Click += (_, _) =>
        {
            try { click(); }
            catch (Exception ex) { ShowError(ex); }
        };
        return b;
    }

    private GlowButton Nav(string text, bool active, Func<Task> click)
    {
        var b = Nav(text, active, () => { });
        b.Click += async (_, _) =>
        {
            try { await click(); }
            catch (Exception ex) { ShowError(ex); }
        };
        return b;
    }

    private Control BuildCenter()
    {
        var center = new TableLayoutPanel
        {
            Dock = DockStyle.Fill,
            RowCount = 4,
            ColumnCount = 1,
            Padding = new Padding(12, 12, 8, 12)
        };
        center.RowStyles.Add(new RowStyle(SizeType.Absolute, 56));
        center.RowStyles.Add(new RowStyle(SizeType.Percent, 100));
        center.RowStyles.Add(new RowStyle(SizeType.Absolute, 54));
        center.RowStyles.Add(new RowStyle(SizeType.Absolute, 86));

        center.Controls.Add(BuildSearchTools(), 0, 0);
        center.Controls.Add(BuildGridCard(), 0, 1);
        center.Controls.Add(BuildPager(), 0, 2);
        center.Controls.Add(BuildBanner(), 0, 3);
        return center;
    }

    private Control BuildSearchTools()
    {
        var host = new Panel { Dock = DockStyle.Fill };

        var searchCard = new RoundedPanel
        {
            Dock = DockStyle.Left,
            Width = 455,
            Height = 44,
            Margin = Padding.Empty,
            Padding = new Padding(14, 7, 12, 5),
            Radius = 11
        };
        var icon = new Label { Text = "⌕", AutoSize = true, Font = new Font("Segoe UI Symbol", 16), Dock = DockStyle.Left, Width = 30, TextAlign = ContentAlignment.MiddleCenter };
        _search.BorderStyle = BorderStyle.None;
        _search.Font = new Font("Segoe UI", 10);
        _search.Dock = DockStyle.Fill;
        _search.TextChanged += (_, _) => Render();
        searchCard.Controls.Add(_search);
        searchCard.Controls.Add(icon);
        host.Controls.Add(searchCard);

        _filter.Width = 165;
        _filter.Height = 38;
        _filter.Items.AddRange(new object[] { "Все статусы", "Отправлен", "В пути", "Возвращён", "Потерян", "Просрочен" });
        _filter.SelectedIndex = 0;
        _filter.SelectedIndexChanged += (_, _) => Render();
        _filter.Location = new Point(468, 3);
        host.Controls.Add(_filter);

        var import = Action("⇧  Импорт Excel", async () => await ImportExcel(), 136);
        import.Anchor = AnchorStyles.Top | AnchorStyles.Right;
        import.Location = new Point(Math.Max(642, host.Width - 140), 1);
        import.Margin = Padding.Empty;
        host.Controls.Add(import);
        host.Resize += (_, _) => import.Left = host.Width - import.Width;

        return host;
    }

    private Control BuildGridCard()
    {
        var card = new RoundedPanel { Dock = DockStyle.Fill, Radius = 13, Padding = new Padding(1), Margin = new Padding(0, 0, 0, 0) };
        _grid.Dock = DockStyle.Fill;
        _grid.ReadOnly = true;
        _grid.AllowUserToAddRows = false;
        _grid.AllowUserToDeleteRows = false;
        _grid.AllowUserToResizeRows = false;
        _grid.SelectionMode = DataGridViewSelectionMode.FullRowSelect;
        _grid.MultiSelect = false;
        _grid.AutoSizeColumnsMode = DataGridViewAutoSizeColumnsMode.Fill;
        _grid.RowHeadersVisible = false;
        _grid.BorderStyle = BorderStyle.None;
        _grid.BackgroundColor = Color.Transparent;
        _grid.CellBorderStyle = DataGridViewCellBorderStyle.SingleHorizontal;
        _grid.ColumnHeadersBorderStyle = DataGridViewHeaderBorderStyle.None;
        _grid.ColumnHeadersHeight = 46;
        _grid.ColumnHeadersHeightSizeMode = DataGridViewColumnHeadersHeightSizeMode.DisableResizing;
        _grid.RowTemplate.Height = 43;
        _grid.EnableHeadersVisualStyles = false;
        _grid.SelectionChanged += (_, _) => ShowSelected();
        _grid.CellPainting += GridCellPainting;

        _grid.Columns.Add(new DataGridViewCheckBoxColumn { Name = "Check", HeaderText = "", FillWeight = 8, ReadOnly = true });
        _grid.Columns.Add("THU", "THU");
        _grid.Columns.Add("Order", "№ заказа");
        _grid.Columns.Add("Store", "Магазин");
        _grid.Columns.Add("Date", "Дата отправки");
        _grid.Columns.Add("Status", "Статус");
        _grid.Columns["THU"]!.FillWeight = 22;
        _grid.Columns["Order"]!.FillWeight = 22;
        _grid.Columns["Store"]!.FillWeight = 28;
        _grid.Columns["Date"]!.FillWeight = 24;
        _grid.Columns["Status"]!.FillWeight = 25;

        card.Controls.Add(_grid);
        return card;
    }

    private Control BuildPager()
    {
        var host = new Panel { Dock = DockStyle.Fill, Padding = new Padding(2, 12, 2, 2) };
        _recordCount.Text = "Показано 0 из 0";
        _recordCount.Location = new Point(0, 17);
        host.Controls.Add(_recordCount);

        var page = new Label
        {
            Text = "1     2     3     4     5     …     49",
            AutoSize = true,
            Font = new Font("Segoe UI", 9),
            Anchor = AnchorStyles.Top | AnchorStyles.Right
        };
        page.Location = new Point(440, 18);
        host.Controls.Add(page);
        host.Resize += (_, _) => page.Left = Math.Max(200, host.Width - page.Width - 10);
        return host;
    }

    private Control BuildBanner()
    {
        var banner = new RoundedPanel
        {
            Dock = DockStyle.Fill,
            Radius = 14,
            Margin = new Padding(0, 3, 0, 0),
            BackColor = Color.FromArgb(5, 31, 23),
            BorderColor = Color.FromArgb(22, 88, 68)
        };
        var leaf = new Label { Text = "♧", AutoSize = true, Font = new Font("Segoe UI Symbol", 34), ForeColor = UiPalette.Mint, Location = new Point(22, 15) };
        var text = new Label
        {
            Text = "С Л Е Д И .   В О З В Р А Щ А Й .   С О З Д А В А Й   Б О Л Е Е   З А М К Н У Т О Е   Б У Д У Щ Е Е .",
            AutoSize = true,
            Font = new Font("Segoe UI", 7.3f, FontStyle.Bold),
            ForeColor = Color.FromArgb(213, 232, 224),
            Location = new Point(88, 31)
        };
        banner.Controls.Add(leaf);
        banner.Controls.Add(text);
        return banner;
    }

    private Control BuildDetails()
    {
        var stack = new TableLayoutPanel
        {
            Dock = DockStyle.Fill,
            ColumnCount = 1,
            RowCount = 8,
            Padding = new Padding(4),
            BackColor = Color.Transparent
        };
        stack.RowStyles.Add(new RowStyle(SizeType.Absolute, 112));
        stack.RowStyles.Add(new RowStyle(SizeType.Absolute, 124));
        stack.RowStyles.Add(new RowStyle(SizeType.Absolute, 36));
        stack.RowStyles.Add(new RowStyle(SizeType.Absolute, 42));
        stack.RowStyles.Add(new RowStyle(SizeType.Absolute, 34));
        stack.RowStyles.Add(new RowStyle(SizeType.Absolute, 94));
        stack.RowStyles.Add(new RowStyle(SizeType.Absolute, 48));
        stack.RowStyles.Add(new RowStyle(SizeType.Percent, 100));

        var header = new Panel { Dock = DockStyle.Fill };
        _detailThu.Location = new Point(4, 14);
        _tubeArt.Location = new Point(185, 2);
        header.Controls.Add(_detailThu);
        header.Controls.Add(_tubeArt);
        stack.Controls.Add(header, 0, 0);

        var info = new TableLayoutPanel { Dock = DockStyle.Fill, ColumnCount = 2, RowCount = 4, Padding = new Padding(4, 2, 4, 4) };
        info.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 42));
        info.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 58));
        for (var i = 0; i < 4; i++) info.RowStyles.Add(new RowStyle(SizeType.Percent, 25));
        AddInfoRow(info, 0, "№ заказа", _orderValue);
        AddInfoRow(info, 1, "Магазин", _storeValue);
        AddInfoRow(info, 2, "Дата отправки", _dateValue);
        AddInfoRow(info, 3, "Текущий статус", _currentStatus);
        stack.Controls.Add(info, 0, 1);

        stack.Controls.Add(new Label
        {
            Text = "Изменить статус",
            AutoSize = true,
            Font = new Font("Segoe UI", 10, FontStyle.Bold),
            Margin = new Padding(4, 8, 0, 0)
        }, 0, 2);

        _status.Dock = DockStyle.Fill;
        _status.Items.AddRange(new object[] { "Автоматически", "Отправлен", "В пути", "Возвращён", "Потерян" });
        _status.SelectedIndex = 0;
        _status.DrawItem += StatusDrawItem;
        stack.Controls.Add(_status, 0, 3);

        stack.Controls.Add(new Label
        {
            Text = "Комментарий",
            AutoSize = true,
            Font = new Font("Segoe UI", 9, FontStyle.Bold),
            Margin = new Padding(4, 9, 0, 0)
        }, 0, 4);

        var commentHost = new Panel { Dock = DockStyle.Fill, Padding = new Padding(2) };
        _comment.Dock = DockStyle.Fill;
        _comment.TextChanged += (_, _) => _commentCount.Text = $"{_comment.TextLength}/500";
        _commentCount.Anchor = AnchorStyles.Right | AnchorStyles.Bottom;
        _commentCount.Location = new Point(270, 70);
        commentHost.Controls.Add(_comment);
        commentHost.Controls.Add(_commentCount);
        stack.Controls.Add(commentHost, 0, 5);

        _saveStatus.FillColor = UiPalette.EmeraldBright;
        _saveStatus.HoverFillColor = Color.FromArgb(45, 245, 177);
        _saveStatus.PressedFillColor = UiPalette.Emerald;
        _saveStatus.BorderColor = UiPalette.Mint;
        _saveStatus.ForeColor = Color.FromArgb(0, 24, 16);
        _saveStatus.Click += async (_, _) => await Safe(SaveStatus);
        stack.Controls.Add(_saveStatus, 0, 6);

        var hint = new Label
        {
            Text = "Выберите строку в таблице, чтобы посмотреть данные тубуса и изменить его статус вручную.",
            Dock = DockStyle.Top,
            AutoSize = false,
            Height = 72,
            TextAlign = ContentAlignment.MiddleLeft,
            Padding = new Padding(4)
        };
        stack.Controls.Add(hint, 0, 7);

        _details.Controls.Add(stack);
        return _details;
    }

    private void AddInfoRow(TableLayoutPanel table, int row, string key, Label value)
    {
        var k = new Label { Text = key, AutoSize = true, Anchor = AnchorStyles.Left, Font = new Font("Segoe UI", 8.8f) };
        value.Anchor = AnchorStyles.Left;
        value.Font = new Font("Segoe UI", 8.8f);
        table.Controls.Add(k, 0, row);
        table.Controls.Add(value, 1, row);
    }

    private GlowButton Action(string text, Func<Task> fn, int width)
    {
        var b = new GlowButton
        {
            Text = text,
            Width = width,
            Height = 40,
            Font = new Font("Segoe UI", 9),
            Margin = new Padding(0, 0, 8, 0),
            Radius = 10
        };
        b.Click += async (_, _) => await Safe(fn, b);
        return b;
    }

    private GlowButton Action(string text, Action fn, int width) =>
        Action(text, () => { fn(); return Task.CompletedTask; }, width);

    private async Task Safe(Func<Task> fn, Control? disable = null)
    {
        try
        {
            if (disable is not null) disable.Enabled = false;
            await fn();
        }
        catch (Exception ex) { ShowError(ex); }
        finally { if (disable is not null) disable.Enabled = true; }
    }

    private void ShowError(Exception ex) =>
        MessageBox.Show(this, ex.Message, "TubeControl", MessageBoxButtons.OK, MessageBoxIcon.Error);

    private async Task Reload()
    {
        _state.Text = "Синхронизация…";
        var r = await _api.GetTubesAsync(_config.DeviceToken);
        _all.Clear();
        _all.AddRange(r.Items);
        Render();
        _state.Text = $"Записей: {_all.Count}";
    }

    private void Render()
    {
        if (_grid.Columns.Count == 0) return;
        var q = _search.Text.Trim().ToLowerInvariant();
        var sf = FilterCode(_filter.SelectedItem?.ToString() ?? "Все статусы");
        _visible.Clear();
        _visible.AddRange(_all.Where(t =>
            (q.Length == 0 || $"{t.Thu} {t.StoreCode} {t.OrderNumber}".ToLowerInvariant().Contains(q))
            && (sf == "all" || t.Status == sf)));

        _grid.Rows.Clear();
        foreach (var t in _visible)
        {
            var idx = _grid.Rows.Add(false, t.Thu, t.OrderNumber, t.StoreCode, t.ShippedAt, ExcelService.StatusRu(t.Status));
            var row = _grid.Rows[idx];
            row.Tag = t;
            row.MinimumHeight = 43;
        }

        _recordCount.Text = $"Показано {_visible.Count} из {_all.Count}";
        ShowSelected();
    }

    private void GridCellPainting(object? sender, DataGridViewCellPaintingEventArgs e)
    {
        if (e.RowIndex < 0) return;
        if (e.ColumnIndex != _grid.Columns["Status"]!.Index) return;

        e.PaintBackground(e.CellBounds, true);
        var text = e.FormattedValue?.ToString() ?? "";
        var color = UiDrawing.StatusColor(text);
        var width = Math.Min(e.CellBounds.Width - 14, Math.Max(88, TextRenderer.MeasureText(text, _grid.Font).Width + 38));
        var pill = new Rectangle(e.CellBounds.X + 7, e.CellBounds.Y + 8, width, e.CellBounds.Height - 16);

        e.Graphics.SmoothingMode = SmoothingMode.AntiAlias;
        using var path = UiDrawing.Rounded(pill, pill.Height / 2);
        using var fill = new SolidBrush(Color.FromArgb(DarkTheme ? 45 : 28, color));
        using var pen = new Pen(Color.FromArgb(190, color), 1);
        e.Graphics.FillPath(fill, path);
        e.Graphics.DrawPath(pen, path);

        var dot = new Rectangle(pill.X + 10, pill.Y + pill.Height / 2 - 4, 8, 8);
        using var dotBrush = new SolidBrush(color);
        e.Graphics.FillEllipse(dotBrush, dot);
        var textRect = new Rectangle(pill.X + 25, pill.Y, pill.Width - 30, pill.Height);
        TextRenderer.DrawText(e.Graphics, text, _grid.Font, textRect, color,
            TextFormatFlags.Left | TextFormatFlags.VerticalCenter | TextFormatFlags.EndEllipsis);

        e.Handled = true;
    }

    private void ShowSelected()
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
        _currentStatus.Text = "●  " + ExcelService.StatusRu(t.Status);
        _currentStatus.ForeColor = UiDrawing.StatusColor(t.Status);

        _status.SelectedItem = StatusRuFromCode(t.ManualStatus ?? t.Status);
        if (_status.SelectedIndex < 0) _status.SelectedIndex = 0;
        _comment.Text = t.StatusComment ?? "";
    }

    private async Task SaveStatus()
    {
        if (_grid.SelectedRows.Count == 0 || _grid.SelectedRows[0].Tag is not TubeItem t) return;
        var code = StatusCode(_status.SelectedItem?.ToString() ?? "Автоматически");
        await _api.UpdateStatusAsync(_config.DeviceToken, t.Thu, code, _comment.Text.Trim());
        await Reload();
    }

    private async Task ImportExcel()
    {
        using var dlg = new OpenFileDialog
        {
            Filter = "Excel (*.xlsx;*.xlsm)|*.xlsx;*.xlsm",
            Title = "Выберите файл отгрузки"
        };
        if (dlg.ShowDialog(this) != DialogResult.OK) return;
        _state.Text = "Импорт Excel…";
        var items = ExcelService.ReadImport(dlg.FileName);
        await _api.ImportAsync(_config.DeviceToken, items);
        await Reload();
        MessageBox.Show(this, $"Импортировано: {items.Count}", "TubeControl");
    }

    private void Export()
    {
        var rows = _visible.Where(t => t.Status != "returned").ToList();
        using var dlg = new SaveFileDialog
        {
            Filter = "Excel (*.xlsx)|*.xlsx",
            FileName = $"TubeControl_не_возвращены_{DateTime.Now:yyyyMMdd_HHmm}.xlsx"
        };
        if (dlg.ShowDialog(this) != DialogResult.OK) return;
        ExcelService.Export(dlg.FileName, rows);
        MessageBox.Show(this, $"Выгружено: {rows.Count}", "TubeControl");
    }

    private async Task AddThu()
    {
        using var f = CreateDialog("Добавить THU", 420, 355);
        var thu = DialogTextBox("THU");
        var store = DialogTextBox("Код магазина");
        var order = DialogTextBox("№ заказа");
        var date = new DateTimePicker { Width = 320, Format = DateTimePickerFormat.Short };
        var ok = DialogButton("Добавить");

        var p = DialogStack();
        p.Controls.AddRange(new Control[] { LabelSmall("THU"), thu, LabelSmall("Код магазина"), store, LabelSmall("№ заказа"), order, LabelSmall("Дата отправки"), date, ok });
        f.Controls.Add(p);
        ok.Click += (_, _) => f.DialogResult = DialogResult.OK;
        ApplyThemeTo(f);

        if (f.ShowDialog(this) != DialogResult.OK || string.IsNullOrWhiteSpace(thu.Text)) return;
        await _api.ImportAsync(_config.DeviceToken, new[]
        {
            new ImportItem
            {
                Thu = thu.Text.Trim().ToUpperInvariant(),
                StoreCode = store.Text.Trim(),
                OrderNumber = order.Text.Trim(),
                ShippedAt = date.Value.ToString("yyyy-MM-dd")
            }
        });
        await Reload();
    }

    private async Task ManualReturn()
    {
        var value = Prompt("Зарегистрировать возврат", "Введите THU:");
        if (string.IsNullOrWhiteSpace(value)) return;
        await _api.UpdateStatusAsync(_config.DeviceToken, value.Trim().ToUpperInvariant(), "returned", "Возврат зарегистрирован вручную в Windows");
        await Reload();
    }

    private async Task PairAndroid()
    {
        var e = await _api.CreateEnrollmentAsync(_config.DeviceToken);
        var payload = JsonSerializer.Serialize(new { type = "tubecontrol-enroll-v2", server = _config.Server.TrimEnd('/'), code = e.Code });

        using var gen = new QRCodeGenerator();
        using var data = gen.CreateQrCode(payload, QRCodeGenerator.ECCLevel.Q);
        var png = new PngByteQRCode(data).GetGraphic(8);
        using var ms = new MemoryStream(png);
        using var img = Image.FromStream(ms);
        using var copy = new Bitmap(img);

        using var f = CreateDialog("Подключить Android", 500, 620);
        var label = new Label
        {
            Text = "Сканируйте QR-код в Android TubeControl.\nКод одноразовый и действует 10 минут.",
            AutoSize = true,
            TextAlign = ContentAlignment.MiddleCenter,
            Font = new Font("Segoe UI", 11, FontStyle.Bold)
        };
        var pic = new PictureBox { Image = new Bitmap(copy), Width = 390, Height = 390, SizeMode = PictureBoxSizeMode.Zoom, BackColor = Color.White };
        var p = DialogStack();
        p.Controls.Add(label);
        p.Controls.Add(pic);
        f.Controls.Add(p);
        ApplyThemeTo(f);
        f.ShowDialog(this);
    }

    private async Task Devices()
    {
        var data = await _api.GetDevicesAsync(_config.DeviceToken);
        using var f = CreateDialog("Устройства", 790, 510);
        var grid = new DataGridView
        {
            Dock = DockStyle.Fill,
            ReadOnly = true,
            SelectionMode = DataGridViewSelectionMode.FullRowSelect,
            AutoSizeColumnsMode = DataGridViewAutoSizeColumnsMode.Fill,
            RowHeadersVisible = false,
            AllowUserToAddRows = false,
            BorderStyle = BorderStyle.None
        };
        grid.Columns.Add("name", "Имя");
        grid.Columns.Add("kind", "Тип");
        grid.Columns.Add("last", "Последняя активность");
        grid.Columns.Add("state", "Состояние");
        foreach (var d in data.Items)
        {
            var i = grid.Rows.Add(d.Name, d.Kind == "android" ? "Android" : "Windows", d.LastSeenAt?.LocalDateTime.ToString("g") ?? "—", d.RevokedAt is null ? "Активно" : "Отключено");
            grid.Rows[i].Tag = d;
        }

        var revoke = DialogButton("Отключить выбранное устройство");
        revoke.Dock = DockStyle.Bottom;
        revoke.Height = 46;
        revoke.Click += async (_, _) =>
        {
            if (grid.SelectedRows.Count == 0 || grid.SelectedRows[0].Tag is not DeviceRecord d || d.Id == data.CurrentDeviceId || d.RevokedAt is not null) return;
            if (MessageBox.Show(this, $"Отключить {d.Name}?", "TubeControl", MessageBoxButtons.YesNo) == DialogResult.Yes)
            {
                await _api.RevokeDeviceAsync(_config.DeviceToken, d.Id);
                f.Close();
                await Devices();
            }
        };
        f.Controls.Add(grid);
        f.Controls.Add(revoke);
        ApplyThemeTo(f);
        f.ShowDialog(this);
    }

    private void ShowSettings()
    {
        using var f = CreateDialog("Настройки", 450, 280);
        var title = new Label { Text = "Оформление", AutoSize = true, Font = new Font("Segoe UI", 13, FontStyle.Bold) };
        var dark = DialogButton("Тёмная тема");
        var light = DialogButton("Светлая тема");
        dark.Click += (_, _) => { SetTheme("dark"); f.Close(); };
        light.Click += (_, _) => { SetTheme("light"); f.Close(); };
        var p = DialogStack();
        p.Controls.Add(title);
        p.Controls.Add(dark);
        p.Controls.Add(light);
        f.Controls.Add(p);
        ApplyThemeTo(f);
        f.ShowDialog(this);
    }

    private string? Prompt(string title, string text)
    {
        using var f = CreateDialog(title, 440, 220);
        var l = new Label { Text = text, AutoSize = true, Font = new Font("Segoe UI", 10, FontStyle.Bold) };
        var tb = DialogTextBox("");
        var ok = DialogButton("OK");
        ok.Click += (_, _) => f.DialogResult = DialogResult.OK;
        var p = DialogStack();
        p.Controls.Add(l);
        p.Controls.Add(tb);
        p.Controls.Add(ok);
        f.Controls.Add(p);
        ApplyThemeTo(f);
        return f.ShowDialog(this) == DialogResult.OK ? tb.Text : null;
    }

    private Form CreateDialog(string title, int width, int height)
    {
        var f = new Form
        {
            Text = title,
            Width = width,
            Height = height,
            StartPosition = FormStartPosition.CenterParent,
            FormBorderStyle = FormBorderStyle.FixedDialog,
            MaximizeBox = false,
            MinimizeBox = false,
            Icon = Icon
        };
        return f;
    }

    private static FlowLayoutPanel DialogStack() => new()
    {
        Dock = DockStyle.Fill,
        FlowDirection = FlowDirection.TopDown,
        Padding = new Padding(28),
        WrapContents = false,
        AutoScroll = true
    };

    private TextBox DialogTextBox(string placeholder) => new()
    {
        Width = 320,
        Height = 35,
        PlaceholderText = placeholder,
        Font = new Font("Segoe UI", 10),
        BorderStyle = BorderStyle.FixedSingle
    };

    private GlowButton DialogButton(string text) => new()
    {
        Text = text,
        Width = 320,
        Height = 42,
        FillColor = UiPalette.Emerald,
        HoverFillColor = UiPalette.EmeraldBright,
        PressedFillColor = Color.FromArgb(9, 130, 91),
        ForeColor = Color.Black,
        BorderColor = UiPalette.Mint,
        Font = new Font("Segoe UI", 9, FontStyle.Bold)
    };

    private Label LabelSmall(string text) => new() { Text = text, AutoSize = true, ForeColor = Muted, Font = new Font("Segoe UI", 8.6f) };

    private void SetTheme(string theme)
    {
        if (_config.Theme == theme) return;
        _config.Theme = theme;
        _store.Save(_config);
        ApplyTheme();
        Render();
    }

    private void ApplyTheme()
    {
        BackColor = Bg;
        ForeColor = Fg;
        ApplyThemeTo(this);

        _themeLight.FillColor = DarkTheme ? Color.Transparent : Color.FromArgb(17, 93, 69);
        _themeLight.BorderColor = DarkTheme ? Border : UiPalette.EmeraldBright;
        _themeLight.ForeColor = DarkTheme ? Muted : Color.White;

        _themeDark.FillColor = DarkTheme ? Color.FromArgb(17, 93, 69) : Color.Transparent;
        _themeDark.BorderColor = DarkTheme ? UiPalette.EmeraldBright : Border;
        _themeDark.ForeColor = DarkTheme ? Color.White : Muted;

        _details.BackColor = Panel;
        _details.BorderColor = Border;
        _detailThu.ForeColor = Fg;
        _tubeArt.DarkTheme = DarkTheme;
        _saveStatus.ForeColor = Color.FromArgb(0, 28, 18);
        _commentCount.ForeColor = Muted;
        _userName.ForeColor = Fg;
        _license.ForeColor = Muted;
        _state.ForeColor = Muted;
        _recordCount.ForeColor = Muted;

        _grid.BackgroundColor = Panel;
        _grid.DefaultCellStyle.BackColor = Panel;
        _grid.DefaultCellStyle.ForeColor = Fg;
        _grid.DefaultCellStyle.SelectionBackColor = DarkTheme ? Color.FromArgb(7, 69, 50) : Color.FromArgb(209, 242, 229);
        _grid.DefaultCellStyle.SelectionForeColor = Fg;
        _grid.DefaultCellStyle.Font = new Font("Segoe UI", 8.8f);
        _grid.DefaultCellStyle.Padding = new Padding(6, 0, 4, 0);
        _grid.AlternatingRowsDefaultCellStyle.BackColor = DarkTheme ? Color.FromArgb(7, 25, 20) : Color.FromArgb(247, 251, 249);
        _grid.ColumnHeadersDefaultCellStyle.BackColor = DarkTheme ? Color.FromArgb(6, 19, 15) : Color.FromArgb(236, 245, 241);
        _grid.ColumnHeadersDefaultCellStyle.ForeColor = Fg;
        _grid.ColumnHeadersDefaultCellStyle.Font = new Font("Segoe UI", 8.7f, FontStyle.Bold);
        _grid.ColumnHeadersDefaultCellStyle.Padding = new Padding(7, 0, 3, 0);
        _grid.GridColor = Border;

        _search.BackColor = Panel;
        _search.ForeColor = Fg;
        _filter.BackColor = Panel;
        _filter.ForeColor = Fg;
        _status.BackColor = Panel2;
        _status.ForeColor = Fg;
        _comment.BackColor = Panel2;
        _comment.ForeColor = Fg;

        foreach (Control c in Controls) c.Invalidate(true);
    }

    private void ApplyThemeTo(Control root)
    {
        if (root == _chrome) return;

        if (root is RoundedPanel rp)
        {
            if (rp != _details && rp.BackColor == UiPalette.DarkPanel) rp.BackColor = Panel;
            rp.BorderColor = Border;
        }
        else if (root is GlowButton gb)
        {
            if (gb.Tag is string navTag && navTag.StartsWith("nav"))
            {
                var active = navTag == "nav-active";
                gb.FillColor = active
                    ? (DarkTheme ? Color.FromArgb(9, 55, 42) : Color.FromArgb(210, 244, 231))
                    : Color.Transparent;
                gb.HoverFillColor = DarkTheme ? Color.FromArgb(9, 47, 37) : Color.FromArgb(225, 244, 236);
                gb.BorderColor = active ? UiPalette.EmeraldBright : Color.Transparent;
                gb.ForeColor = active ? (DarkTheme ? Color.White : Color.FromArgb(10, 72, 49)) : Fg;
                gb.BorderWidth = active ? 1 : 0;
            }
            else if (gb != _saveStatus && gb != _themeDark && gb != _themeLight)
            {
                gb.FillColor = Panel;
                gb.HoverFillColor = Panel2;
                gb.PressedFillColor = DarkTheme ? Color.FromArgb(11, 67, 50) : Color.FromArgb(210, 238, 226);
                gb.BorderColor = UiPalette.Emerald;
                gb.ForeColor = Fg;
            }
        }
        else if (root is TextBox tb)
        {
            tb.BackColor = Panel2;
            tb.ForeColor = Fg;
        }
        else if (root is ComboBox cb)
        {
            cb.BackColor = Panel2;
            cb.ForeColor = Fg;
        }
        else if (root is Label l)
        {
            if (l != _currentStatus && l.ForeColor != UiPalette.EmeraldBright && l.ForeColor != UiPalette.Mint)
                l.ForeColor = l.Font.Bold ? Fg : Muted;
        }
        else if (root is Panel or FlowLayoutPanel or TableLayoutPanel)
        {
            if (root.BackColor != Color.Transparent && root != _chrome)
                root.BackColor = Bg;
        }

        foreach (Control c in root.Controls) ApplyThemeTo(c);
    }

    private void StatusDrawItem(object? sender, DrawItemEventArgs e)
    {
        if (e.Index < 0) return;
        e.DrawBackground();
        var text = _status.Items[e.Index]?.ToString() ?? "";
        var color = text == "Автоматически" ? Muted : UiDrawing.StatusColor(text);
        using var dot = new SolidBrush(color);
        e.Graphics.SmoothingMode = SmoothingMode.AntiAlias;
        e.Graphics.FillEllipse(dot, e.Bounds.X + 9, e.Bounds.Y + e.Bounds.Height / 2 - 4, 8, 8);
        TextRenderer.DrawText(e.Graphics, text, _status.Font,
            new Rectangle(e.Bounds.X + 25, e.Bounds.Y, e.Bounds.Width - 28, e.Bounds.Height),
            Fg, TextFormatFlags.Left | TextFormatFlags.VerticalCenter | TextFormatFlags.EndEllipsis);
        e.DrawFocusRectangle();
    }

    private void ToggleMaximize()
    {
        if (_maximized)
        {
            _maximized = false;
            Bounds = _restoreBounds.Width > 0 ? _restoreBounds : new Rectangle(100, 80, 1380, 820);
            if (_maxButton is not null) _maxButton.Text = "□";
        }
        else
        {
            _restoreBounds = Bounds;
            MaximizeToWorkArea();
        }
    }

    private void MaximizeToWorkArea()
    {
        var wa = Screen.FromControl(this).WorkingArea;
        Bounds = wa;
        _maximized = true;
        if (_maxButton is not null) _maxButton.Text = "❐";
    }

    private static string FilterCode(string ru) => ru switch
    {
        "Отправлен" => "sent",
        "В пути" => "transit",
        "Возвращён" => "returned",
        "Потерян" => "lost",
        "Просрочен" => "overdue",
        _ => "all"
    };

    private static string StatusCode(string ru) => ru switch
    {
        "Отправлен" => "sent",
        "В пути" => "transit",
        "Возвращён" => "returned",
        "Потерян" => "lost",
        _ => "auto"
    };

    private static string StatusRuFromCode(string code) => code switch
    {
        "sent" => "Отправлен",
        "transit" => "В пути",
        "returned" => "Возвращён",
        "lost" => "Потерян",
        _ => "Автоматически"
    };

    [DllImport("user32.dll")]
    private static extern bool ReleaseCapture();

    [DllImport("user32.dll")]
    private static extern IntPtr SendMessage(IntPtr hWnd, int msg, int wParam, int lParam);
}
