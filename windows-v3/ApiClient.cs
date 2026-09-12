using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json;

namespace TubeControl.Windows;

public sealed class ApiClient
{
    private readonly HttpClient _http;
    private readonly JsonSerializerOptions _json = new() { PropertyNameCaseInsensitive = true };

    public ApiClient(string baseUrl)
    {
        _http = new HttpClient { BaseAddress = new Uri(baseUrl.TrimEnd('/') + "/"), Timeout = TimeSpan.FromSeconds(30) };
    }

    private HttpRequestMessage Req(HttpMethod method, string path, string? token = null, object? body = null)
    {
        var req = new HttpRequestMessage(method, path);
        if (!string.IsNullOrWhiteSpace(token)) req.Headers.Authorization = new AuthenticationHeaderValue("Bearer", token);
        if (body is not null) req.Content = JsonContent.Create(body);
        return req;
    }

    private async Task<T> Send<T>(HttpRequestMessage req)
    {
        using var resp = await _http.SendAsync(req);
        var text = await resp.Content.ReadAsStringAsync();
        if (!resp.IsSuccessStatusCode)
        {
            try
            {
                var e = JsonSerializer.Deserialize<Dictionary<string, object>>(text, _json);
                if (e is not null && e.TryGetValue("message", out var msg)) throw new InvalidOperationException(msg?.ToString());
            }
            catch (JsonException) { }
            throw new InvalidOperationException($"HTTP {(int)resp.StatusCode}: {text}");
        }
        return JsonSerializer.Deserialize<T>(text, _json) ?? throw new InvalidOperationException("Пустой ответ сервера");
    }

    public Task<ActivationResponse> ActivateAsync(string code, string deviceName) =>
        Send<ActivationResponse>(Req(HttpMethod.Post, "api/v2/activate", body: new { code, device_name = deviceName }));

    public Task<TubeListResponse> GetTubesAsync(string token) =>
        Send<TubeListResponse>(Req(HttpMethod.Get, "api/v3/tubes", token));

    public async Task ImportAsync(string token, IReadOnlyList<ImportItem> items)
    {
        using var req = Req(HttpMethod.Post, "api/v1/tubes/import", token, new { items });
        await Send<JsonElement>(req);
    }

    public async Task UpdateStatusAsync(string token, string thu, string status, string comment)
    {
        using var req = Req(HttpMethod.Patch, $"api/v3/tubes/{Uri.EscapeDataString(thu)}/status", token, new { status, comment });
        await Send<JsonElement>(req);
    }

    public Task<EnrollmentResponse> CreateEnrollmentAsync(string token) =>
        Send<EnrollmentResponse>(Req(HttpMethod.Post, "api/v2/enrollments", token, new { }));

    public Task<DeviceListResponse> GetDevicesAsync(string token) =>
        Send<DeviceListResponse>(Req(HttpMethod.Get, "api/v2/devices", token));

    public async Task RevokeDeviceAsync(string token, string id)
    {
        using var req = Req(HttpMethod.Post, $"api/v2/devices/{Uri.EscapeDataString(id)}/revoke", token, new { });
        await Send<JsonElement>(req);
    }
}
