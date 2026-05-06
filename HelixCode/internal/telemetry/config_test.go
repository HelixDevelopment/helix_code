package telemetry

import (
	"errors"
	"testing"
	"time"
)

// staticEnv builds a deterministic envLookup from a map. Missing keys return "".
func staticEnv(m map[string]string) func(string) string {
	return func(k string) string {
		if v, ok := m[k]; ok {
			return v
		}
		return ""
	}
}

func TestLoadConfigFromEnv_AllUnset_ReturnsNoop(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Enabled {
		t.Errorf("Enabled = true, want false")
	}
	if cfg.Exporter != ExporterNoop {
		t.Errorf("Exporter = %q, want %q", cfg.Exporter, ExporterNoop)
	}
}

func TestLoadConfigFromEnv_StdoutExporterSet_Enabled(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"HELIXCODE_OTEL_EXPORTER": "stdout",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Enabled {
		t.Errorf("Enabled = false, want true")
	}
	if cfg.Exporter != ExporterStdout {
		t.Errorf("Exporter = %q, want %q", cfg.Exporter, ExporterStdout)
	}
}

func TestLoadConfigFromEnv_OTLPGRPC_ProtocolSet(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "otel:4317",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Enabled {
		t.Errorf("Enabled = false, want true")
	}
	if cfg.Exporter != ExporterOTLPGRPC {
		t.Errorf("Exporter = %q, want %q", cfg.Exporter, ExporterOTLPGRPC)
	}
	if cfg.Endpoint != "otel:4317" {
		t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, "otel:4317")
	}
}

func TestLoadConfigFromEnv_OTLPHTTP_ProtocolSet(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "https://otel:4318",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Enabled {
		t.Errorf("Enabled = false, want true")
	}
	if cfg.Exporter != ExporterOTLPHTTP {
		t.Errorf("Exporter = %q, want %q", cfg.Exporter, ExporterOTLPHTTP)
	}
	if cfg.Endpoint != "https://otel:4318" {
		t.Errorf("Endpoint = %q, want %q", cfg.Endpoint, "https://otel:4318")
	}
}

func TestLoadConfigFromEnv_TracesExporterConsole_Wins(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"OTEL_TRACES_EXPORTER":        "console",
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Exporter != ExporterStdout {
		t.Errorf("Exporter = %q, want %q (console > otlp protocol)", cfg.Exporter, ExporterStdout)
	}
	if !cfg.Enabled {
		t.Errorf("Enabled = false, want true")
	}
}

func TestLoadConfigFromEnv_TracesExporterNone_DisablesTelemetry(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"OTEL_TRACES_EXPORTER":        "none",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "otel:4317",
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Enabled {
		t.Errorf("Enabled = true, want false (explicit none)")
	}
	if cfg.Exporter != ExporterNoop {
		t.Errorf("Exporter = %q, want %q", cfg.Exporter, ExporterNoop)
	}
}

func TestLoadConfigFromEnv_HelixcodeOverride_Wins(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"HELIXCODE_OTEL_EXPORTER": "stdout",
		"OTEL_TRACES_EXPORTER":    "otlp",
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Exporter != ExporterStdout {
		t.Errorf("Exporter = %q, want %q (HelixCode override)", cfg.Exporter, ExporterStdout)
	}
}

func TestLoadConfigFromEnv_UnsupportedProtocol_Errors(t *testing.T) {
	_, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"OTEL_EXPORTER_OTLP_PROTOCOL": "http/json",
	}))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrUnsupportedExporter) {
		t.Errorf("err = %v, want ErrUnsupportedExporter", err)
	}
}

func TestLoadConfigFromEnv_ServiceName_Default(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ServiceName != DefaultServiceName {
		t.Errorf("ServiceName = %q, want %q", cfg.ServiceName, DefaultServiceName)
	}
}

func TestLoadConfigFromEnv_ServiceName_FromEnv(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"OTEL_SERVICE_NAME": "my-service",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ServiceName != "my-service" {
		t.Errorf("ServiceName = %q, want %q", cfg.ServiceName, "my-service")
	}
}

