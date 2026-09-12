package ru.tubecontrol.scanner;

import android.content.SharedPreferences;
import android.graphics.Bitmap;
import android.os.Bundle;
import android.text.InputType;
import android.view.inputmethod.EditorInfo;
import android.widget.EditText;
import android.widget.ImageView;
import android.widget.TextView;

import androidx.activity.result.ActivityResultLauncher;
import androidx.appcompat.app.AlertDialog;
import androidx.appcompat.app.AppCompatActivity;
import androidx.security.crypto.EncryptedSharedPreferences;
import androidx.security.crypto.MasterKeys;

import com.google.android.material.button.MaterialButton;
import com.google.zxing.BarcodeFormat;
import com.journeyapps.barcodescanner.BarcodeEncoder;
import com.journeyapps.barcodescanner.ScanContract;
import com.journeyapps.barcodescanner.ScanIntentResult;
import com.journeyapps.barcodescanner.ScanOptions;

import org.json.JSONObject;

import java.util.Collections;
import java.util.Locale;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;

import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;

public class MainActivity extends AppCompatActivity {
    private static final String DEFAULT_BASE_URL = "https://tubecontrol-api-msk-misibun.amvera.io";
    private static final String PREF_TOKEN = "scanner_token";
    private static final String PREF_SERVER = "server_url";
    private static final String SETUP_TYPE = "tubecontrol-setup-v1";
    private static final String SETUP_PREFIX = "TCSETUP1|";
    private static final MediaType JSON = MediaType.get("application/json; charset=utf-8");

    private final ExecutorService executor = Executors.newSingleThreadExecutor();
    private final OkHttpClient http = new OkHttpClient.Builder()
            .connectTimeout(10, TimeUnit.SECONDS)
            .readTimeout(12, TimeUnit.SECONDS)
            .writeTimeout(12, TimeUnit.SECONDS)
            .callTimeout(15, TimeUnit.SECONDS)
            .build();

    private MaterialButton scanButton;
    private MaterialButton manualButton;
    private MaterialButton settingsButton;
    private EditText thuInput;
    private TextView statusBox;
    private TextView serverText;
    private SharedPreferences prefs;

    private final ActivityResultLauncher<ScanOptions> barcodeLauncher =
            registerForActivityResult(new ScanContract(), this::onThuScanResult);

    private final ActivityResultLauncher<ScanOptions> setupLauncher =
            registerForActivityResult(new ScanContract(), this::onSetupScanResult);

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        scanButton = findViewById(R.id.scanButton);
        manualButton = findViewById(R.id.manualButton);
        settingsButton = findViewById(R.id.settingsButton);
        thuInput = findViewById(R.id.thuInput);
        statusBox = findViewById(R.id.statusBox);
        serverText = findViewById(R.id.serverText);
        prefs = createSecurePrefs();

        scanButton.setOnClickListener(v -> startThuScanner());
        manualButton.setOnClickListener(v -> submitManual());
        settingsButton.setOnClickListener(v -> showDeviceSettings());
        thuInput.setOnEditorActionListener((v, actionId, event) -> {
            if (actionId == EditorInfo.IME_ACTION_DONE) {
                submitManual();
                return true;
            }
            return false;
        });

