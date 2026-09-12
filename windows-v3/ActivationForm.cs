namespace TubeControl.Windows;

public sealed class ActivationForm : Form
{
    private readonly ConfigStore _store;
    private readonly AppConfig _config;
    private readonly ApiClient _api;
    private readonly TextBox _key = new() { Width = 280, Font = new Font("Segoe UI", 13), PlaceholderText = "TC-XXXX-XXXX" };
    private readonly TextBox _name = new() { Width = 280, Font = new Font("Segoe UI", 11), Text = Environment.MachineName };
    private readonly Label _message = new() { AutoSize = true, ForeColor = Color.FromArgb(160, 180, 174) };
    private readonly Button _activate = new() { Text = "Активировать", Width = 160, Height = 44, FlatStyle = FlatStyle.Flat };

    public ActivationForm(ConfigStore store, AppConfig config, ApiClient api)
    {
        _store = store; _config = config; _api = api;
        Text = "TubeControl — активация"; Width = 620; Height = 430; StartPosition = FormStartPosition.CenterScreen;
        BackColor = Color.FromArgb(5, 9, 8); ForeColor = Color.White; FormBorderStyle = FormBorderStyle.FixedDialog; MaximizeBox = false;
        _activate.BackColor = Color.FromArgb(16, 185, 129); _activate.ForeColor = Color.Black; _activate.FlatAppearance.BorderSize = 0;
        _activate.Click += async (_, _) => await Activate();
        _key.CharacterCasing = CharacterCasing.Upper;
        var title = new Label { Text = "TubeControl", Font = new Font("Segoe UI", 30, FontStyle.Bold), AutoSize = true, ForeColor = Color.FromArgb(108,255,203) };
        var lead = new Label { Text = "Активация Windows", Font = new Font("Segoe UI", 18, FontStyle.Bold), AutoSize = true };
        var info = new Label { Text = "Получите одноразовый ключ на сайте TubeControl и введите его здесь.\nБраузер для активации не используется.", AutoSize = true, ForeColor = Color.FromArgb(160,180,174) };
        var nameLbl = new Label { Text = "Имя компьютера", AutoSize = true, ForeColor = Color.FromArgb(160,180,174) };
        var keyLbl = new Label { Text = "Одноразовый ключ", AutoSize = true, ForeColor = Color.FromArgb(160,180,174) };
        var panel = new FlowLayoutPanel { Dock = DockStyle.Fill, FlowDirection = FlowDirection.TopDown, Padding = new Padding(34), WrapContents = false, AutoScroll = true };
        panel.Controls.Add(title); panel.Controls.Add(new Label { Height = 8 }); panel.Controls.Add(lead); panel.Controls.Add(info);
        panel.Controls.Add(new Label { Height = 12 }); panel.Controls.Add(nameLbl); panel.Controls.Add(_name); panel.Controls.Add(keyLbl); panel.Controls.Add(_key); panel.Controls.Add(new Label { Height = 6 }); panel.Controls.Add(_activate); panel.Controls.Add(_message);
        Controls.Add(panel);
    }

    private async Task Activate()
    {
        var key = _key.Text.Trim().ToUpperInvariant();
        if (key.Length < 8) { _message.ForeColor = Color.OrangeRed; _message.Text = "Проверьте ключ активации"; return; }
        _activate.Enabled = false; _message.ForeColor = Color.FromArgb(160,180,174); _message.Text = "Проверяем ключ…";
        try
        {
            var r = await _api.ActivateAsync(key, string.IsNullOrWhiteSpace(_name.Text) ? Environment.MachineName : _name.Text.Trim());
            _config.DeviceToken = r.DeviceToken; _config.DeviceId = r.Device.Id; _config.DeviceName = r.Device.Name; _store.Save(_config);
            DialogResult = DialogResult.OK; Close();
        }
        catch (Exception ex) { _message.ForeColor = Color.OrangeRed; _message.Text = "Ключ не принят: " + ex.Message; }
        finally { _activate.Enabled = true; }
    }
}
