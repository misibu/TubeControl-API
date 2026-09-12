package ru.tubecontrol.scanner;

import android.content.SharedPreferences;
import android.os.Build;
import android.os.Bundle;
import android.view.inputmethod.EditorInfo;
import android.widget.EditText;
import android.widget.TextView;

import androidx.activity.result.ActivityResultLauncher;
import androidx.appcompat.app.AlertDialog;
import androidx.appcompat.app.AppCompatActivity;
import androidx.security.crypto.EncryptedSharedPreferences;
import androidx.security.crypto.MasterKeys;

import com.google.android.material.button.MaterialButton;
import com.google.zxing.BarcodeFormat;
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
    private static final String PREF_TOKEN = "device_token_v2";
    private static final String PREF_SERVER = "server_url_v2";
    private static final String PREF_DEVICE_ID = "device_id_v2";
    private static final String PREF_DEVICE_NAME = "device_name_v2";
    private static final String SETUP_TYPE = "tubecontrol-enroll-v2";
    private static final MediaType JSON = MediaType.get("application/json; charset=utf-8");

    private final ExecutorService executor = Executors.newSingleThreadExecutor();
    private final OkHttpClient http = new OkHttpClient.Builder()
            .connectTimeout(10, TimeUnit.SECONDS)
            .readTimeout(15, TimeUnit.SECONDS)
            .writeTimeout(15, TimeUnit.SECONDS)
            .callTimeout(20, TimeUnit.SECONDS)
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
                    "tubecontrol_secure_v2",
                    masterKeyAlias,
                    this,
                    EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                    EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
            );
        } catch (Exception ignored) {
            return getSharedPreferences("tubecontrol_private_v2", MODE_PRIVATE);
        }
    }

    private String pref(String key, String fallback) {
        if (prefs == null) return fallback;
        String value = prefs.getString(key, fallback);
        return value == null ? fallback : value.trim();
    }

    private String getToken() { return pref(PREF_TOKEN, ""); }
    private String getBaseUrl() {
        String server = pref(PREF_SERVER, DEFAULT_BASE_URL);
        if (server.isEmpty()) server = DEFAULT_BASE_URL;
        while (server.endsWith("/")) server = server.substring(0, server.length() - 1);
        return server;
    }
    private boolean isConfigured() { return !getToken().isEmpty(); }

    private void refreshUi() {
        String server = getBaseUrl().replace("https://", "").replace("http://", "");
        String name = pref(PREF_DEVICE_NAME, "");
        serverText.setText(name.isEmpty() ? "Сервер: " + server : "Устройство: " + name + "\nСервер: " + server);
        boolean ready = isConfigured();
        scanButton.setEnabled(ready);
        manualButton.setEnabled(ready);
        if (ready) {
            showNeutral("Готов к сканированию");
        } else {
            showWarning("Устройство не подключено\nОтсканируйте QR с Windows");
        }
    }

    private void showFirstSetupDialog() {
        AlertDialog dialog = new AlertDialog.Builder(this)
                .setTitle("Подключение TubeControl")
                .setMessage("На компьютере нажмите «Подключить телефон», затем отсканируйте показанный QR. Токен вводить не нужно.")
                .setPositiveButton("Сканировать QR", (d, which) -> startSetupScanner())
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
        String[] items = {"Переподключить по QR", "Сбросить подключение"};
        new AlertDialog.Builder(this)
                .setTitle("Настройки устройства")
                .setItems(items, (dialog, which) -> {
                    if (which == 0) startSetupScanner();
                    if (which == 1) confirmReset();
                })
                .setNegativeButton("Закрыть", null)
                .show();
    }

    private void startSetupScanner() {
        ScanOptions options = new ScanOptions();
        options.setDesiredBarcodeFormats(Collections.singletonList(BarcodeFormat.QR_CODE.toString()));
        options.setPrompt("Наведите камеру на QR подключения TubeControl");
        options.setBeepEnabled(true);
        options.setOrientationLocked(true);
        options.setCaptureActivity(PortraitCaptureActivity.class);
        setupLauncher.launch(options);
    }

    private void onSetupScanResult(ScanIntentResult result) {
        if (result == null || result.getContents() == null) {
            if (!isConfigured()) {
                showWarning("Подключение не выполнено");
                showFirstSetupDialog();
            }
            return;
        }
        try {
            JSONObject json = new JSONObject(result.getContents().trim());
            if (!SETUP_TYPE.equals(json.optString("type", ""))) {
                throw new IllegalArgumentException("wrong type");
            }
            String server = json.optString("server", "").trim();
            String code = json.optString("code", "").trim().toUpperCase(Locale.ROOT);
            while (server.endsWith("/")) server = server.substring(0, server.length() - 1);
            if (!server.startsWith("https://") || code.length() < 6) {
                throw new IllegalArgumentException("bad payload");
            }
            enrollDevice(server, code);
        } catch (Exception e) {
            showError("Неверный QR подключения\nИспользуйте QR из TubeControl Windows");
            if (!isConfigured()) showFirstSetupDialog();
        }
    }

    private void enrollDevice(String server, String code) {
        setBusy(true);
        showNeutral("Подключаем устройство...");
        executor.execute(() -> {
            try {
                JSONObject payload = new JSONObject();
                payload.put("code", code);
                payload.put("device_name", buildDeviceName());
                Request request = new Request.Builder()
                        .url(server + "/api/v2/enroll")
                        .header("Accept", "application/json")
                        .post(RequestBody.create(payload.toString(), JSON))
                        .build();
                try (Response response = http.newCall(request).execute()) {
                    String body = response.body() == null ? "" : response.body().string();
                    if (response.code() != 200) {
                        runOnUiThread(() -> {
                            setBusy(false);
                            showError("QR недействителен или истёк\nСоздайте новый QR в Windows");
                            if (!isConfigured()) showFirstSetupDialog();
                        });
                        return;
                    }
                    JSONObject json = new JSONObject(body);
                    String token = json.optString("device_token", "").trim();
                    JSONObject device = json.optJSONObject("device");
                    String id = device == null ? "" : device.optString("id", "");
                    String name = device == null ? buildDeviceName() : device.optString("name", buildDeviceName());
                    if (token.isEmpty()) throw new IllegalStateException("empty token");
                    prefs.edit()
                            .putString(PREF_SERVER, server)
                            .putString(PREF_TOKEN, token)
                            .putString(PREF_DEVICE_ID, id)
                            .putString(PREF_DEVICE_NAME, name)
                            .apply();
                    runOnUiThread(() -> {
                        setBusy(false);
                        refreshUi();
                        showSuccess("Устройство подключено\nМожно сканировать THU");
                    });
                }
            } catch (Exception e) {
                runOnUiThread(() -> {
                    setBusy(false);
                    showError("Не удалось подключиться к серверу\n" + safeMessage(e));
                    if (!isConfigured()) showFirstSetupDialog();
                });
            }
        });
    }

    private String buildDeviceName() {
        String manufacturer = Build.MANUFACTURER == null ? "Android" : Build.MANUFACTURER.trim();
        String model = Build.MODEL == null ? "Scanner" : Build.MODEL.trim();
        return (manufacturer + " " + model).trim();
    }

    private void confirmReset() {
        new AlertDialog.Builder(this)
                .setTitle("Сбросить подключение?")
                .setMessage("После сброса потребуется новый QR с компьютера.")
                .setPositiveButton("Сбросить", (dialog, which) -> {
                    prefs.edit().clear().apply();
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
                        .header("Authorization", "Bearer " + token)
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
                    } catch (Exception ignored) {}
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
            showError("Устройство отключено или авторизация истекла\nПодключите его заново через QR");
            return;
        }
        showError("THU: " + thu + "\nОшибка сервера (HTTP " + code + ")");
    }

    private String safeMessage(Exception e) {
        String message = e.getMessage();
        if (message == null || message.trim().isEmpty()) return e.getClass().getSimpleName();
        if (message.length() > 100) return message.substring(0, 100);
        return message;
    }

    private void setBusy(boolean busy) {
        scanButton.setEnabled(!busy && isConfigured());
        manualButton.setEnabled(!busy && isConfigured());
        settingsButton.setEnabled(!busy);
        thuInput.setEnabled(!busy && isConfigured());
    }

    private void showNeutral(String text) { setStatus(text, R.color.status_neutral_bg, R.color.status_neutral_text); }
    private void showSuccess(String text) { setStatus(text, R.color.status_green_bg, R.color.status_green_text); }
    private void showWarning(String text) { setStatus(text, R.color.status_yellow_bg, R.color.status_yellow_text); }
    private void showError(String text) { setStatus(text, R.color.status_red_bg, R.color.status_red_text); }

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
}
