namespace TubeControl.Windows;

internal class Button : System.Windows.Forms.Button
{
    public Button()
    {
        SetStyle(ControlStyles.SupportsTransparentBackColor, true);
    }
}
