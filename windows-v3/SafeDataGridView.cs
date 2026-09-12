namespace TubeControl.Windows;

// WinForms DataGridView rejects Color.Transparent at runtime. The redesigned
// interface intentionally uses transparent-looking cards, so this small
// compatibility wrapper translates transparent requests into the current
// dark panel color instead of allowing the form constructor to crash.
internal sealed class DataGridView : System.Windows.Forms.DataGridView
{
    public new Color BackgroundColor
    {
        get => base.BackgroundColor;
        set => base.BackgroundColor = value == Color.Transparent ? UiPalette.DarkPanel : value;
    }
}
