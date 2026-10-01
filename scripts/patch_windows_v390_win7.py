from pathlib import Path
import re

root = Path("windows-v3")
project_path = root / "TubeControl.Windows.csproj"
program_path = root / "Program.cs"
startup_path = root / "StartupDiagnostics.cs"
models_path = root / "Models.cs"
api_path = root / "ApiClient.cs"
config_path = root / "ConfigStore.cs"
excel_path = root / "ExcelService.cs"
main_path = root / "MainForm.cs"
activation_path = root / "ActivationForm.cs"
ui_path = root / "UiKit.cs"

# v3.9.0 — Windows 7 SP1 compatibility build.
# .NET 8 WinForms does not run on Windows 7, so this build targets .NET Framework 4.8.
# It also removes Segoe UI from the startup/font dependency chain.

project_path.write_text(r'''<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>WinExe</OutputType>
    <TargetFramework>net48</TargetFramework>
    <UseWindowsForms>true</UseWindowsForms>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <LangVersion>latest</LangVersion>
    <ApplicationIcon>tubecontrol.ico</ApplicationIcon>
    <AssemblyName>TubeControl-Windows-v3</AssemblyName>
    <RootNamespace>TubeControl.Windows</RootNamespace>
    <Version>3.9.0</Version>
    <PlatformTarget>AnyCPU</PlatformTarget>
    <Prefer32Bit>false</Prefer32Bit>
    <AutoGenerateBindingRedirects>true</AutoGenerateBindingRedirects>
    <GenerateBindingRedirectsOutputType>true</GenerateBindingRedirectsOutputType>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.NETFramework.ReferenceAssemblies.net48" Version="1.0.3" PrivateAssets="all" />
    <PackageReference Include="ClosedXML" Version="0.104.2" />
    <PackageReference Include="QRCoder" Version="1.6.0" />
    <PackageReference Include="Newtonsoft.Json" Version="13.0.3" />
    <PackageReference Include="Fody" Version="6.8.2" PrivateAssets="all" />
    <PackageReference Include="Costura.Fody" Version="5.7.0" PrivateAssets="all" />
  </ItemGroup>
  <ItemGroup>
    <Reference Include="System.Security" />
  </ItemGroup>
</Project>
''', encoding="utf-8", newline="\n")

program_path.write_text(r'''using System.Net;

namespace TubeControl.Windows;

internal static class Program
{
    [STAThread]
    static void Main()
    {
        StartupDiagnostics.Init();

        // Windows 7/.NET Framework may otherwise negotiate an obsolete TLS version.
        ServicePointManager.SecurityProtocol = SecurityProtocolType.Tls12;

        Application.EnableVisualStyles();
        Application.SetCompatibleTextRenderingDefault(false);

        var store = new ConfigStore();
        var config = store.Load();
        var api = new ApiClient(config.Server);

        if (string.IsNullOrWhiteSpace(config.DeviceToken))
        {
            using var activation = new ActivationForm(store, config, api);
            if (activation.ShowDialog() != DialogResult.OK) return;
            config = store.Load();
            api = new ApiClient(config.Server);
        }

        Application.Run(new MainForm(store, config, api));
    }
}
''', encoding="utf-8", newline="\n")

startup_path.write_text(r'''using System.Text;

namespace TubeControl.Windows;

internal static class StartupDiagnostics
{
    private static bool _initialized;

    private static readonly string LogPath = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "TubeControl", "startup-error.log");

    internal static void Init()
    {
        if (_initialized) return;
        _initialized = true;

        try
        {
            Application.SetUnhandledExceptionMode(UnhandledExceptionMode.CatchException);
            Application.ThreadException += (_, e) => Report(e.Exception);
            AppDomain.CurrentDomain.UnhandledException += (_, e) =>
            {
                if (e.ExceptionObject is Exception ex) Report(ex);
            };
        }
        catch { }
    }

    private static void Report(Exception ex)
    {
        try
        {
            var dir = Path.GetDirectoryName(LogPath);
            if (!string.IsNullOrWhiteSpace(dir)) Directory.CreateDirectory(dir);
            var text = new StringBuilder()
                .AppendLine(DateTime.Now.ToString("O"))
                .AppendLine(ex.ToString())
                .AppendLine(new string('-', 80))
                .ToString();
            File.AppendAllText(LogPath, text, Encoding.UTF8);
        }
        catch { }

        try
        {
            MessageBox.Show(
                "TubeControl не смог запуститься.\n\n" + ex.Message +
                "\n\nДиагностический файл:\n" + LogPath,
                "TubeControl — ошибка запуска",
                MessageBoxButtons.OK,
                MessageBoxIcon.Error);
        }
        catch { }
    }
}
''', encoding="utf-8", newline="\n")

