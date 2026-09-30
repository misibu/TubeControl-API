from pathlib import Path

program_path = Path("windows-v3/Program.cs")
ui_path = Path("windows-v3/UiKit.cs")
project_path = Path("windows-v3/TubeControl.Windows.csproj")

p = program_path.read_text(encoding="utf-8")
u = ui_path.read_text(encoding="utf-8")
c = project_path.read_text(encoding="utf-8")

# v3.8.5 — hardened startup for older/corporate Windows PCs with a broken
# or incomplete Segoe UI installation.  The bootstrap font must never depend
# on Segoe UI; only after WinForms has a safe default may the normal UI probe it.

# Add Microsoft Sans Serif as an additional fallback for explicit UI fonts.
u = u.replace(
    'new[] { "Segoe UI", "Tahoma", "Arial" }',
    'new[] { "Segoe UI", "Tahoma", "Arial", "Microsoft Sans Serif" }'
)
u = u.replace(
    'new[] { "Segoe UI Symbol", "Segoe UI", "Tahoma", "Arial" }',
    'new[] { "Segoe UI Symbol", "Segoe UI", "Tahoma", "Arial", "Microsoft Sans Serif" }'
)

# Add a bootstrap font path that deliberately does not touch Segoe UI.
marker = '''    public static Font CreateSymbol(float size, FontStyle style = FontStyle.Regular) =>
        CreateFromCandidates(size, style, new[] { "Segoe UI Symbol", "Segoe UI", "Tahoma", "Arial", "Microsoft Sans Serif" });

'''
bootstrap = '''    public static Font CreateSafeDefault(float size) =>
        CreateFromCandidates(size, FontStyle.Regular, new[] { "Tahoma", "Arial", "Microsoft Sans Serif" });

'''
if "public static Font CreateSafeDefault(" not in u:
    if marker not in u:
        raise SystemExit("UiFonts CreateSymbol marker not found")
    u = u.replace(marker, marker + bootstrap, 1)

# Set the safe legacy font before visual styles are enabled, so even base Control
# construction cannot fall back to a damaged Segoe UI Regular face.
old_init = '''        try { Application.SetHighDpiMode(HighDpiMode.SystemAware); } catch { }
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
        }'''
new_init = '''        try { Application.SetHighDpiMode(HighDpiMode.SystemAware); } catch { }

        try
        {
            Application.SetDefaultFont(UiFonts.CreateSafeDefault(9f));
        }
        catch
        {
            // Continue with the operating-system default. Explicit controls still
            // use UiFonts and therefore have their own fallback chain.
        }

        Application.EnableVisualStyles();
        Application.SetCompatibleTextRenderingDefault(false);'''
if old_init not in p:
    raise SystemExit("v3.8.4 InitializeApplication block not found")
p = p.replace(old_init, new_init, 1)

# Build metadata.
c = c.replace("<Version>3.8.4</Version>", "<Version>3.8.5</Version>")

program_path.write_text(p, encoding="utf-8", newline="\n")
ui_path.write_text(u, encoding="utf-8", newline="\n")
project_path.write_text(c, encoding="utf-8", newline="\n")
print("v3.8.5 hardened font startup patch applied")
