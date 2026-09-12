using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace TubeControl.Windows;

public sealed class ConfigStore
{
    private readonly string _path;
    public ConfigStore()
    {
        var dir = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "TubeControl");
        Directory.CreateDirectory(dir);
        _path = Path.Combine(dir, "config-v3.json");
    }

    public AppConfig Load()
    {
        try
        {
            if (!File.Exists(_path)) return new AppConfig();
            var dto = JsonSerializer.Deserialize<ConfigDto>(File.ReadAllText(_path)) ?? new ConfigDto();
            var token = "";
            if (!string.IsNullOrWhiteSpace(dto.ProtectedToken))
            {
                var protectedBytes = Convert.FromBase64String(dto.ProtectedToken);
                var plain = ProtectedData.Unprotect(protectedBytes, null, DataProtectionScope.CurrentUser);
                token = Encoding.UTF8.GetString(plain);
            }
            return new AppConfig
            {
                Server = string.IsNullOrWhiteSpace(dto.Server) ? new AppConfig().Server : dto.Server,
                DeviceToken = token,
                DeviceId = dto.DeviceId ?? "",
                DeviceName = string.IsNullOrWhiteSpace(dto.DeviceName) ? Environment.MachineName : dto.DeviceName,
                Theme = dto.Theme == "light" ? "light" : "dark"
            };
        }
        catch { return new AppConfig(); }
    }

    public void Save(AppConfig config)
    {
        string protectedToken = "";
        if (!string.IsNullOrWhiteSpace(config.DeviceToken))
        {
            var raw = Encoding.UTF8.GetBytes(config.DeviceToken);
            protectedToken = Convert.ToBase64String(ProtectedData.Protect(raw, null, DataProtectionScope.CurrentUser));
        }
        var dto = new ConfigDto
        {
            Server = config.Server,
            ProtectedToken = protectedToken,
            DeviceId = config.DeviceId,
            DeviceName = config.DeviceName,
            Theme = config.Theme
        };
        File.WriteAllText(_path, JsonSerializer.Serialize(dto, new JsonSerializerOptions { WriteIndented = true }));
    }

    private sealed class ConfigDto
    {
        public string? Server { get; set; }
        public string? ProtectedToken { get; set; }
        public string? DeviceId { get; set; }
        public string? DeviceName { get; set; }
        public string? Theme { get; set; }
    }
}