models_path.write_text(r'''using Newtonsoft.Json;

namespace TubeControl.Windows;

public sealed class AppConfig
{
    public string Server { get; set; } = "https://tubecontrol-api-msk-misibun.amvera.io";
    public string DeviceToken { get; set; } = "";
    public string DeviceId { get; set; } = "";
    public string DeviceName { get; set; } = Environment.MachineName;
    public string Theme { get; set; } = "dark";
    public int OverdueDays { get; set; } = 14;
}

public sealed class TubeItem
{
    [JsonProperty("thu")] public string Thu { get; set; } = "";
    [JsonProperty("shipped_at")] public string ShippedAt { get; set; } = "";
    [JsonProperty("store_code")] public string StoreCode { get; set; } = "";
    [JsonProperty("order_number")] public string OrderNumber { get; set; } = "";
    [JsonProperty("returned_at")] public DateTimeOffset? ReturnedAt { get; set; }
    [JsonProperty("manual_status")] public string? ManualStatus { get; set; }
    [JsonProperty("status")] public string Status { get; set; } = "transit";
    [JsonProperty("status_comment")] public string? StatusComment { get; set; }
    [JsonProperty("updated_at")] public DateTimeOffset UpdatedAt { get; set; }
}

public sealed class TubeListResponse
{
    [JsonProperty("items")] public List<TubeItem> Items { get; set; } = new();
    [JsonProperty("count")] public int Count { get; set; }
}

public sealed class ImportItem
{
    [JsonProperty("thu")] public string Thu { get; set; } = "";
    [JsonProperty("shipped_at")] public string ShippedAt { get; set; } = "";
    [JsonProperty("store_code")] public string StoreCode { get; set; } = "";
    [JsonProperty("order_number")] public string OrderNumber { get; set; } = "";
}

public sealed class ActivationResponse
{
    [JsonProperty("device_token")] public string DeviceToken { get; set; } = "";
    [JsonProperty("device")] public DeviceIdentity Device { get; set; } = new();
}

public sealed class DeviceIdentity
{
    [JsonProperty("id")] public string Id { get; set; } = "";
    [JsonProperty("name")] public string Name { get; set; } = "";
    [JsonProperty("kind")] public string Kind { get; set; } = "";
}

public sealed class EnrollmentResponse
{
    [JsonProperty("code")] public string Code { get; set; } = "";
    [JsonProperty("expires_at")] public DateTimeOffset ExpiresAt { get; set; }
}

public sealed class DeviceRecord
{
    [JsonProperty("id")] public string Id { get; set; } = "";
    [JsonProperty("name")] public string Name { get; set; } = "";
    [JsonProperty("kind")] public string Kind { get; set; } = "";
    [JsonProperty("created_at")] public DateTimeOffset CreatedAt { get; set; }
    [JsonProperty("last_seen_at")] public DateTimeOffset? LastSeenAt { get; set; }
    [JsonProperty("revoked_at")] public DateTimeOffset? RevokedAt { get; set; }
}

public sealed class DeviceListResponse
{
    [JsonProperty("items")] public List<DeviceRecord> Items { get; set; } = new();
    [JsonProperty("current_device_id")] public string CurrentDeviceId { get; set; } = "";
}
''', encoding="utf-8", newline="\n")

