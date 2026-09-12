from pathlib import Path

p = Path("windows-v3/MainForm.cs")
s = p.read_text(encoding="utf-8")

# v3.8.2 — only explicit reactivation is allowed to open the ActivationForm.
# A 401 from Refresh / Android pairing / device management must never pop the
# activation window automatically. The app stays open and points the user to
# Settings -> Repeat Windows activation.

old_safe = '''        catch (TubeControlApiException ex) when (ex.StatusCode == 401)
        {
            await ReauthorizeWindows();
        }
        catch (Exception ex) { ShowError(ex); }
        finally { if (disable is not null) disable.Enabled = true; }'''

new_safe = '''        catch (TubeControlApiException ex) when (ex.StatusCode == 401)
        {
            _state.Text = "Требуется активация Windows · Настройки → Повторная активация Windows";
            _state.ForeColor = UiPalette.Yellow;
        }
        catch (Exception ex) { ShowError(ex); }
        finally { if (disable is not null) disable.Enabled = true; }'''

if old_safe not in s:
    raise SystemExit("v3.8 Safe 401 handler not found")

s = s.replace(old_safe, new_safe, 1)
p.write_text(s, encoding="utf-8", newline="\n")
