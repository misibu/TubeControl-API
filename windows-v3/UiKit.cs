using System.Drawing.Drawing2D;

namespace TubeControl.Windows;

internal static class UiPalette
{
    public static readonly Color Emerald = Color.FromArgb(16, 185, 129);
    public static readonly Color EmeraldBright = Color.FromArgb(24, 235, 164);
    public static readonly Color Mint = Color.FromArgb(108, 255, 203);
    public static readonly Color Dark = Color.FromArgb(3, 10, 8);
    public static readonly Color DarkPanel = Color.FromArgb(7, 22, 18);
    public static readonly Color DarkPanel2 = Color.FromArgb(10, 29, 24);
    public static readonly Color DarkBorder = Color.FromArgb(31, 74, 61);
    public static readonly Color Light = Color.FromArgb(240, 247, 244);
    public static readonly Color LightPanel = Color.White;
    public static readonly Color LightBorder = Color.FromArgb(191, 216, 205);
    public static readonly Color Blue = Color.FromArgb(44, 153, 255);
    public static readonly Color Yellow = Color.FromArgb(255, 198, 0);
    public static readonly Color Red = Color.FromArgb(255, 82, 82);
}

internal static class UiDrawing
{
    public static GraphicsPath Rounded(Rectangle bounds, int radius)
    {
        var path = new GraphicsPath();
        var r = Math.Max(2, Math.Min(radius, Math.Min(bounds.Width, bounds.Height) / 2));
        var d = r * 2;
        path.AddArc(bounds.X, bounds.Y, d, d, 180, 90);
        path.AddArc(bounds.Right - d, bounds.Y, d, d, 270, 90);
        path.AddArc(bounds.Right - d, bounds.Bottom - d, d, d, 0, 90);
        path.AddArc(bounds.X, bounds.Bottom - d, d, d, 90, 90);
        path.CloseFigure();
        return path;
    }

    public static Color StatusColor(string? value)
    {
        var s = (value ?? string.Empty).Trim().ToLowerInvariant();
        if (s.Contains("возвращ") || s == "returned") return UiPalette.EmeraldBright;
        if (s.Contains("потер") || s.Contains("просроч") || s == "lost" || s == "overdue") return UiPalette.Red;
        if (s.Contains("отправ") || s == "sent") return UiPalette.Blue;
        return UiPalette.Yellow;
    }
}

internal sealed class RoundedPanel : Panel
{
    public int Radius { get; set; } = 14;
    public int BorderWidth { get; set; } = 1;
    public Color BorderColor { get; set; } = UiPalette.DarkBorder;

    public RoundedPanel()
    {
        DoubleBuffered = true;
        BackColor = UiPalette.DarkPanel;
        Padding = new Padding(1);
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        base.OnPaint(e);
        e.Graphics.SmoothingMode = SmoothingMode.AntiAlias;
        var rect = new Rectangle(0, 0, Math.Max(1, Width - 1), Math.Max(1, Height - 1));
        using var path = UiDrawing.Rounded(rect, Radius);
        using var fill = new SolidBrush(BackColor);
        using var pen = new Pen(BorderColor, BorderWidth);
        e.Graphics.FillPath(fill, path);
        if (BorderWidth > 0) e.Graphics.DrawPath(pen, path);
    }
}

// Fully owner-drawn control. It intentionally does NOT derive from Button:
// native WinForms button painting can flash its rectangular background around
// rounded corners on mouse down, especially in the light theme.
internal sealed class GlowButton : Control
{
    private int _radius = 20;
    private bool _hover;
    private bool _pressed;

    public int Radius
    {
        get => _radius;
        set { _radius = Math.Max(2, value); UpdateRoundedRegion(); Invalidate(); }
    }

    public Color BorderColor { get; set; } = UiPalette.Emerald;
    public Color FillColor { get; set; } = UiPalette.DarkPanel;
    public Color HoverFillColor { get; set; } = UiPalette.DarkPanel2;
    public Color PressedFillColor { get; set; } = Color.FromArgb(12, 70, 53);
    public int BorderWidth { get; set; } = 1;
    public ContentAlignment TextAlign { get; set; } = ContentAlignment.MiddleCenter;

