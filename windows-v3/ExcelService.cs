using ClosedXML.Excel;
using System.Globalization;

namespace TubeControl.Windows;

public static class ExcelService
{
    public static List<ImportItem> ReadImport(string path)
    {
        using var wb = new XLWorkbook(path);
        var ws = wb.Worksheets.First();
        var date = ReadShipmentDate(ws.Cell("H6"));
        if (date is null) throw new InvalidOperationException("Не удалось прочитать дату отгрузки из H6");

        int headerRow = 0, thuCol = 0, storeCol = 0, orderCol = 0;
        var lastCol = Math.Min(ws.LastColumnUsed()?.ColumnNumber() ?? 80, 120);
        for (int r = 1; r <= Math.Min(ws.LastRowUsed()?.RowNumber() ?? 40, 50); r++)
        {
            for (int c = 1; c <= lastCol; c++)
            {
                var h = Norm(ws.Cell(r, c).GetFormattedString());
                if (h == Norm("№ ЕО")) { headerRow = r; thuCol = c; }
            }
            if (headerRow > 0) break;
        }
        if (headerRow == 0) throw new InvalidOperationException("Не найден заголовок «№ ЕО»");

        for (int c = 1; c <= lastCol; c++)
        {
            var h = Norm(ws.Cell(headerRow, c).GetFormattedString());
            if (h is "КОД ТК" or "КОД" or "КОД МАГАЗИНА") storeCol = c;
            if (h == Norm("№ заказа на поставку SAP")) orderCol = c;
        }
        if (storeCol == 0) throw new InvalidOperationException("Не найден столбец «Код ТК»");
        if (orderCol == 0) throw new InvalidOperationException("Не найден столбец «№ заказа на поставку SAP»");

        var result = new List<ImportItem>();
        var lastRow = ws.LastRowUsed()?.RowNumber() ?? headerRow;
        for (int r = headerRow + 1; r <= lastRow; r++)
        {
            var thu = ws.Cell(r, thuCol).GetFormattedString().Trim().ToUpperInvariant();
            if (string.IsNullOrWhiteSpace(thu)) continue;
            result.Add(new ImportItem
            {
                Thu = thu,
                StoreCode = ws.Cell(r, storeCol).GetFormattedString().Trim(),
                OrderNumber = ws.Cell(r, orderCol).GetFormattedString().Trim(),
                ShippedAt = date.Value.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture)
            });
        }
        if (result.Count == 0) throw new InvalidOperationException("В файле не найдено THU для импорта");
        return result;
    }

    public static void Export(string path, IEnumerable<TubeItem> rows)
    {
        using var wb = new XLWorkbook();
        var ws = wb.AddWorksheet("Не возвращены");
        ws.Cell(1, 1).Value = "THU"; ws.Cell(1, 2).Value = "Дата отправки"; ws.Cell(1, 3).Value = "Магазин"; ws.Cell(1, 4).Value = "№ заказа"; ws.Cell(1, 5).Value = "Статус";
        var i = 2;
        foreach (var t in rows)
        {
            ws.Cell(i, 1).Value = t.Thu; ws.Cell(i, 2).Value = t.ShippedAt; ws.Cell(i, 3).Value = t.StoreCode; ws.Cell(i, 4).Value = t.OrderNumber; ws.Cell(i, 5).Value = StatusRu(t.Status); i++;
        }
        ws.Row(1).Style.Font.Bold = true;
        ws.Columns().AdjustToContents();
        wb.SaveAs(path);
    }

    private static DateTime? ReadShipmentDate(IXLCell cell)
    {
        if (cell.TryGetValue<DateTime>(out var dt)) return dt.Date;
        var s = cell.GetFormattedString().Trim();
        var cultures = new[] { CultureInfo.GetCultureInfo("ru-RU"), CultureInfo.InvariantCulture };
        foreach (var c in cultures) if (DateTime.TryParse(s, c, DateTimeStyles.None, out dt)) return dt.Date;
        return null;
    }

    private static string Norm(string s) => string.Join(' ', s.Replace('\u00A0', ' ').Split(' ', StringSplitOptions.RemoveEmptyEntries)).Trim().ToUpperInvariant();
    public static string StatusRu(string s) => s switch { "sent" => "Отправлен", "transit" => "В пути", "returned" => "Возвращён", "lost" => "Потерян", "overdue" => "Просрочен", _ => s };
}