api_path.write_text(r'''using System.Net;
using System.Net.Http;
using System.Net.Http.Headers;
using System.Text;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;

namespace TubeControl.Windows;

public sealed class TubeControlApiException : Exception
{
    public int? StatusCode { get; }
    public bool IsTemporary { get; }

    public TubeControlApiException(string message, int? statusCode = null, bool isTemporary = false, Exception? inner = null)
        : base(message, inner)
    {
        StatusCode = statusCode;
        IsTemporary = isTemporary;
    }
}

public sealed class ApiClient
{
    private readonly HttpClient _http;

    public ApiClient(string baseUrl)
    {
        _http = new HttpClient
        {
            BaseAddress = new Uri(baseUrl.TrimEnd('/') + "/"),
            Timeout = TimeSpan.FromSeconds(20)
        };
    }

    private HttpRequestMessage Req(HttpMethod method, string path, string? token = null, object? body = null)
    {
        var req = new HttpRequestMessage(method, path);
        if (!string.IsNullOrWhiteSpace(token))
            req.Headers.Authorization = new AuthenticationHeaderValue("Bearer", token);

        if (body is not null)
        {
            var json = JsonConvert.SerializeObject(body);
            req.Content = new StringContent(json, Encoding.UTF8, "application/json");
        }

        return req;
    }

    private async Task<T> Send<T>(HttpRequestMessage req)
    {
        HttpResponseMessage resp;
        try
        {
            resp = await _http.SendAsync(req);
        }
        catch (TaskCanceledException ex)
        {
            throw new TubeControlApiException("Сервер TubeControl не отвечает. Проверьте интернет-соединение и повторите попытку.", null, true, ex);
        }
        catch (HttpRequestException ex)
        {
            throw new TubeControlApiException("Не удалось подключиться к серверу TubeControl. Проверьте интернет-соединение и повторите попытку.", null, true, ex);
        }

        using (resp)
        {
            var text = await resp.Content.ReadAsStringAsync();
            if (!resp.IsSuccessStatusCode)
            {
                var code = (int)resp.StatusCode;
                if (resp.StatusCode == HttpStatusCode.BadGateway ||
                    resp.StatusCode == HttpStatusCode.ServiceUnavailable ||
                    resp.StatusCode == HttpStatusCode.GatewayTimeout)
                    throw new TubeControlApiException($"Сервер TubeControl временно недоступен ({code}). Повторите попытку через несколько минут.", code, true);

                if (resp.StatusCode == HttpStatusCode.Unauthorized)
                    throw new TubeControlApiException("Сохранённая активация Windows больше не действительна. Требуется повторная активация.", code);

                if (resp.StatusCode == HttpStatusCode.Forbidden)
                    throw new TubeControlApiException("Доступ для этого устройства запрещён.", code);

                if (resp.StatusCode == HttpStatusCode.NotFound)
                    throw new TubeControlApiException("Запрошенные данные не найдены на сервере.", code);

                var serverMessage = TryReadServerMessage(text);
                throw new TubeControlApiException(serverMessage ?? $"Сервер вернул ошибку {code}.", code, code >= 500);
            }

            try
            {
                var value = JsonConvert.DeserializeObject<T>(text);
                if (value is null) throw new TubeControlApiException("Сервер вернул пустой ответ.");
                return value;
            }
            catch (JsonException ex)
            {
                throw new TubeControlApiException("Сервер вернул некорректный ответ. Повторите попытку позже.", null, true, ex);
            }
        }
    }

    private string? TryReadServerMessage(string text)
    {
        if (string.IsNullOrWhiteSpace(text)) return null;
        var trimmed = text.TrimStart();
        if (trimmed.StartsWith("<!DOCTYPE", StringComparison.OrdinalIgnoreCase) ||
            trimmed.StartsWith("<html", StringComparison.OrdinalIgnoreCase))
            return null;

        try
        {
            var obj = JObject.Parse(text);
            foreach (var key in new[] { "message", "error", "detail" })
            {
                var value = obj[key];
                if (value != null && value.Type == JTokenType.String)
                    return value.Value<string>();
            }
        }
        catch (JsonException) { }
        return null;
    }

    public Task<ActivationResponse> ActivateAsync(string code, string deviceName) =>
        Send<ActivationResponse>(Req(HttpMethod.Post, "api/v2/activate", body: new { code, device_name = deviceName }));

    public Task<TubeListResponse> GetTubesAsync(string token) =>
        Send<TubeListResponse>(Req(HttpMethod.Get, "api/v3/tubes", token));

    public async Task ImportAsync(string token, IReadOnlyList<ImportItem> items)
    {
        using var req = Req(HttpMethod.Post, "api/v1/tubes/import", token, new { items });
        await Send<JToken>(req);
    }

    public async Task UpdateStatusAsync(string token, string thu, string status, string comment)
    {
        using var req = Req(new HttpMethod("PATCH"), $"api/v3/tubes/{Uri.EscapeDataString(thu)}/status", token, new { status, comment });
        await Send<JToken>(req);
    }

    public Task<EnrollmentResponse> CreateEnrollmentAsync(string token) =>
        Send<EnrollmentResponse>(Req(HttpMethod.Post, "api/v2/enrollments", token, new { }));

    public Task<DeviceListResponse> GetDevicesAsync(string token) =>
        Send<DeviceListResponse>(Req(HttpMethod.Get, "api/v2/devices", token));

    public async Task RevokeDeviceAsync(string token, string id)
    {
        using var req = Req(HttpMethod.Post, $"api/v2/devices/{Uri.EscapeDataString(id)}/revoke", token, new { });
        await Send<JToken>(req);
    }
}
''', encoding="utf-8", newline="\n")