    public GlowButton()
    {
        SetStyle(ControlStyles.UserPaint |
                 ControlStyles.AllPaintingInWmPaint |
                 ControlStyles.OptimizedDoubleBuffer |
                 ControlStyles.ResizeRedraw |
                 ControlStyles.SupportsTransparentBackColor |
                 ControlStyles.Selectable, true);
        BackColor = Color.Transparent;
        ForeColor = Color.White;
        Cursor = Cursors.Hand;
        TabStop = true;
        Font = new Font("Segoe UI", 9f, FontStyle.Regular);
    }

    protected override void OnResize(EventArgs e)
    {
        base.OnResize(e);
        UpdateRoundedRegion();
    }

    protected override void OnParentChanged(EventArgs e)
    {
        base.OnParentChanged(e);
        UpdateRoundedRegion();
        Invalidate();
    }

    private void UpdateRoundedRegion()
    {
        if (Width <= 1 || Height <= 1) return;
        var rect = new Rectangle(0, 0, Width, Height);
        using var path = UiDrawing.Rounded(rect, Radius);
        var next = new Region(path);
        var old = Region;
        Region = next;
        old?.Dispose();
    }

    protected override void OnPaintBackground(PaintEventArgs pevent)
    {
        // The parent is visible outside Region, so never paint a rectangular
        // background that can appear as white/light corners when pressed.
    }

    protected override void OnMouseEnter(EventArgs e)
    {
        _hover = true;
        Invalidate();
        base.OnMouseEnter(e);
    }

    protected override void OnMouseLeave(EventArgs e)
    {
        _hover = false;
        _pressed = false;
        Invalidate();
        base.OnMouseLeave(e);
    }

    protected override void OnMouseDown(MouseEventArgs e)
    {
        if (e.Button == MouseButtons.Left && Enabled)
        {
            _pressed = true;
            Capture = true;
            Focus();
            Invalidate();
        }
        base.OnMouseDown(e);
    }

    protected override void OnMouseUp(MouseEventArgs e)
    {
        var fire = _pressed && e.Button == MouseButtons.Left && ClientRectangle.Contains(e.Location) && Enabled;
        _pressed = false;
        Capture = false;
        Invalidate();
        base.OnMouseUp(e);
        if (fire) OnClick(EventArgs.Empty);
    }

    protected override void OnKeyDown(KeyEventArgs e)
    {
        if (Enabled && (e.KeyCode == Keys.Space || e.KeyCode == Keys.Enter))
        {
            _pressed = true;
            Invalidate();
            e.Handled = true;
        }
        base.OnKeyDown(e);
    }

    protected override void OnKeyUp(KeyEventArgs e)
    {
        if (_pressed && Enabled && (e.KeyCode == Keys.Space || e.KeyCode == Keys.Enter))
        {
            _pressed = false;
            Invalidate();
            OnClick(EventArgs.Empty);
            e.Handled = true;
        }
        base.OnKeyUp(e);
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        g.PixelOffsetMode = PixelOffsetMode.HighQuality;

        var rect = new Rectangle(1, 1, Math.Max(1, Width - 3), Math.Max(1, Height - 3));
        var fillColor = !Enabled
            ? Color.FromArgb(35, 45, 42)
            : _pressed ? PressedFillColor : _hover ? HoverFillColor : FillColor;
        var borderColor = Enabled ? BorderColor : Color.FromArgb(80, 90, 86);

        using var path = UiDrawing.Rounded(rect, Radius);
        using var fill = new SolidBrush(fillColor);
        g.FillPath(fill, path);

        if (BorderWidth > 0)
        {
            using var pen = new Pen(borderColor, BorderWidth);
            g.DrawPath(pen, path);
        }

        var flags = TextFormatFlags.VerticalCenter | TextFormatFlags.EndEllipsis | TextFormatFlags.NoPadding;
        var textRect = Rectangle.Inflate(rect, -16, 0);
        if (TextAlign is ContentAlignment.MiddleLeft or ContentAlignment.TopLeft or ContentAlignment.BottomLeft)
            flags |= TextFormatFlags.Left;
        else if (TextAlign is ContentAlignment.MiddleRight or ContentAlignment.TopRight or ContentAlignment.BottomRight)
            flags |= TextFormatFlags.Right;
        else
            flags |= TextFormatFlags.HorizontalCenter;

        TextRenderer.DrawText(g, Text, Font, textRect, Enabled ? ForeColor : Color.Gray, flags);

        if (Focused && ShowFocusCues)
        {
            var focus = Rectangle.Inflate(rect, -4, -4);
            ControlPaint.DrawFocusRectangle(g, focus, ForeColor, Color.Transparent);
        }
    }
}

