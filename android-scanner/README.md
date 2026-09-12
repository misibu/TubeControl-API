# TubeControl Scanner for Android

Installable Android client for registering reusable tube returns.

- Scans Code 128 barcodes with the phone camera.
- Supports manual THU entry.
- Calls `POST https://tubecontrol-api-msk-misibun.amvera.io/api/v1/returns`.
- Requests `SCANNER_TOKEN` on first launch instead of embedding it into the APK.
- Stores the token in Android encrypted preferences when available.
- Shows green `Возврат зарегистрирован`, yellow `Уже возвращён`, or red `THU не найден`.

The debug APK produced by GitHub Actions is installable on Android 8.0 (API 26) and newer.