config_path.write_text(r'''using System.Security.Cryptography;
using System.Text;
using Newtonsoft.Json;

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
            var dto = JsonConvert.DeserializeObject<ConfigDto>(File.ReadAllText(_path)) ?? new ConfigDto();
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
                Theme = dto.Theme == "light" ? "light" : "dark",
                OverdueDays = dto.OverdueDays >= 1 && dto.OverdueDays <= 365 ? dto.OverdueDays : 14
            };
        }
        catch
        {
            return new AppConfig();
        }
    }

    public void Save(AppConfig config)
    {
        var protectedToken = "";
        if (!string.IsNullOrWhiteSpace(config.DeviceToken))
        {
            var raw = Encoding.UTF8.GetBytes(config.DeviceToken);
            protectedToken = Convert.ToBase64String(
                ProtectedData.Protect(raw, null, DataProtectionScope.CurrentUser));
        }

        var dto = new ConfigDto
        {
            Server = config.Server,
            ProtectedToken = protectedToken,
            DeviceId = config.DeviceId,
            DeviceName = config.DeviceName,
            Theme = config.Theme,
            OverdueDays = Math.Min(365, Math.Max(1, config.OverdueDays))
        };

        File.WriteAllText(_path, JsonConvert.SerializeObject(dto, Formatting.Indented), Encoding.UTF8);
    }

    private sealed class ConfigDto
    {
        public string? Server { get; set; }
        public string? ProtectedToken { get; set; }
        public string? DeviceId { get; set; }
        public string? DeviceName { get; set; }
        public string? Theme { get; set; }
        public int OverdueDays { get; set; } = 14;
    }
}
''', encoding="utf-8", newline="\n")

# Win7/.NET Framework compatibility for string split/join overloads.
e = excel_path.read_text(encoding="utf-8")
e = e.replace(
    "string.Join(' ', s.Replace('\\u00A0', ' ').Split(' ', StringSplitOptions.RemoveEmptyEntries))",
    "string.Join(\" \", s.Replace('\\u00A0', ' ').Split(new[] { ' ' }, StringSplitOptions.RemoveEmptyEntries))"
)
excel_path.write_text(e, encoding="utf-8", newline="\n")

# Remove .NET Core-only TextBox.PlaceholderText/Math.Clamp/System.Text.Json usages.
m = main_path.read_text(encoding="utf-8")
m = m.replace('using System.Text.Json;\n', 'using Newtonsoft.Json;\n')
m = m.replace('JsonSerializer.Serialize(', 'JsonConvert.SerializeObject(')
m = m.replace(
    'private readonly TextBox _search = new() { PlaceholderText = "Поиск по THU, заказу или магазину…" };',
    'private readonly TextBox _search = new();'
)
m = m.replace(
    'private readonly TextBox _comment = new() { Multiline = true, PlaceholderText = "Добавить комментарий…", MaxLength = 500, BorderStyle = BorderStyle.FixedSingle };',
    'private readonly TextBox _comment = new() { Multiline = true, MaxLength = 500, BorderStyle = BorderStyle.FixedSingle };'
)
m = m.replace('            PlaceholderText = placeholder,\n', '')
m = re.sub(r'\s*PlaceholderText\s*=\s*placeholder\s*,?', '', m)
m = re.sub(r'\s*PlaceholderText\s*=\s*"[^"]*"\s*,?', '', m)
m = m.replace(
    'Math.Clamp(_config.OverdueDays, 1, 365)',
    'Math.Min(365, Math.Max(1, _config.OverdueDays))'
)
main_path.write_text(m, encoding="utf-8", newline="\n")

a = activation_path.read_text(encoding="utf-8")
a = a.replace(', PlaceholderText = "TC-XXXX-XXXX"', '')
activation_path.write_text(a, encoding="utf-8", newline="\n")

# Do not depend on any Segoe face on Windows 7. Explicit UI fonts use legacy-safe families.
u = ui_path.read_text(encoding="utf-8")
u = u.replace(
    'new[] { "Segoe UI Symbol", "Segoe UI", "Tahoma", "Arial", "Microsoft Sans Serif" }',
    'new[] { "Tahoma", "Arial", "Microsoft Sans Serif" }'
)
u = u.replace(
    'new[] { "Segoe UI", "Tahoma", "Arial", "Microsoft Sans Serif" }',
    'new[] { "Tahoma", "Arial", "Microsoft Sans Serif" }'
)
u = u.replace(
    'new[] { "Tahoma", "Arial", "Microsoft Sans Serif" });\n\n    public static Font CreateSafeDefault',
    'new[] { "Tahoma", "Arial", "Microsoft Sans Serif" });\n\n    public static Font CreateSafeDefault'
)
ui_path.write_text(u, encoding="utf-8", newline="\n")

print("v3.9.0 Windows 7 compatibility patch applied")