internal sealed class TubeIllustration : Control
{
    public bool DarkTheme { get; set; } = true;

    public TubeIllustration()
    {
        DoubleBuffered = true;
        MinimumSize = new Size(110, 80);
        BackColor = UiPalette.DarkPanel;
    }

    protected override void OnParentChanged(EventArgs e)
    {
        base.OnParentChanged(e);
        if (Parent is not null) BackColor = Parent.BackColor;
    }

    protected override void OnPaint(PaintEventArgs e)
    {
        base.OnPaint(e);
        var g = e.Graphics;
        g.SmoothingMode = SmoothingMode.AntiAlias;
        g.TranslateTransform(Width * .12f, Height * .12f);
        g.RotateTransform(-24f);

        var body = new RectangleF(16, 18, Math.Max(62, Width * .55f), Math.Max(34, Height * .30f));
        using (var shadow = new SolidBrush(Color.FromArgb(45, UiPalette.EmeraldBright)))
            g.FillEllipse(shadow, body.X - 8, body.Y - 8, body.Width + 34, body.Height + 34);
        using (var bodyBrush = new LinearGradientBrush(body, Color.FromArgb(6, 15, 13), Color.FromArgb(22, 40, 33), 0f))
        using (var borderPen = new Pen(UiPalette.Mint, 3f))
        {
            g.FillRectangle(bodyBrush, body);
            g.DrawRectangle(borderPen, body.X, body.Y, body.Width, body.Height);
            g.DrawEllipse(borderPen, body.Right - body.Height * .45f, body.Y, body.Height * .45f, body.Height);
        }

        var band = new RectangleF(body.X + body.Width * .43f, body.Y + 2, body.Width * .22f, body.Height - 4);
        using (var bandBrush = new SolidBrush(Color.FromArgb(190, 215, 208))) g.FillRectangle(bandBrush, band);
        using (var barPen = new Pen(Color.FromArgb(5, 85, 62), 2f))
        {
            for (var i = 0; i < 5; i++)
            {
                var x = band.X + 4 + i * 4;
                g.DrawLine(barPen, x, band.Y + 5, x, band.Bottom - 5);
            }
        }

        g.ResetTransform();
        var c = new Rectangle(Width - 54, Height - 54, 44, 44);
        using var glow = new SolidBrush(Color.FromArgb(70, UiPalette.EmeraldBright));
        g.FillEllipse(glow, c.X - 5, c.Y - 5, c.Width + 10, c.Height + 10);
        using var circleFill = new SolidBrush(Color.FromArgb(8, 75, 56));
        using var circlePen = new Pen(UiPalette.Mint, 2.5f);
        g.FillEllipse(circleFill, c);
        g.DrawEllipse(circlePen, c);
        using var check = new Pen(Color.White, 4f) { StartCap = LineCap.Round, EndCap = LineCap.Round };
        g.DrawLines(check, new[] { new Point(c.X + 11, c.Y + 23), new Point(c.X + 18, c.Y + 30), new Point(c.X + 33, c.Y + 14) });
    }
}