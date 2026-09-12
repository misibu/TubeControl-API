using System.Runtime.Versioning;

namespace TubeControl.Windows;

internal static class Program
{
    [STAThread]
    [SupportedOSPlatform("windows")]
    static void Main()
    {
        ApplicationConfiguration.Initialize();
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