func TestLoadConfigFromEnv_ResourceAttrs_Parsed(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"OTEL_RESOURCE_ATTRIBUTES": "env=prod,team=core",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.ResourceAttrs["env"]; got != "prod" {
		t.Errorf("ResourceAttrs[env] = %q, want %q", got, "prod")
	}
	if got := cfg.ResourceAttrs["team"]; got != "core" {
		t.Errorf("ResourceAttrs[team] = %q, want %q", got, "core")
	}
}

func TestLoadConfigFromEnv_ResourceAttrs_MalformedEntriesSkipped(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"OTEL_RESOURCE_ATTRIBUTES": "key1=val1,malformed,key2=val2",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.ResourceAttrs["key1"]; got != "val1" {
		t.Errorf("ResourceAttrs[key1] = %q, want %q", got, "val1")
	}
	if got := cfg.ResourceAttrs["key2"]; got != "val2" {
		t.Errorf("ResourceAttrs[key2] = %q, want %q", got, "val2")
	}
	if _, exists := cfg.ResourceAttrs["malformed"]; exists {
		t.Errorf("ResourceAttrs[malformed] should not exist")
	}
	if len(cfg.ResourceAttrs) != 2 {
		t.Errorf("len(ResourceAttrs) = %d, want 2", len(cfg.ResourceAttrs))
	}
}

func TestLoadConfigFromEnv_ResourceAttrs_EmptyValuesTolerated(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"OTEL_RESOURCE_ATTRIBUTES": "key1=,key2=val2",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, exists := cfg.ResourceAttrs["key1"]; !exists || got != "" {
		t.Errorf("ResourceAttrs[key1] = %q, exists=%v, want empty existing", got, exists)
	}
	if got := cfg.ResourceAttrs["key2"]; got != "val2" {
		t.Errorf("ResourceAttrs[key2] = %q, want %q", got, "val2")
	}
}

func TestLoadConfigFromEnv_Insecure_True(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		"OTEL_EXPORTER_OTLP_INSECURE": "true",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Insecure {
		t.Errorf("Insecure = false, want true")
	}
}

func TestLoadConfigFromEnv_Insecure_False(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
		"OTEL_EXPORTER_OTLP_INSECURE": "false",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Insecure {
		t.Errorf("Insecure = true, want false")
	}
}

func TestLoadConfigFromEnv_BatchTimeout_FromEnv(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"HELIXCODE_OTEL_EXPORTER": "stdout",
		"OTEL_BSP_SCHEDULE_DELAY": "2000",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BatchTimeout != 2*time.Second {
		t.Errorf("BatchTimeout = %v, want %v", cfg.BatchTimeout, 2*time.Second)
	}
}

func TestLoadConfigFromEnv_BatchTimeout_DefaultWhenUnset(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"HELIXCODE_OTEL_EXPORTER": "stdout",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BatchTimeout != DefaultBatchTimeout {
		t.Errorf("BatchTimeout = %v, want %v", cfg.BatchTimeout, DefaultBatchTimeout)
	}
}

func TestLoadConfigFromEnv_ExportTimeout_FromEnv(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(map[string]string{
		"HELIXCODE_OTEL_EXPORTER": "stdout",
		"OTEL_BSP_EXPORT_TIMEOUT": "10000",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ExportTimeout != 10*time.Second {
		t.Errorf("ExportTimeout = %v, want %v", cfg.ExportTimeout, 10*time.Second)
	}
}

func TestLoadConfigFromEnv_ShutdownTimeout_DefaultOnly(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ShutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, DefaultShutdownTimeout)
	}
}

