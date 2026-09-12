using System.Text.Json.Serialization;

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
    [JsonPropertyName("thu")] public string Thu { get; set; } = "";
    [JsonPropertyName("shipped_at")] public string ShippedAt { get; set; } = "";
    [JsonPropertyName("store_code")] public string StoreCode { get; set; } = "";
    [JsonPropertyName("order_number")] public string OrderNumber { get; set; } = "";
    [JsonPropertyName("returned_at")] public DateTimeOffset? ReturnedAt { get; set; }
    [JsonPropertyName("manual_status")] public string? ManualStatus { get; set; }
    [JsonPropertyName("status")] public string Status { get; set; } = "transit";
    [JsonPropertyName("status_comment")] public string? StatusComment { get; set; }
    [JsonPropertyName("updated_at")] public DateTimeOffset UpdatedAt { get; set; }
}

public sealed class TubeListResponse
{
    [JsonPropertyName("items")] public List<TubeItem> Items { get; set; } = new();
    [JsonPropertyName("count")] public int Count { get; set; }
}

public sealed class ImportItem
{
    [JsonPropertyName("thu")] public string Thu { get; set; } = "";
    [JsonPropertyName("shipped_at")] public string ShippedAt { get; set; } = "";
    [JsonPropertyName("store_code")] public string StoreCode { get; set; } = "";
    [JsonPropertyName("order_number")] public string OrderNumber { get; set; } = "";
}

public sealed class ActivationResponse
{
    [JsonPropertyName("device_token")] public string DeviceToken { get; set; } = "";
    [JsonPropertyName("device")] public DeviceIdentity Device { get; set; } = new();
}

public sealed class DeviceIdentity
{
    [JsonPropertyName("id")] public string Id { get; set; } = "";
    [JsonPropertyName("name")] public string Name { get; set; } = "";
    [JsonPropertyName("kind")] public string Kind { get; set; } = "";
}

public sealed class EnrollmentResponse
{
    [JsonPropertyName("code")] public string Code { get; set; } = "";
    [JsonPropertyName("expires_at")] public DateTimeOffset ExpiresAt { get; set; }
}

public sealed class DeviceRecord
{
    [JsonPropertyName("id")] public string Id { get; set; } = "";
    [JsonPropertyName("name")] public string Name { get; set; } = "";
    [JsonPropertyName("kind")] public string Kind { get; set; } = "";
    [JsonPropertyName("created_at")] public DateTimeOffset CreatedAt { get; set; }
    [JsonPropertyName("last_seen_at")] public DateTimeOffset? LastSeenAt { get; set; }
    [JsonPropertyName("revoked_at")] public DateTimeOffset? RevokedAt { get; set; }
}

public sealed class DeviceListResponse
{
    [JsonPropertyName("items")] public List<DeviceRecord> Items { get; set; } = new();
    [JsonPropertyName("current_device_id")] public string CurrentDeviceId { get; set; } = "";
}