        refreshUi();
        if (!isConfigured()) {
            showFirstSetupDialog();
        }
    }

    private SharedPreferences createSecurePrefs() {
        try {
            String masterKeyAlias = MasterKeys.getOrCreate(MasterKeys.AES256_GCM_SPEC);
            return EncryptedSharedPreferences.create(
                    "tubecontrol_secure",
                    masterKeyAlias,
                    this,
                    EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                    EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
            );
        } catch (Exception ignored) {
            return getSharedPreferences("tubecontrol_private", MODE_PRIVATE);
        }
    }

    private String getToken() {
        if (prefs == null) return "";
        String token = prefs.getString(PREF_TOKEN, "");
        return token == null ? "" : token.trim();
    }

    private String getBaseUrl() {
        if (prefs == null) return DEFAULT_BASE_URL;
        String server = prefs.getString(PREF_SERVER, DEFAULT_BASE_URL);
        if (server == null || server.trim().isEmpty()) return DEFAULT_BASE_URL;
        return normalizeServer(server);
    }

    private boolean isConfigured() {
        return !getToken().isEmpty();
    }

    private String normalizeServer(String value) {
        String server = value == null ? "" : value.trim();
        while (server.endsWith("/")) {
            server = server.substring(0, server.length() - 1);
        }
        return server;
    }

    private void refreshUi() {
        String server = getBaseUrl().replace("https://", "").replace("http://", "");
        serverText.setText("Сервер: " + server);
        if (isConfigured()) {
            scanButton.setEnabled(true);
            manualButton.setEnabled(true);
            showNeutral("Готов к сканированию");
        } else {
            scanButton.setEnabled(false);
            manualButton.setEnabled(false);
            showWarning("Требуется настройка устройства\nОтсканируйте QR настройки");
        }
    }

    private void showFirstSetupDialog() {
        AlertDialog dialog = new AlertDialog.Builder(this)
                .setTitle("Настройка устройства")
                .setMessage("Для работы сканера отсканируйте QR-код настройки TubeControl. Вводить длинный токен вручную не нужно.")
                .setPositiveButton("Сканировать QR", (d, which) -> startSetupScanner())
                .setNegativeButton("Ввести вручную", (d, which) -> showManualTokenDialog())
                .create();
        dialog.setCancelable(false);
        dialog.setCanceledOnTouchOutside(false);
        dialog.show();
    }

    private void showDeviceSettings() {
        if (!isConfigured()) {
            showFirstSetupDialog();
            return;
        }

        String[] items = {
                "Сканировать новый QR настройки",
                "Показать QR для другого устройства",
                "Ввести токен вручную",
                "Сбросить настройку"
        };

        new AlertDialog.Builder(this)
                .setTitle("Настройки устройства")
                .setItems(items, (dialog, which) -> {
                    if (which == 0) startSetupScanner();
                    if (which == 1) showConfigQr();
                    if (which == 2) showManualTokenDialog();
                    if (which == 3) confirmReset();
                })
                .setNegativeButton("Закрыть", null)
                .show();
    }

    private void startSetupScanner() {
        ScanOptions options = new ScanOptions();
        options.setDesiredBarcodeFormats(Collections.singletonList(BarcodeFormat.QR_CODE.toString()));
        options.setPrompt("Наведите камеру на QR настройки TubeControl");
        options.setBeepEnabled(true);
        options.setOrientationLocked(true);
        options.setCaptureActivity(PortraitCaptureActivity.class);
        setupLauncher.launch(options);
    }

    private void onSetupScanResult(ScanIntentResult result) {
        if (result == null || result.getContents() == null) {
            if (!isConfigured()) showWarning("Настройка не выполнена\nОтсканируйте QR настройки");
            return;
        }

        try {
            SetupConfig config = parseSetupPayload(result.getContents());
            saveConfiguration(config.server, config.token);
            refreshUi();
            showSuccess("Устройство настроено\nМожно сканировать THU");
        } catch (Exception e) {
            showError("Неверный QR настройки\nИспользуйте QR TubeControl");
            if (!isConfigured()) {
                scanButton.setEnabled(false);
                manualButton.setEnabled(false);
            }
        }
    }

    private SetupConfig parseSetupPayload(String raw) throws Exception {
        String value = raw == null ? "" : raw.trim();
        String server;
        String token;

        if (value.startsWith("{")) {
            JSONObject json = new JSONObject(value);
            if (!SETUP_TYPE.equals(json.optString("type", ""))) {
                throw new IllegalArgumentException("wrong type");
            }
            server = json.optString("server", "");
            token = json.optString("token", "");
        } else if (value.startsWith(SETUP_PREFIX)) {
            String[] parts = value.split("\\|", 3);
            if (parts.length != 3) throw new IllegalArgumentException("wrong format");
            server = parts[1];
            token = parts[2];
        } else {
            throw new IllegalArgumentException("unknown setup QR");
        }

        server = normalizeServer(server);
        token = token.trim();
        if (!server.startsWith("https://")) throw new IllegalArgumentException("https required");
        if (token.length() < 8) throw new IllegalArgumentException("token too short");
        return new SetupConfig(server, token);
    }

    private void saveConfiguration(String server, String token) {
        prefs.edit()
                .putString(PREF_SERVER, normalizeServer(server))
                .putString(PREF_TOKEN, token.trim())
                .apply();
    }

    private void showManualTokenDialog() {
        final EditText input = new EditText(this);
        input.setSingleLine(true);
        input.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD);
        input.setHint("SCANNER_TOKEN");
        input.setPadding(32, 16, 32, 16);

        AlertDialog dialog = new AlertDialog.Builder(this)
                .setTitle("Ручная настройка")
                .setMessage("Используйте этот способ только для первого устройства. После настройки оно сможет показать QR для остальных телефонов.")
                .setView(input)
                .setPositiveButton("Сохранить", null)
                .setNegativeButton("Отмена", null)
                .create();

        dialog.setOnShowListener(ignored -> dialog.getButton(AlertDialog.BUTTON_POSITIVE).setOnClickListener(v -> {
            String value = input.getText().toString().trim();
            if (value.length() < 8) {
                input.setError("Проверьте токен");
                return;
            }
            saveConfiguration(getBaseUrl(), value);
            dialog.dismiss();
            refreshUi();
            showSuccess("Устройство настроено\nМожно сканировать THU");
        }));
        dialog.show();
    }

    private void showConfigQr() {
        if (!isConfigured()) {
            showFirstSetupDialog();
            return;
        }

        try {
            JSONObject config = new JSONObject();
            config.put("type", SETUP_TYPE);
            config.put("server", getBaseUrl());
            config.put("token", getToken());

            BarcodeEncoder encoder = new BarcodeEncoder();
            Bitmap bitmap = encoder.encodeBitmap(config.toString(), BarcodeFormat.QR_CODE, 760, 760);
            ImageView image = new ImageView(this);
            image.setImageBitmap(bitmap);
            image.setAdjustViewBounds(true);
            image.setPadding(20, 20, 20, 20);

            new AlertDialog.Builder(this)
                    .setTitle("QR настройки TubeControl")
                    .setMessage("Отсканируйте этот QR на другом телефоне. QR содержит ключ доступа — показывайте его только доверенным сотрудникам.")
                    .setView(image)
                    .setPositiveButton("Закрыть", null)
                    .show();
        } catch (Exception e) {
            showError("Не удалось создать QR настройки");
        }
    }

    private void confirmReset() {
        new AlertDialog.Builder(this)
                .setTitle("Сбросить настройку?")
                .setMessage("После сброса потребуется снова отсканировать QR настройки.")
                .setPositiveButton("Сбросить", (dialog, which) -> {
                    prefs.edit().remove(PREF_TOKEN).remove(PREF_SERVER).apply();
                    refreshUi();
                    showFirstSetupDialog();
                })
                .setNegativeButton("Отмена", null)
                .show();
    }

    private void startThuScanner() {
        if (!isConfigured()) {
            showFirstSetupDialog();
            return;
        }

        ScanOptions options = new ScanOptions();
        options.setDesiredBarcodeFormats(Collections.singletonList(BarcodeFormat.CODE_128.toString()));
        options.setPrompt("Наведите камеру на штрихкод THU");
        options.setBeepEnabled(true);
        options.setOrientationLocked(true);
        options.setCaptureActivity(PortraitCaptureActivity.class);
        barcodeLauncher.launch(options);
    }

    private void onThuScanResult(ScanIntentResult result) {
        if (result == null || result.getContents() == null) {
            showNeutral("Сканирование отменено");
            return;
        }
        String thu = normalizeThu(result.getContents());
        if (thu.isEmpty()) {
            showError("Штрихкод пустой");
            return;
        }
        sendReturn(thu);
    }

    private void submitManual() {
        if (!isConfigured()) {
            showFirstSetupDialog();
            return;
        }
        String thu = normalizeThu(thuInput.getText().toString());
        if (thu.isEmpty()) {
            thuInput.setError("Введите THU");
            return;
        }
        sendReturn(thu);
    }

    private String normalizeThu(String value) {
        return value == null ? "" : value.trim().toUpperCase(Locale.ROOT);
    }

    private void sendReturn(String thu) {
        String token = getToken();
        if (token.isEmpty()) {
            showFirstSetupDialog();
            return;
        }

        setBusy(true);
        showNeutral("THU: " + thu + "\nПроверяем...");

        executor.execute(() -> {
            try {
                JSONObject payload = new JSONObject();
                payload.put("thu", thu);

                Request request = new Request.Builder()
                        .url(getBaseUrl() + "/api/v1/returns")
                        .header("X-API-Key", token)
                        .header("Accept", "application/json")
                        .post(RequestBody.create(payload.toString(), JSON))
                        .build();

                try (Response response = http.newCall(request).execute()) {
                    int code = response.code();
                    String body = response.body() == null ? "" : response.body().string();
                    String status = "";
                    String returnedAt = "";
                    try {
                        JSONObject json = new JSONObject(body);
                        status = json.optString("status", "");
                        returnedAt = json.optString("returned_at", "");
                    } catch (Exception ignored) {
                    }

                    final String finalStatus = status;
                    final String finalReturnedAt = returnedAt;
                    runOnUiThread(() -> handleApiResponse(thu, code, finalStatus, finalReturnedAt));
                }
            } catch (Exception e) {
                runOnUiThread(() -> {
                    setBusy(false);
                    showError("THU: " + thu + "\nНет связи с сервером\n" + safeMessage(e));
                });
            }
        });
    }

    private void handleApiResponse(String thu, int code, String status, String returnedAt) {
        setBusy(false);
        thuInput.setText("");

        if (code == 200 && "returned".equals(status)) {
            showSuccess("THU: " + thu + "\nВозврат зарегистрирован");
            return;
        }

        if (code == 200 && "already_returned".equals(status)) {
            String suffix = returnedAt.isEmpty() ? "" : "\n" + returnedAt;
            showWarning("THU: " + thu + "\nУже возвращён" + suffix);
            return;
        }

        if (code == 404 || "not_found".equals(status)) {
            showError("THU: " + thu + "\nTHU не найден");
            return;
        }

        if (code == 401 || code == 403) {
            showError("Ошибка авторизации\nОткройте «Настройки устройства» и отсканируйте новый QR");
            return;
        }

        showError("THU: " + thu + "\nОшибка сервера (HTTP " + code + ")");
    }

    private String safeMessage(Exception e) {
        String message = e.getMessage();
        if (message == null || message.trim().isEmpty()) {
            return e.getClass().getSimpleName();
        }
        if (message.length() > 100) {
            return message.substring(0, 100);
        }
        return message;
    }

    private void setBusy(boolean busy) {
        scanButton.setEnabled(!busy && isConfigured());
        manualButton.setEnabled(!busy && isConfigured());
        settingsButton.setEnabled(!busy);
        thuInput.setEnabled(!busy);
    }

    private void showNeutral(String text) {
        setStatus(text, R.color.status_neutral_bg, R.color.status_neutral_text);
    }

    private void showSuccess(String text) {
        setStatus(text, R.color.status_green_bg, R.color.status_green_text);
    }

    private void showWarning(String text) {
        setStatus(text, R.color.status_yellow_bg, R.color.status_yellow_text);
    }

    private void showError(String text) {
        setStatus(text, R.color.status_red_bg, R.color.status_red_text);
    }

    private void setStatus(String text, int backgroundColor, int textColor) {
        statusBox.setText(text);
        statusBox.setBackgroundColor(getColor(backgroundColor));
        statusBox.setTextColor(getColor(textColor));
    }

    @Override
    protected void onDestroy() {
        executor.shutdownNow();
        super.onDestroy();
    }

    private static class SetupConfig {
        final String server;
        final String token;

        SetupConfig(String server, String token) {
            this.server = server;
            this.token = token;
        }
    }
}
