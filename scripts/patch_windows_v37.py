from pathlib import Path

main_path = Path("windows-v3/MainForm.cs")
ui_path = Path("windows-v3/UiKit.cs")

s = main_path.read_text(encoding="utf-8")
u = ui_path.read_text(encoding="utf-8")


def replace_between(text: str, start_marker: str, end_marker: str, replacement: str) -> str:
    start = text.index(start_marker)
    end = text.index(end_marker, start)
    return text[:start] + replacement + "\n\n" + text[end:]

# ---------------------------------------------------------------------------
# v3.7 — final dark-theme surfaces, compact toolbar labels and custom table marks
# ---------------------------------------------------------------------------

# Short labels requested by the user.
s = s.replace(
    'var ret = Action("Зарегистрировать возврат", async () => await ManualReturn(), 210);',
    'var ret = Action("Возврат", async () => await ManualReturn(), 112);',
)
s = s.replace(
    'var status = Action("Изменить статус", EditSelectedStatus, 160);',
    'var status = Action("Редактировать", EditSelectedStatus, 132);',
)

# The search card needs an explicit accent tag so recursive theme updates can always
# recolor it after Light -> Dark switching instead of leaving the previous white surface.
s = s.replace(
    'BorderColor = UiPalette.Emerald,\n            Margin = Padding.Empty\n        };\n        _search.BorderStyle',
    'BorderColor = UiPalette.Emerald,\n            Tag = "accent-surface",\n            Margin = Padding.Empty\n        };\n        _search.BorderStyle',
)

# Owner draw the filter items so Windows never paints the default blue selection block.
s = s.replace(
    '_filter.FlatStyle = FlatStyle.Flat;\n        if (_filter.Items.Count == 0)',
    '_filter.FlatStyle = FlatStyle.Flat;\n        _filter.DrawMode = DrawMode.OwnerDrawFixed;\n        _filter.ItemHeight = 28;\n        _filter.DrawItem -= FilterDrawItem;\n        _filter.DrawItem += FilterDrawItem;\n        if (_filter.Items.Count == 0)',
)

# Prevent native blue header-selection color and configure the first column for custom painting.
s = s.replace(
    '_grid.ColumnHeadersHeightSizeMode = DataGridViewColumnHeadersHeightSizeMode.DisableResizing;\n        _grid.RowTemplate.Height = 42;',
    '_grid.ColumnHeadersHeightSizeMode = DataGridViewColumnHeadersHeightSizeMode.DisableResizing;\n        _grid.ColumnHeadersDefaultCellStyle.SelectionBackColor = _grid.ColumnHeadersDefaultCellStyle.BackColor;\n        _grid.ColumnHeadersDefaultCellStyle.SelectionForeColor = _grid.ColumnHeadersDefaultCellStyle.ForeColor;\n        _grid.RowTemplate.Height = 42;',
)

# Replace the standard checkbox column with a plain value column. The mark is now drawn
# completely by GridCellPainting, so the native blue checkbox can never appear.
s = s.replace(
    '_grid.Columns.Add(new DataGridViewCheckBoxColumn { Name = "Check", HeaderText = "", FillWeight = 8, ReadOnly = true });',
    '_grid.Columns.Add(new DataGridViewTextBoxColumn { Name = "Check", HeaderText = "", FillWeight = 8, ReadOnly = true, SortMode = DataGridViewColumnSortMode.NotSortable });',
)

