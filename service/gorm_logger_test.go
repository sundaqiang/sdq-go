package service

import (
	"bytes"
	"strings"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"moul.io/zapgorm2"
)

func TestGormUsesNativeDebugSwitch(t *testing.T) {
	oldConfig, oldLog, oldDB := config, ZapLog, Db
	t.Cleanup(func() { config, ZapLog, Db = oldConfig, oldLog, oldDB })
	var output bytes.Buffer
	config = Config{Server: &Server{Trace: "test_trace"}}
	ZapLog = newLogTestLogger(&output)
	InitGORM(&Gorm{Type: "sqlite", Name: ":memory:"})
	sqlDB, err := Db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	output.Reset()

	if err := Db.Exec("CREATE TABLE users (password TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("ordinary GORM operations printed SQL: %s", output.String())
	}

	const secret = "private-sql-value"
	output.Reset()
	if err := Db.Debug().Exec("INSERT INTO users (password) VALUES (?)", secret).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "INSERT INTO users") || !strings.Contains(output.String(), secret) {
		t.Fatalf("GORM Debug did not print the SQL and its value: %s", output.String())
	}

	output.Reset()
	var value string
	if err := Db.Raw("SELECT password FROM users").Scan(&value).Error; err != nil {
		t.Fatal(err)
	}
	if value != secret || output.Len() != 0 {
		t.Fatal("Debug leaked to the original session or changed application data")
	}
	if err := Db.Exec("INSERT INTO missing_table (password) VALUES (?)", secret).Error; err == nil {
		t.Fatal("expected database error to reach caller")
	}
	if !strings.Contains(output.String(), `"level":"error"`) || !strings.Contains(output.String(), secret) {
		t.Fatalf("disabling debug suppressed or redacted error logging: %s", output.String())
	}

	output.Reset()
	if err := Db.Session(&gorm.Session{Logger: Db.Logger.LogMode(logger.Silent)}).Exec("INSERT INTO missing_table (password) VALUES (?)", secret).Error; err == nil {
		t.Fatal("expected database error even with a silent logger")
	}
	if output.Len() != 0 {
		t.Fatal("GORM's native Silent mode was not respected")
	}

	redactingLog := zapgorm2.New(newLogTestLogger(&output, "sql"))
	redactingDB := Db.Session(&gorm.Session{Logger: redactingLog}).Debug()
	if err := redactingDB.Exec("INSERT INTO users (password) VALUES (?)", secret).Error; err != nil {
		t.Fatal(err)
	}
	if err := redactingDB.Raw("SELECT password FROM users WHERE password = ?", secret).Scan(&value).Error; err != nil {
		t.Fatal(err)
	}
	assertLogOmits(t, output.String(), secret)
	if value != secret || !strings.Contains(output.String(), redactedLogValue) {
		t.Fatal("SQL log field redaction changed the query or omitted the log entry")
	}
}
