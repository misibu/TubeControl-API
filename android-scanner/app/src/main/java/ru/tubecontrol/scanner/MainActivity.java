package ru.tubecontrol.scanner;

import android.content.SharedPreferences;
import android.os.Bundle;
import android.text.InputType;
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
    private static final String BASE_URL = "https://tubecontrol-api-msk-misibun.amvera.io";
    private static final String PREF_TOKEN = "scanner_token";
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
    private MaterialButton tokenButton;
    private EditText thuInput;
    private TextView statusBox;
    private SharedPreferences prefs;

    private final ActivityResultLauncher<ScanOptions> barcodeLauncher =
            registerForActivityResult(new ScanContract(), this::onScanResult);

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        scanButton = findViewById(R.id.scanButton);
        manualButton = findViewById(R.id.manualButton);
        tokenButton = findViewById(R.id.tokenButton);
        thuInput = findViewById(R.id.thuInput);
        statusBox = findViewById(R.id.statusBox);
        prefs = createSecurePrefs();

        scanButton.setOnClickListener(v -> startScanner());
        manualButton.setOnClickListener(v -> submitManual());
        tokenButton.setOnClickListener(v -> showTokenDialog(true));
        thuInput.setOnEditorActionListener((v, actionId, event) -> {
            if (actionId == EditorInfo.IME_ACTION_DONE) {
                submitManual();
                return true;
            }
            return false;
        });

        if (getToken().isEmpty()) {
            showTokenDialog(false);
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
        if (prefs == null) {
            return "";
        }
        String token = prefs.getString(PREF_TOKEN, "");
        return token == null ? "" : token.trim();
    }

    private void showTokenDialog(boolean allowCancel) {
        final EditText input = new EditText(this);
        input.setSingleLine(true);
        input.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD);
        input.setHint("SCANNER_TOKEN");
        input.setPadding(32, 16, 32, 16);

        AlertDialog.Builder builder = new AlertDialog.Builder(this)
                .setTitle("Токен сканера")
                .setMessage("Введите SCANNER_TOKEN из настроек TubeControl API. Токен хранится только на этом телефоне.")
                .setView(input)
                .setPositiveButton("Сохранить", null);

        if (allowCancel && !getToken().isEmpty()) {
            builder.setNegativeButton("Отмена", null);
        }

        AlertDialog dialog = builder.create();
        boolean canCancel = allowCancel && !getToken().isEmpty();
        dialog.setCancelable(canCancel);
        dialog.setCanceledOnTouchOutside(canCancel);
        dialog.setOnShowListener(ignored -> dialog.getButton(AlertDialog.BUTTON_POSITIVE).setOnClickListener(v -> {
            String value = input.getText().toString().trim();
            if (value.isEmpty()) {
                input.setError("Введите токен");
                return;
            }
            prefs.edit().putString(PREF_TOKEN, value).apply();
            dialog.dismiss();
            showNeutral("Токен сохранён\nМожно сканировать THU");
        }));
        dialog.show();
    }

    private void startScanner() {
        if (getToken().isEmpty()) {
            showTokenDialog(false);
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

    private void onScanResult(ScanIntentResult result) {
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
            showTokenDialog(false);
            return;
        }

        setBusy(true);
        showNeutral("THU: " + thu + "\nПроверяем...");

        executor.execute(() -> {
            try {
                JSONObject payload = new JSONObject();
                payload.put("thu", thu);

                Request request = new Request.Builder()
                        .url(BASE_URL + "/api/v1/returns")
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
            showError("Ошибка авторизации\nПроверьте SCANNER_TOKEN");
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
        scanButton.setEnabled(!busy);
        manualButton.setEnabled(!busy);
        tokenButton.setEnabled(!busy);
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
}