# Full custom cell paint for the first column + existing status pill rendering.
grid_paint = r'''    private void GridCellPainting(object? sender, DataGridViewCellPaintingEventArgs e)
    {
        var checkIndex = _grid.Columns["Check"]?.Index ?? -1;
        var statusIndex = _grid.Columns["Status"]?.Index ?? -1;

        if (e.ColumnIndex == checkIndex)
        {
            if (e.RowIndex < 0)
            {
                var headerBack = DarkTheme ? Color.FromArgb(6, 19, 15) : Color.FromArgb(236, 245, 241);
                using var hb = new SolidBrush(headerBack);
                e.Graphics.FillRectangle(hb, e.CellBounds);
                using var line = new Pen(Border, 1);
                e.Graphics.DrawLine(line, e.CellBounds.Left, e.CellBounds.Bottom - 1, e.CellBounds.Right, e.CellBounds.Bottom - 1);
                e.Handled = true;
                return;
            }

            e.PaintBackground(e.CellBounds, true);
            var isChecked = e.Value is bool b && b;
            var size = 15;
            var box = new Rectangle(
                e.CellBounds.X + (e.CellBounds.Width - size) / 2,
                e.CellBounds.Y + (e.CellBounds.Height - size) / 2,
                size,
                size);

            e.Graphics.SmoothingMode = SmoothingMode.AntiAlias;
            using var path = UiDrawing.Rounded(box, 4);
            var borderColor = isChecked ? UiPalette.EmeraldBright : (DarkTheme ? Color.FromArgb(120, 157, 144) : Color.FromArgb(112, 137, 127));
            using var pen = new Pen(borderColor, 1.2f);
            using var fill = new SolidBrush(isChecked
                ? (DarkTheme ? Color.FromArgb(11, 100, 73) : Color.FromArgb(198, 242, 225))
                : (DarkTheme ? Color.FromArgb(5, 18, 14) : Color.FromArgb(248, 251, 250)));
            e.Graphics.FillPath(fill, path);
            e.Graphics.DrawPath(pen, path);

            if (isChecked)
            {
                using var check = new Pen(DarkTheme ? Color.White : Color.FromArgb(5, 76, 51), 2f)
                {
                    StartCap = LineCap.Round,
                    EndCap = LineCap.Round
                };
                e.Graphics.DrawLines(check, new[]
                {
                    new Point(box.X + 3, box.Y + 8),
                    new Point(box.X + 6, box.Y + 11),
                    new Point(box.X + 12, box.Y + 4)
                });
            }

            e.Handled = true;
            return;
        }

        if (e.RowIndex < 0 || e.ColumnIndex != statusIndex) return;

        e.PaintBackground(e.CellBounds, true);
        var text = e.FormattedValue?.ToString() ?? "";
        var color = UiDrawing.StatusColor(text);
        var width = Math.Min(e.CellBounds.Width - 14, Math.Max(88, TextRenderer.MeasureText(text, _grid.Font).Width + 38));
        var pill = new Rectangle(e.CellBounds.X + 7, e.CellBounds.Y + 8, width, e.CellBounds.Height - 16);

        e.Graphics.SmoothingMode = SmoothingMode.AntiAlias;
        using var statusPath = UiDrawing.Rounded(pill, pill.Height / 2);
        using var statusFill = new SolidBrush(Color.FromArgb(DarkTheme ? 45 : 28, color));
        using var statusPen = new Pen(Color.FromArgb(190, color), 1);
        e.Graphics.FillPath(statusFill, statusPath);
        e.Graphics.DrawPath(statusPen, statusPath);

        var dot = new Rectangle(pill.X + 10, pill.Y + pill.Height / 2 - 4, 8, 8);
        using var dotBrush = new SolidBrush(color);
        e.Graphics.FillEllipse(dotBrush, dot);
        var textRect = new Rectangle(pill.X + 25, pill.Y, pill.Width - 30, pill.Height);
        TextRenderer.DrawText(e.Graphics, text, _grid.Font, textRect, color,
            TextFormatFlags.Left | TextFormatFlags.VerticalCenter | TextFormatFlags.EndEllipsis);

        e.Handled = true;
    }'''
s = replace_between(s, "    private void GridCellPainting(object? sender, DataGridViewCellPaintingEventArgs e)\n", "    private void ShowSelected()\n", grid_paint)

# Draw filter entries with app colors rather than the native Windows blue highlight.
filter_draw = r'''    private void FilterDrawItem(object? sender, DrawItemEventArgs e)
    {
        if (e.Index < 0) return;
        var text = _filter.Items[e.Index]?.ToString() ?? string.Empty;
        var selected = (e.State & DrawItemState.Selected) != 0;
        var back = selected
            ? (DarkTheme ? Color.FromArgb(13, 64, 49) : Color.FromArgb(214, 242, 231))
            : Panel2;
        var fore = selected
            ? (DarkTheme ? Color.White : Color.FromArgb(10, 72, 49))
            : Fg;
        using var bg = new SolidBrush(back);
        e.Graphics.FillRectangle(bg, e.Bounds);
        TextRenderer.DrawText(e.Graphics, text, _filter.Font,
            new Rectangle(e.Bounds.X + 8, e.Bounds.Y, e.Bounds.Width - 12, e.Bounds.Height),
            fore, TextFormatFlags.Left | TextFormatFlags.VerticalCenter | TextFormatFlags.EndEllipsis);
    }
'''
marker = "    private void StatusDrawItem(object? sender, DrawItemEventArgs e)\n"
if "private void FilterDrawItem(" not in s:
    s = s.replace(marker, filter_draw + "\n" + marker)

# Theme switching must always repaint rounded surfaces. Previous code only recolored a
# panel if it was still the original dark value; after switching to light that condition
# was false, leaving a white pill under the dark search field.
old_rounded = '''        if (root is RoundedPanel rp)
        {
            if (rp != _details && rp.BackColor == UiPalette.DarkPanel) rp.BackColor = Panel;
            rp.BorderColor = Border;
        }'''
new_rounded = '''        if (root is RoundedPanel rp)
        {
            rp.BackColor = Panel;
            rp.BorderColor = rp.Tag is string surfaceTag && surfaceTag == "accent-surface"
                ? UiPalette.Emerald
                : Border;
            rp.Invalidate();
        }'''
s = s.replace(old_rounded, new_rounded)

# Explicitly keep grid header selection colors synchronized with the actual theme.
s = s.replace(
    '_grid.ColumnHeadersDefaultCellStyle.Padding = new Padding(7, 0, 3, 0);\n        _grid.GridColor = Border;',
    '_grid.ColumnHeadersDefaultCellStyle.Padding = new Padding(7, 0, 3, 0);\n        _grid.ColumnHeadersDefaultCellStyle.SelectionBackColor = _grid.ColumnHeadersDefaultCellStyle.BackColor;\n        _grid.ColumnHeadersDefaultCellStyle.SelectionForeColor = _grid.ColumnHeadersDefaultCellStyle.ForeColor;\n        _grid.GridColor = Border;',
)

# Repaint filter immediately when theme changes.
s = s.replace(
    '_filter.BackColor = Panel;\n        _filter.ForeColor = Fg;',
    '_filter.BackColor = Panel2;\n        _filter.ForeColor = Fg;\n        _filter.Invalidate();',
)

main_path.write_text(s, encoding="utf-8", newline="\n")
ui_path.write_text(u, encoding="utf-8", newline="\n")
