from pathlib import Path
import re

root = Path("windows-v3")
ui_path = root / "UiKit.cs"

# v3.8.3 — tolerate PCs where Segoe UI is missing, damaged, or has no Regular face.
# Never let a font family problem prevent TubeControl from starting.

u = ui_path.read_text(encoding="utf-8")

helper = r'''
internal static class UiFonts
{
    public static Font Create(float size, FontStyle style = FontStyle.Regular) =>
        CreateFromCandidates(size, style, new[] { "Segoe UI", "Tahoma", "Arial" });

    public static Font CreateSymbol(float size, FontStyle style = FontStyle.Regular) =>
        CreateFromCandidates(size, style, new[] { "Segoe UI Symbol", "Segoe UI", "Tahoma", "Arial" });

    private static Font CreateFromCandidates(float size, FontStyle style, IEnumerable<string> candidates)
    {
        foreach (var name in candidates)
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
            var system = SystemFonts.MessageBoxFont;
            var family = system.FontFamily;
            var safeStyle = family.IsStyleAvailable(style)
                ? style
                : family.IsStyleAvailable(FontStyle.Regular)
                    ? FontStyle.Regular
                    : system.Style;
            return new Font(family, size, safeStyle, GraphicsUnit.Point);
        }
        catch
        {
            // Microsoft Sans Serif is present on supported Windows installations.
            return new Font(FontFamily.GenericSansSerif, size, FontStyle.Regular, GraphicsUnit.Point);
        }
    }
}
'''

marker = "internal static class UiDrawing"
if "internal static class UiFonts" not in u:
    if marker not in u:
        raise SystemExit("UiDrawing marker not found")
    u = u.replace(marker, helper + "\n" + marker, 1)
    ui_path.write_text(u, encoding="utf-8", newline="\n")

regular_re = re.compile(r'new Font\("Segoe UI",\s*([^,\)\n]+)(?:,\s*([^\)\n]+))?\)')
symbol_re = re.compile(r'new Font\("Segoe UI Symbol",\s*([^,\)\n]+)(?:,\s*([^\)\n]+))?\)')

def regular_sub(m):
    size = m.group(1).strip()
    style = m.group(2)
    return f"UiFonts.Create({size}" + (f", {style.strip()}" if style else "") + ")"

def symbol_sub(m):
    size = m.group(1).strip()
    style = m.group(2)
    return f"UiFonts.CreateSymbol({size}" + (f", {style.strip()}" if style else "") + ")"

changed = 0
for path in root.glob("*.cs"):
    s = path.read_text(encoding="utf-8")
    ns, n1 = regular_re.subn(regular_sub, s)
    ns, n2 = symbol_re.subn(symbol_sub, ns)
    if n1 + n2:
        path.write_text(ns, encoding="utf-8", newline="\n")
        changed += n1 + n2

if changed == 0:
    raise SystemExit("No Segoe UI font constructors were patched")

print(f"Patched {changed} explicit Segoe UI font constructors with safe fallbacks")
