using System.Net;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json;

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
    private readonly JsonSerializerOptions _json = new() { PropertyNameCaseInsensitive = true };

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
        if (!string.IsNullOrWhiteSpace(token)) req.Headers.Authorization = new AuthenticationHeaderValue("Bearer", token);
        if (body is not null) req.Content = JsonContent.Create(body);
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
                if (resp.StatusCode is HttpStatusCode.BadGateway or HttpStatusCode.ServiceUnavailable or HttpStatusCode.GatewayTimeout)
                    throw new TubeControlApiException($"Сервер TubeControl временно недоступен ({code}). Повторите попытку через несколько минут.", code, true);

                if (resp.StatusCode == HttpStatusCode.Unauthorized)
                    throw new TubeControlApiException("Устройство не авторизовано. Откройте «Настройки» и проверьте подключение устройства.", code);

                if (resp.StatusCode == HttpStatusCode.Forbidden)
                    throw new TubeControlApiException("Доступ для этого устройства запрещён.", code);

                if (resp.StatusCode == HttpStatusCode.NotFound)
                    throw new TubeControlApiException("Запрошенные данные не найдены на сервере.", code);

                var serverMessage = TryReadServerMessage(text);
                throw new TubeControlApiException(serverMessage ?? $"Сервер вернул ошибку {code}.", code, code >= 500);
            }

            try
            {
                return JsonSerializer.Deserialize<T>(text, _json)
                    ?? throw new TubeControlApiException("Сервер вернул пустой ответ.");
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
            var e = JsonSerializer.Deserialize<Dictionary<string, JsonElement>>(text, _json);
            if (e is null) return null;
            foreach (var key in new[] { "message", "error", "detail" })
            {
                if (e.TryGetValue(key, out var value) && value.ValueKind == JsonValueKind.String)
                    return value.GetString();
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
