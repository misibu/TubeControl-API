from pathlib import Path

p = Path("windows-v3/MainForm.cs")
s = p.read_text(encoding="utf-8")


def replace_between(text: str, start_marker: str, end_marker: str, replacement: str) -> str:
    start = text.index(start_marker)
    end = text.index(end_marker, start)
    return text[:start] + replacement + "\n\n" + text[end:]

# In the minimal v3.4 layout the old right-side status editor is no longer built.
# ShowSelected must therefore never touch its hidden ComboBox (_status), otherwise
# setting SelectedIndex=0 on an empty item list throws InvalidArgumentException.
# Keep the v3.4 modal EditSelectedStatus method that sits immediately after it.
show_selected = r'''    private void ShowSelected()
    {
        foreach (DataGridViewRow row in _grid.Rows)
        {
            if (row.Cells.Count > 0)
                row.Cells[0].Value = _grid.SelectedRows.Count > 0 && row == _grid.SelectedRows[0];
        }
    }'''
s = replace_between(s, "    private void ShowSelected()\n", "    private async Task EditSelectedStatus()\n", show_selected)

# Make the minimal toolbar calmer and consistent with Android: same pill radius
# and enough width for Russian labels at common Windows scaling levels.
s = s.replace('var import = Action("Импорт Excel", async () => await ImportExcel(), 138);',
              'var import = Action("Импорт Excel", async () => await ImportExcel(), 142);')
s = s.replace('var add = Action("Добавить THU", async () => await AddThu(), 142);',
              'var add = Action("Добавить THU", async () => await AddThu(), 146);')
s = s.replace('var ret = Action("Зарегистрировать возврат", async () => await ManualReturn(), 196);',
              'var ret = Action("Зарегистрировать возврат", async () => await ManualReturn(), 206);')
s = s.replace('var status = Action("Изменить статус", EditSelectedStatus, 158);',
              'var status = Action("Изменить статус", EditSelectedStatus, 164);')
s = s.replace('var export = Action("Выгрузить", Export, 118);',
              'var export = Action("Выгрузить", Export, 122);')

# Remove native-looking focus rectangles from status drop-down owner drawing.
s = s.replace('        e.DrawFocusRectangle();\n', '')

p.write_text(s, encoding="utf-8", newline="\n")