func TestLoadConfigFromEnv_BlockedAttributeKeys_AlwaysIncludesDefaults(t *testing.T) {
	cfg, err := LoadConfigFromEnv(staticEnv(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.BlockedAttributeKeys) < len(DefaultBlockedAttributeKeys) {
		t.Errorf("len(BlockedAttributeKeys) = %d, want >= %d (CONST-042 floor)",
			len(cfg.BlockedAttributeKeys), len(DefaultBlockedAttributeKeys))
	}
	// Verify every default key is present in the resulting list.
	have := make(map[string]bool)
	for _, k := range cfg.BlockedAttributeKeys {
		have[k] = true
	}
	for _, k := range DefaultBlockedAttributeKeys {
		if !have[k] {
			t.Errorf("BlockedAttributeKeys missing default key %q", k)
		}
	}
}

func TestParseResourceAttrs_EmptyString(t *testing.T) {
	got := parseResourceAttrs("")
	if len(got) != 0 {
		t.Errorf("parseResourceAttrs(\"\") = %v, want empty", got)
	}
}

func TestParseResourceAttrs_TrimsWhitespace(t *testing.T) {
	got := parseResourceAttrs("k1 = v1 ,  k2=v2 ")
	if got["k1"] != "v1" {
		t.Errorf("got[k1] = %q, want %q", got["k1"], "v1")
	}
	if got["k2"] != "v2" {
		t.Errorf("got[k2] = %q, want %q", got["k2"], "v2")
	}
}

func TestParseDurationMillis_Negative_ReturnsDefault(t *testing.T) {
	got := parseDurationMillis("-100", 7*time.Second)
	if got != 7*time.Second {
		t.Errorf("parseDurationMillis(-100) = %v, want %v", got, 7*time.Second)
	}
}

func TestParseDurationMillis_NonNumeric_ReturnsDefault(t *testing.T) {
	got := parseDurationMillis("not-a-number", 7*time.Second)
	if got != 7*time.Second {
		t.Errorf("parseDurationMillis(not-a-number) = %v, want %v", got, 7*time.Second)
	}
}

func TestParseDurationMillis_EmptyReturnsDefault(t *testing.T) {
	got := parseDurationMillis("", 7*time.Second)
	if got != 7*time.Second {
		t.Errorf("parseDurationMillis(\"\") = %v, want %v", got, 7*time.Second)
	}
}

func TestParseDurationMillis_ValidValue(t *testing.T) {
	got := parseDurationMillis("1500", 7*time.Second)
	if got != 1500*time.Millisecond {
		t.Errorf("parseDurationMillis(1500) = %v, want %v", got, 1500*time.Millisecond)
	}
}

func TestParseBool_VariousFormats(t *testing.T) {
	cases := []struct {
		in  string
		def bool
		out bool
	}{
		{"true", false, true},
		{"TRUE", false, true},
		{"True", false, true},
		{"1", false, true},
		{"false", true, false},
		{"FALSE", true, false},
		{"False", true, false},
		{"0", true, false},
		{"", true, true},
		{"", false, false},
		{"banana", true, true},  // unrecognised → default
		{"banana", false, false},
	}
	for _, c := range cases {
		got := parseBool(c.in, c.def)
		if got != c.out {
			t.Errorf("parseBool(%q, def=%v) = %v, want %v", c.in, c.def, got, c.out)
		}
	}
}

func TestResolveExporterKind_PrecedenceTable(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want ExporterKind
		err  bool
	}{
		{
			name: "all unset -> noop",
			env:  nil,
			want: ExporterNoop,
		},
		{
			name: "helixcode override wins over traces console",
			env:  map[string]string{"HELIXCODE_OTEL_EXPORTER": "stdout", "OTEL_TRACES_EXPORTER": "none"},
			want: ExporterStdout,
		},
		{
			name: "traces console wins over otlp protocol",
			env:  map[string]string{"OTEL_TRACES_EXPORTER": "console", "OTEL_EXPORTER_OTLP_PROTOCOL": "grpc"},
			want: ExporterStdout,
		},
		{
			name: "traces none wins over otlp protocol",
			env:  map[string]string{"OTEL_TRACES_EXPORTER": "none", "OTEL_EXPORTER_OTLP_PROTOCOL": "grpc"},
			want: ExporterNoop,
		},
		{
			name: "otlp protocol grpc",
			env:  map[string]string{"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc"},
			want: ExporterOTLPGRPC,
		},
		{
			name: "otlp protocol http/protobuf",
			env:  map[string]string{"OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf"},
			want: ExporterOTLPHTTP,
		},
		{
			name: "traces otlp -> falls through to protocol or default grpc",
			env:  map[string]string{"OTEL_TRACES_EXPORTER": "otlp", "OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf"},
			want: ExporterOTLPHTTP,
		},
		{
			name: "unsupported protocol errors",
			env:  map[string]string{"OTEL_EXPORTER_OTLP_PROTOCOL": "http/json"},
			err:  true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveExporterKind(staticEnv(c.env))
			if c.err {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, ErrUnsupportedExporter) {
					t.Fatalf("err = %v, want ErrUnsupportedExporter", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
