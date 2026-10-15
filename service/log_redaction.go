package service

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"strings"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

const redactedLogValue = "[REDACTED]"

var redactedLogBuffers = buffer.NewPool()

// Redact after encoding so reflected values, object marshalers and With fields
// follow the same rules without modifying application data.
type redactingEncoder struct {
	zapcore.Encoder
	fields map[string]struct{}
}

func newRedactingEncoder(encoder zapcore.Encoder, sensitiveFields []string) zapcore.Encoder {
	fields := make(map[string]struct{})
	for _, field := range sensitiveFields {
		if key := normalizeLogField(field); key != "" {
			fields[key] = struct{}{}
		}
	}
	if len(fields) == 0 {
		return encoder
	}
	return &redactingEncoder{Encoder: encoder, fields: fields}
}

func normalizeLogField(field string) string {
	return strings.Map(func(r rune) rune {
		if r == '_' || r == '-' {
			return -1
		}
		return r
	}, strings.ToLower(strings.TrimSpace(field)))
}

func (e *redactingEncoder) Clone() zapcore.Encoder {
	return &redactingEncoder{Encoder: e.Encoder.Clone(), fields: e.fields}
}

func (e *redactingEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	encoded, err := e.Encoder.EncodeEntry(entry, fields)
	if err != nil {
		return nil, errors.New("log encoding failed")
	}
	defer encoded.Free()

	var event map[string]any
	decoder := stdjson.NewDecoder(bytes.NewReader(encoded.Bytes()))
	decoder.UseNumber()
	if err := decoder.Decode(&event); err != nil {
		// Never fall back to writing the original, potentially sensitive entry.
		return nil, errors.New("log redaction could not decode entry")
	}
	e.redact(event)
	clean, err := stdjson.Marshal(event)
	if err != nil {
		return nil, errors.New("log redaction could not encode entry")
	}
	result := redactedLogBuffers.Get()
	result.AppendBytes(clean)
	result.AppendString(zapcore.DefaultLineEnding)
	return result, nil
}

func (e *redactingEncoder) redact(value any) any {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if _, sensitive := e.fields[normalizeLogField(key)]; sensitive {
				value[key] = redactedLogValue
			} else {
				value[key] = e.redact(child)
			}
		}
	case []any:
		for i, child := range value {
			value[i] = e.redact(child)
		}
	case string:
		// JSON payloads are often logged as strings instead of structured fields.
		trimmed := strings.TrimSpace(value)
		if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') || !stdjson.Valid([]byte(trimmed)) {
			return value
		}
		var nested any
		decoder := stdjson.NewDecoder(strings.NewReader(trimmed))
		decoder.UseNumber()
		if decoder.Decode(&nested) != nil {
			return redactedLogValue
		}
		clean, err := stdjson.Marshal(e.redact(nested))
		if err != nil {
			return redactedLogValue
		}
		return string(clean)
	}
	return value
}
