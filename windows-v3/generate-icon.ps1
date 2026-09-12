Add-Type -AssemblyName System.Drawing
$bmp = New-Object System.Drawing.Bitmap 256,256
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
$g.Clear([System.Drawing.Color]::FromArgb(5,12,10))
$rect = New-Object System.Drawing.RectangleF 8,8,240,240
$brush = New-Object System.Drawing.Drawing2D.LinearGradientBrush($rect,[System.Drawing.Color]::FromArgb(9,26,21),[System.Drawing.Color]::FromArgb(4,12,10),45)
$g.FillRectangle($brush,8,8,240,240)
$pen = New-Object System.Drawing.Pen([System.Drawing.Color]::FromArgb(16,185,129),8)
$g.DrawEllipse($pen,30,58,160,90)
$g.DrawLine($pen,55,75,155,140)
$g.DrawEllipse($pen,142,132,76,76)
$checkPen = New-Object System.Drawing.Pen([System.Drawing.Color]::FromArgb(108,255,203),11)
$checkPen.StartCap=[System.Drawing.Drawing2D.LineCap]::Round; $checkPen.EndCap=[System.Drawing.Drawing2D.LineCap]::Round
$g.DrawLines($checkPen,[System.Drawing.Point[]]@((New-Object System.Drawing.Point 160,170),(New-Object System.Drawing.Point 176,187),(New-Object System.Drawing.Point 204,153)))
$g.Dispose()
$hIcon=$bmp.GetHicon(); $icon=[System.Drawing.Icon]::FromHandle($hIcon); $fs=[System.IO.File]::Open('tubecontrol.ico',[System.IO.FileMode]::Create); $icon.Save($fs); $fs.Close(); $icon.Dispose(); $bmp.Dispose()
