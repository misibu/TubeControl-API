from pathlib import Path

program_path = Path("windows-v3/Program.cs")
ui_path = Path("windows-v3/UiKit.cs")

p = program_path.read_text(encoding="utf-8")
u = ui_path.read_text(encoding="utf-8")

# v3.8.4 — avoid WinForms' generated Segoe UI default font initialization.
# Some older/corporate PCs have a damaged or incomplete Segoe UI installation where
# Font("Segoe UI", Regular) throws before TubeControl can create its own controls.

old = '''        ApplicationConfiguration.Initialize();
        var store = new ConfigStore();'''
new = '''        InitializeApplication();
        var store = new ConfigStore();'''
if old not in p:
    raise SystemExit("ApplicationConfiguration.Initialize marker not found")
p = p.replace(old, new, 1)

marker = '''    [STAThread]
    [SupportedOSPlatform("windows")]
    static void Main()
'''
helper = '''    private static void InitializeApplication()
    {
        // Do not use ApplicationConfiguration.Initialize(): generated WinForms startup
        // can request Segoe UI Regular before our UI font fallback is active.
        try { Application.SetHighDpiMode(HighDpiMode.SystemAware); } catch { }
        Application.EnableVisualStyles();
        Application.SetCompatibleTextRenderingDefault(false);

        try
        {
            Application.SetDefaultFont(UiFonts.Create(9f, FontStyle.Regular));
        }
        catch
        {
            // If the OS rejects changing the default font, controls still use the
            // explicit UiFonts fallback applied throughout TubeControl.
        }
    }

'''
if "private static void InitializeApplication()" not in p:
    if marker not in p:
        raise SystemExit("Program.Main marker not found")
    p = p.replace(marker, helper + marker, 1)

# Improve the fallback policy: if Segoe UI lacks the requested style, try the next
# family (Tahoma/Arial) with the requested style before accepting another style.
old_block = '''        foreach (var name in candidates)
        {
            try
            {
                using var family = new FontFamily(name);
                if (family.IsStyleAvailable(style))
                    return new Font(family, size, style, GraphicsUnit.Point);
                if (family.IsStyleAvailable(FontStyle.Regular))
                    return new Font(family, size, FontStyle.Regular, GraphicsUnit.Point);
                if (family.IsStyleAvailable(FontStyle.Bold))
                    return new Font(family, size, FontStyle.Bold, GraphicsUnit.Point);
            }
            catch
            {
                // Try the next installed family.
            }
        }

        try
        {
            var system = SystemFonts.MessageBoxFont;'''
new_block = '''        // First pass: preserve the requested style, changing family if necessary.
        foreach (var name in candidates)
        {
            try
            {
                using var family = new FontFamily(name);
                if (family.IsStyleAvailable(style))
                    return new Font(family, size, style, GraphicsUnit.Point);
            }
            catch
            {
                // Try the next installed family.
            }
        }

        // Second pass: any usable style is better than failing startup.
        foreach (var name in candidates)
        {
            try
            {
                using var family = new FontFamily(name);
                if (family.IsStyleAvailable(FontStyle.Regular))
                    return new Font(family, size, FontStyle.Regular, GraphicsUnit.Point);
                if (family.IsStyleAvailable(FontStyle.Bold))
                    return new Font(family, size, FontStyle.Bold, GraphicsUnit.Point);
            }
            catch
            {
                // Try the next installed family.
            }
        }

        try
        {
            var system = SystemFonts.MessageBoxFont;'''
if old_block not in u:
    raise SystemExit("UiFonts fallback block not found")
u = u.replace(old_block, new_block, 1)

program_path.write_text(p, encoding="utf-8", newline="\n")
ui_path.write_text(u, encoding="utf-8", newline="\n")
print("v3.8.4 startup font compatibility patch applied")
