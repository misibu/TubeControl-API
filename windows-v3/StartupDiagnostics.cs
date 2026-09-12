using System.Runtime.CompilerServices;
using System.Text;

namespace TubeControl.Windows;

internal static class StartupDiagnostics
{
    private static readonly string LogPath = Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
        "TubeControl", "startup-error.log");

    [ModuleInitializer]
    internal static void Init()
    {
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
            Directory.CreateDirectory(Path.GetDirectoryName(LogPath)!);
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
