using QRCoder;
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
    private readonly Label _detailThu = new() { AutoSize = true, Font = new Font("Segoe UI", 18, FontStyle.Bold) };
    private readonly Label _detailInfo = new() { AutoSize = true };
    private readonly ComboBox _status = new() { DropDownStyle = ComboBoxStyle.DropDownList, Width = 260 };
    private readonly TextBox _comment = new() { Multiline = true, Width = 260, Height = 90, PlaceholderText = "Комментарий…", MaxLength = 500 };
    private readonly Button _saveStatus = new() { Text = "Сохранить изменения", Height = 42, Width = 260, FlatStyle = FlatStyle.Flat };
    private readonly Panel _details = new() { Width = 310, Padding = new Padding(18) };
    private readonly Button _theme = new() { Width = 130, Height = 38, FlatStyle = FlatStyle.Flat };

    private readonly Color Emerald = Color.FromArgb(16, 185, 129);
    private readonly Color Mint = Color.FromArgb(108, 255, 203);
    private readonly Color Dark = Color.FromArgb(5, 9, 8);
    private readonly Color DarkPanel = Color.FromArgb(10, 22, 18);
    private readonly Color Light = Color.FromArgb(244, 250, 247);

    public MainForm(ConfigStore store, AppConfig config, ApiClient api)
    {
        _store = store; _config = config; _api = api;
        Text = "TubeControl"; WindowState = FormWindowState.Maximized; MinimumSize = new Size(1100, 720);
        Icon = Icon.ExtractAssociatedIcon(Application.ExecutablePath);
        BuildUi(); ApplyTheme(); Shown += async (_, _) => await Reload();
    }

    private void BuildUi()
    {
        var root = new TableLayoutPanel { Dock = DockStyle.Fill, ColumnCount = 2, RowCount = 2 };
        root.ColumnStyles.Add(new ColumnStyle(SizeType.Percent, 100)); root.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute, 330));
        root.RowStyles.Add(new RowStyle(SizeType.AutoSize)); root.RowStyles.Add(new RowStyle(SizeType.Percent, 100));

        var top = new FlowLayoutPanel { Dock = DockStyle.Fill, AutoSize = true, Padding = new Padding(14), WrapContents = true };
        var logo = new Label { Text = "TubeControl", Font = new Font("Segoe UI", 24, FontStyle.Bold), AutoSize = true, Margin = new Padding(4, 6, 30, 0) };
        top.Controls.Add(logo);
        top.Controls.Add(Action("Импорт Excel", async () => await ImportExcel()));
        top.Controls.Add(Action("Синхронизировать", Reload));
        top.Controls.Add(Action("Выгрузить", Export));
        top.Controls.Add(Action("Добавить THU вручную", async () => await AddThu()));
        top.Controls.Add(Action("Зарегистрировать возврат", async () => await ManualReturn()));
        top.Controls.Add(Action("Подключить Android", async () => await PairAndroid()));
        top.Controls.Add(Action("Устройства", async () => await Devices()));
        _theme.Click += (_, _) => ToggleTheme(); top.Controls.Add(_theme);
        _state.Margin = new Padding(18, 12, 0, 0); top.Controls.Add(_state);
        root.Controls.Add(top, 0, 0); root.SetColumnSpan(top, 2);

        var center = new TableLayoutPanel { Dock = DockStyle.Fill, RowCount = 2, Padding = new Padding(14), ColumnCount = 1 };
        center.RowStyles.Add(new RowStyle(SizeType.AutoSize)); center.RowStyles.Add(new RowStyle(SizeType.Percent, 100));
        var tools = new FlowLayoutPanel { Dock = DockStyle.Fill, AutoSize = true, Padding = new Padding(0, 0, 0, 10) };
        _search.Width = 430; _search.Height = 38; _search.TextChanged += (_, _) => Render(); tools.Controls.Add(_search);
        _filter.Width = 180; _filter.Items.AddRange(new object[] { "Все статусы", "Отправлен", "В пути", "Возвращён", "Потерян", "Просрочен" }); _filter.SelectedIndex = 0; _filter.SelectedIndexChanged += (_, _) => Render(); tools.Controls.Add(_filter);
        center.Controls.Add(tools, 0, 0);

        _grid.Dock = DockStyle.Fill; _grid.ReadOnly = true; _grid.AllowUserToAddRows = false; _grid.AllowUserToDeleteRows = false; _grid.SelectionMode = DataGridViewSelectionMode.FullRowSelect; _grid.MultiSelect = false; _grid.AutoSizeColumnsMode = DataGridViewAutoSizeColumnsMode.Fill; _grid.RowHeadersVisible = false; _grid.BorderStyle = BorderStyle.None; _grid.SelectionChanged += (_, _) => ShowSelected();
        _grid.Columns.Add("THU", "THU"); _grid.Columns.Add("Order", "№ заказа"); _grid.Columns.Add("Store", "Магазин"); _grid.Columns.Add("Date", "Дата отправки"); _grid.Columns.Add("Status", "Статус");
        center.Controls.Add(_grid, 0, 1);
        root.Controls.Add(center, 0, 1);

        var detailStack = new FlowLayoutPanel { Dock = DockStyle.Fill, FlowDirection = FlowDirection.TopDown, WrapContents = false, AutoScroll = true };
        detailStack.Controls.Add(new Label { Text = "Выбранный тубус", AutoSize = true, Font = new Font("Segoe UI", 11, FontStyle.Bold) });
        detailStack.Controls.Add(_detailThu); detailStack.Controls.Add(_detailInfo);
        detailStack.Controls.Add(new Label { Text = "Изменить статус", AutoSize = true, Margin = new Padding(0, 22, 0, 4) });
        _status.Items.AddRange(new object[] { "Автоматически", "Отправлен", "В пути", "Возвращён", "Потерян" }); _status.SelectedIndex = 0; detailStack.Controls.Add(_status);
        detailStack.Controls.Add(new Label { Text = "Комментарий", AutoSize = true, Margin = new Padding(0, 12, 0, 4) }); detailStack.Controls.Add(_comment);
        _saveStatus.BackColor = Emerald; _saveStatus.ForeColor = Color.Black; _saveStatus.FlatAppearance.BorderSize = 0; _saveStatus.Margin = new Padding(0, 14, 0, 0); _saveStatus.Click += async (_, _) => await SaveStatus(); detailStack.Controls.Add(_saveStatus);
        _details.Controls.Add(detailStack); _details.Dock = DockStyle.Fill; root.Controls.Add(_details, 1, 1);
        Controls.Add(root);
    }

    private Button Action(string text, Func<Task> fn)
    {
        var b = new Button { Text = text, AutoSize = true, Height = 38, FlatStyle = FlatStyle.Flat, Margin = new Padding(4) };
        b.Click += async (_, _) => { try { b.Enabled = false; await fn(); } catch (Exception ex) { MessageBox.Show(ex.Message, "TubeControl", MessageBoxButtons.OK, MessageBoxIcon.Error); } finally { b.Enabled = true; } };
        return b;
    }
    private Button Action(string text, Action fn) => Action(text, () => { fn(); return Task.CompletedTask; });

    private async Task Reload()
    {
        _state.Text = "Синхронизация…";
        var r = await _api.GetTubesAsync(_config.DeviceToken);
        _all.Clear(); _all.AddRange(r.Items); Render(); _state.Text = $"Записей: {_all.Count}";
    }

    private void Render()
    {
        if (_grid.Columns.Count == 0) return;
        var q = _search.Text.Trim().ToLowerInvariant(); var sf = FilterCode(_filter.SelectedItem?.ToString() ?? "Все статусы");
        _visible.Clear(); _visible.AddRange(_all.Where(t => (q.Length == 0 || $"{t.Thu} {t.StoreCode} {t.OrderNumber}".ToLowerInvariant().Contains(q)) && (sf == "all" || t.Status == sf)));
        _grid.Rows.Clear();
        foreach (var t in _visible)
        {
            var idx = _grid.Rows.Add(t.Thu, t.OrderNumber, t.StoreCode, t.ShippedAt, ExcelService.StatusRu(t.Status));
            var row = _grid.Rows[idx]; row.Tag = t;
            row.Cells[4].Style.ForeColor = t.Status switch { "returned" => Emerald, "lost" or "overdue" => Color.FromArgb(255, 92, 104), "sent" => Color.FromArgb(80, 170, 255), _ => Color.Goldenrod };
        }
        ShowSelected();
    }

    private void ShowSelected()
    {
        if (_grid.SelectedRows.Count == 0 || _grid.SelectedRows[0].Tag is not TubeItem t) { _detailThu.Text = "—"; _detailInfo.Text = "Выберите строку"; _saveStatus.Enabled = false; return; }
        _saveStatus.Enabled = true; _detailThu.Text = t.Thu;
        _detailInfo.Text = $"\n№ заказа: {t.OrderNumber}\nМагазин: {t.StoreCode}\nДата отправки: {t.ShippedAt}\nТекущий статус: {ExcelService.StatusRu(t.Status)}";
        _status.SelectedItem = StatusRuFromCode(t.ManualStatus ?? t.Status); if (_status.SelectedIndex < 0) _status.SelectedIndex = 0;
        _comment.Text = t.StatusComment ?? "";
    }

    private async Task SaveStatus()
    {
        if (_grid.SelectedRows.Count == 0 || _grid.SelectedRows[0].Tag is not TubeItem t) return;
        var code = StatusCode(_status.SelectedItem?.ToString() ?? "Автоматически");
        await _api.UpdateStatusAsync(_config.DeviceToken, t.Thu, code, _comment.Text.Trim()); await Reload();
    }

    private async Task ImportExcel()
    {
        using var dlg = new OpenFileDialog { Filter = "Excel (*.xlsx;*.xlsm)|*.xlsx;*.xlsm", Title = "Выберите файл отгрузки" };
        if (dlg.ShowDialog() != DialogResult.OK) return;
        _state.Text = "Импорт Excel…"; var items = ExcelService.ReadImport(dlg.FileName); await _api.ImportAsync(_config.DeviceToken, items); await Reload(); MessageBox.Show($"Импортировано: {items.Count}", "TubeControl");
    }

    private void Export()
    {
        var rows = _visible.Where(t => t.Status != "returned").ToList();
        using var dlg = new SaveFileDialog { Filter = "Excel (*.xlsx)|*.xlsx", FileName = $"TubeControl_не_возвращены_{DateTime.Now:yyyyMMdd_HHmm}.xlsx" };
        if (dlg.ShowDialog() != DialogResult.OK) return; ExcelService.Export(dlg.FileName, rows); MessageBox.Show($"Выгружено: {rows.Count}", "TubeControl");
    }

    private async Task AddThu()
    {
        using var f = new Form { Text = "Добавить THU", Width = 390, Height = 330, StartPosition = FormStartPosition.CenterParent, FormBorderStyle = FormBorderStyle.FixedDialog, MaximizeBox = false };
        var thu = new TextBox { Width = 300, PlaceholderText = "THU" }; var store = new TextBox { Width = 300, PlaceholderText = "Код магазина" }; var order = new TextBox { Width = 300, PlaceholderText = "№ заказа" }; var date = new DateTimePicker { Width = 300, Format = DateTimePickerFormat.Short }; var ok = new Button { Text = "Добавить", Width = 150, Height = 38 };
        var p = new FlowLayoutPanel { Dock = DockStyle.Fill, FlowDirection = FlowDirection.TopDown, Padding = new Padding(25), WrapContents = false }; p.Controls.AddRange(new Control[] { thu, store, order, date, ok }); f.Controls.Add(p); ok.Click += (_, _) => f.DialogResult = DialogResult.OK; ApplyThemeTo(f);
        if (f.ShowDialog(this) != DialogResult.OK || string.IsNullOrWhiteSpace(thu.Text)) return;
        await _api.ImportAsync(_config.DeviceToken, new[] { new ImportItem { Thu = thu.Text.Trim().ToUpperInvariant(), StoreCode = store.Text.Trim(), OrderNumber = order.Text.Trim(), ShippedAt = date.Value.ToString("yyyy-MM-dd") } }); await Reload();
    }

    private async Task ManualReturn()
    {
        var value = Prompt("Зарегистрировать возврат", "Введите THU:"); if (string.IsNullOrWhiteSpace(value)) return;
        await _api.UpdateStatusAsync(_config.DeviceToken, value.Trim().ToUpperInvariant(), "returned", "Возврат зарегистрирован вручную в Windows"); await Reload();
    }

    private async Task PairAndroid()
    {
        var e = await _api.CreateEnrollmentAsync(_config.DeviceToken);
        var payload = JsonSerializer.Serialize(new { type = "tubecontrol-enroll-v2", server = _config.Server.TrimEnd('/'), code = e.Code });
        using var gen = new QRCodeGenerator(); using var data = gen.CreateQrCode(payload, QRCodeGenerator.ECCLevel.Q); var png = new PngByteQRCode(data).GetGraphic(8); using var ms = new MemoryStream(png); using var img = Image.FromStream(ms); using var copy = new Bitmap(img);
        using var f = new Form { Text = "Подключить Android", Width = 470, Height = 580, StartPosition = FormStartPosition.CenterParent, FormBorderStyle = FormBorderStyle.FixedDialog, MaximizeBox = false };
        var label = new Label { Text = "Отсканируйте этот QR в Android TubeControl.\nQR одноразовый и действует 10 минут.", AutoSize = true, TextAlign = ContentAlignment.MiddleCenter };
        var pic = new PictureBox { Image = new Bitmap(copy), Width = 380, Height = 380, SizeMode = PictureBoxSizeMode.Zoom };
        var p = new FlowLayoutPanel { Dock = DockStyle.Fill, FlowDirection = FlowDirection.TopDown, Padding = new Padding(30), WrapContents = false }; p.Controls.Add(label); p.Controls.Add(pic); f.Controls.Add(p); ApplyThemeTo(f); f.ShowDialog(this);
    }

    private async Task Devices()
    {
        var data = await _api.GetDevicesAsync(_config.DeviceToken);
        using var f = new Form { Text = "Устройства", Width = 760, Height = 460, StartPosition = FormStartPosition.CenterParent };
        var grid = new DataGridView { Dock = DockStyle.Fill, ReadOnly = true, SelectionMode = DataGridViewSelectionMode.FullRowSelect, AutoSizeColumnsMode = DataGridViewAutoSizeColumnsMode.Fill, RowHeadersVisible = false, AllowUserToAddRows = false };
        grid.Columns.Add("name", "Имя"); grid.Columns.Add("kind", "Тип"); grid.Columns.Add("last", "Последняя активность"); grid.Columns.Add("state", "Состояние");
        foreach (var d in data.Items) { var i = grid.Rows.Add(d.Name, d.Kind == "android" ? "Android" : "Windows", d.LastSeenAt?.LocalDateTime.ToString("g") ?? "—", d.RevokedAt is null ? "Активно" : "Отключено"); grid.Rows[i].Tag = d; }
        var revoke = new Button { Dock = DockStyle.Bottom, Height = 44, Text = "Отключить выбранное устройство" }; revoke.Click += async (_, _) => { if (grid.SelectedRows.Count == 0 || grid.SelectedRows[0].Tag is not DeviceRecord d || d.Id == data.CurrentDeviceId || d.RevokedAt is not null) return; if (MessageBox.Show($"Отключить {d.Name}?", "TubeControl", MessageBoxButtons.YesNo) == DialogResult.Yes) { await _api.RevokeDeviceAsync(_config.DeviceToken, d.Id); f.Close(); await Devices(); } };
        f.Controls.Add(grid); f.Controls.Add(revoke); ApplyThemeTo(f); f.ShowDialog(this);
    }

    private string? Prompt(string title, string text)
    {
        using var f = new Form { Text = title, Width = 430, Height = 190, StartPosition = FormStartPosition.CenterParent, FormBorderStyle = FormBorderStyle.FixedDialog, MaximizeBox = false };
        var l = new Label { Text = text, AutoSize = true }; var tb = new TextBox { Width = 340 }; var ok = new Button { Text = "OK", Width = 120 }; ok.Click += (_, _) => f.DialogResult = DialogResult.OK;
        var p = new FlowLayoutPanel { Dock = DockStyle.Fill, FlowDirection = FlowDirection.TopDown, Padding = new Padding(24), WrapContents = false }; p.Controls.Add(l); p.Controls.Add(tb); p.Controls.Add(ok); f.Controls.Add(p); ApplyThemeTo(f); return f.ShowDialog(this) == DialogResult.OK ? tb.Text : null;
    }

    private void ToggleTheme() { _config.Theme = _config.Theme == "dark" ? "light" : "dark"; _store.Save(_config); ApplyTheme(); Render(); }
    private void ApplyTheme() { ApplyThemeTo(this); _theme.Text = _config.Theme == "dark" ? "☀ Светлая" : "☾ Тёмная"; }
    private void ApplyThemeTo(Control root)
    {
        var dark = _config.Theme == "dark"; var bg = dark ? Dark : Light; var panel = dark ? DarkPanel : Color.White; var fg = dark ? Color.White : Color.FromArgb(20, 35, 30); var muted = dark ? Color.FromArgb(160, 180, 174) : Color.FromArgb(70, 90, 82);
        root.BackColor = bg; root.ForeColor = fg;
        foreach (Control c in root.Controls)
        {
            if (c is Button b) { if (b != _saveStatus) { b.BackColor = panel; b.ForeColor = fg; b.FlatAppearance.BorderColor = Emerald; } }
            else if (c is TextBox or ComboBox) { c.BackColor = panel; c.ForeColor = fg; }
            else if (c is Panel or FlowLayoutPanel or TableLayoutPanel) c.BackColor = bg;
            else if (c is Label l && l != _detailThu) l.ForeColor = l.ForeColor == Color.OrangeRed ? l.ForeColor : muted;
            ApplyThemeTo(c);
        }
        _grid.BackgroundColor = bg; _grid.DefaultCellStyle.BackColor = panel; _grid.DefaultCellStyle.ForeColor = fg; _grid.DefaultCellStyle.SelectionBackColor = Color.FromArgb(20, 100, 78); _grid.DefaultCellStyle.SelectionForeColor = Color.White; _grid.ColumnHeadersDefaultCellStyle.BackColor = panel; _grid.ColumnHeadersDefaultCellStyle.ForeColor = Mint; _grid.EnableHeadersVisualStyles = false; _grid.GridColor = dark ? Color.FromArgb(28, 55, 47) : Color.FromArgb(210,225,218); _details.BackColor = panel; _detailThu.ForeColor = Emerald; _saveStatus.BackColor = Emerald; _saveStatus.ForeColor = Color.Black;
    }

    private static string FilterCode(string ru) => ru switch { "Отправлен" => "sent", "В пути" => "transit", "Возвращён" => "returned", "Потерян" => "lost", "Просрочен" => "overdue", _ => "all" };
    private static string StatusCode(string ru) => ru switch { "Отправлен" => "sent", "В пути" => "transit", "Возвращён" => "returned", "Потерян" => "lost", _ => "auto" };
    private static string StatusRuFromCode(string code) => code switch { "sent" => "Отправлен", "transit" => "В пути", "returned" => "Возвращён", "lost" => "Потерян", _ => "Автоматически" };
}
