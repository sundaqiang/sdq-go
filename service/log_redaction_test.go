package service

import (
	"bytes"
	stdjson "encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newLogTestLogger(output *bytes.Buffer, fields ...string) *zap.Logger {
	encoder := newRedactingEncoder(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), fields)
	return zap.New(zapcore.NewCore(encoder, zapcore.AddSync(output), zap.DebugLevel))
}

func assertLogOmits(t *testing.T, output string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(output, secret) {
			t.Errorf("log contains a sensitive test value %q", secret)
		}
	}
}

type redactionTestObject struct{}

func (redactionTestObject) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddString("API-Key", "object-private-key")
	encoder.AddString("name", "visible-object")
	return nil
}

func TestLogRedactionAcrossEncodedFields(t *testing.T) {
	var output bytes.Buffer
	log := newLogTestLogger(&output, "password", "phone", "id_card", "authorization", "api-key", "access-token")
	payload := map[string]any{
		"Password": "nested-password",
		"name":     "visible-name",
		"items": []any{
			map[string]any{"ID-Card": "private-card", "count": 7},
		},
	}
	log.With(zap.String("Authorization", "with-credential")).Info("event",
		zap.Any("body", payload),
		zap.Object("object", redactionTestObject{}),
		zap.String("PHONE", "private-phone"),
		zap.String("json", `{"access_token":"json-token","name":"visible-json"}`),
		zap.String("API-Key", "object-private-key"),
		zap.Int64("large_id", 9007199254740993),
	)
	assertLogOmits(t, output.String(), "nested-password", "private-card", "object-private-key", "private-phone", "with-credential", "json-token")
	for _, expected := range []string{"visible-name", "visible-object", "visible-json", "9007199254740993", redactedLogValue} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("log lost non-sensitive content %q", expected)
		}
	}
	if payload["Password"] != "nested-password" {
		t.Fatal("redaction mutated application data")
	}
	if !stdjson.Valid(bytes.TrimSpace(output.Bytes())) {
		t.Fatal("redaction produced invalid JSON")
	}
}

func TestLogRedactionRequiresExplicitFields(t *testing.T) {
	var output bytes.Buffer
	log := newLogTestLogger(&output)
	child := log.With(zap.Namespace("user"), zap.String("mobile", "private-mobile"))
	child.Info("child", zap.String("password", "child-password"), zap.String("name", "visible-child"))
	log.Info("parent", zap.String("token", "parent-token"))
	for _, value := range []string{"private-mobile", "child-password", "parent-token"} {
		if !strings.Contains(output.String(), value) {
			t.Errorf("empty sensitive-fields unexpectedly hid %q", value)
		}
	}
	output.Reset()
	log = newLogTestLogger(&output, "mobile", "password", "token")
	child = log.With(zap.Namespace("user"), zap.String("mobile", "private-mobile"))
	child.Info("child", zap.String("password", "child-password"), zap.String("name", "visible-child"))
	log.Info("parent", zap.String("token", "parent-token"))
	assertLogOmits(t, output.String(), "private-mobile", "child-password", "parent-token")
	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("got %d log entries, want 2", len(lines))
	}
	var parent map[string]any
	if err := stdjson.Unmarshal(lines[1], &parent); err != nil {
		t.Fatal(err)
	}
	if _, leakedNamespace := parent["user"]; leakedNamespace {
		t.Fatal("child namespace leaked into parent logger")
	}
	if parent["token"] != redactedLogValue {
		t.Fatal("configured sensitive field was not redacted")
	}
}

func TestLogRedactionConfig(t *testing.T) {
	const data = `[log]
sensitive-fields = ["phone", "id_card"]
[[database.redis]]
debug = false
[[database.redis]]
debug = true
[[database.redis]]
`
	path := filepath.Join(t.TempDir(), "logging.toml")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	loader := koanf.New(".")
	if err := loader.Load(file.Provider(path), toml.Parser()); err != nil {
		t.Fatal(err)
	}
	var parsed Config
	if err := loader.UnmarshalWithConf("", &parsed, koanf.UnmarshalConf{Tag: "toml"}); err != nil {
		t.Fatal(err)
	}
	if parsed.Database.Redis[0].Debug || !parsed.Database.Redis[1].Debug || parsed.Database.Redis[2].Debug {
		t.Fatal("redis debug switches did not follow their configuration and defaults")
	}
	oldConfig := config
	config = parsed
	t.Cleanup(func() { config = oldConfig })
	var output bytes.Buffer
	log := zap.New(zapcore.NewCore(getEncoder(), zapcore.AddSync(&output), zap.DebugLevel))
	log.Info("configured", zap.String("PHONE", "configured-phone"), zap.String("password", "default-password"))
	if strings.Contains(output.String(), "configured-phone") {
		t.Fatal("configured sensitive field was not redacted")
	}
	if !strings.Contains(output.String(), "default-password") {
		t.Fatal("field without an explicit rule was unexpectedly redacted")
	}
}
