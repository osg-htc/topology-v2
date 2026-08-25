package config

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// clearTopologyEnv unsets every TOPOLOGY_* var this package reads, so a test
// observes only what it itself sets (not whatever's in the actual shell
// environment this test binary happens to run under).
func clearTopologyEnv(t *testing.T) {
	t.Helper()
	for _, v := range []string{
		"TOPOLOGY_APP_ENV", "TOPOLOGY_PORT", "TOPOLOGY_BASE_URL", "TOPOLOGY_DATABASE_URL",
		"TOPOLOGY_INSTANCE_KEY", "TOPOLOGY_MASTER_KEY_PATH",
		"TOPOLOGY_S3_ENDPOINT", "TOPOLOGY_S3_REGION", "TOPOLOGY_S3_BUCKET",
		"TOPOLOGY_S3_ACCESS_KEY", "TOPOLOGY_S3_SECRET_KEY", "TOPOLOGY_S3_USE_PATH_STYLE",
		"TOPOLOGY_OIDC_ISSUER", "TOPOLOGY_OIDC_CLIENT_ID", "TOPOLOGY_OIDC_CLIENT_SECRET",
		"TOPOLOGY_INSTITUTIONS_API", "TOPOLOGY_GITHUB_REPO", "TOPOLOGY_GITHUB_TOKEN",
	} {
		t.Setenv(v, "")
		os.Unsetenv(v)
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearTopologyEnv(t)
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cases := map[string]struct{ got, want string }{
		"AppEnv":          {c.AppEnv, "development"},
		"Port":            {c.Port, "8080"},
		"BaseURL":         {c.BaseURL, "http://localhost:8080"},
		"MasterKeyPath":   {c.MasterKeyPath, ".topology-master.key"},
		"S3Region":        {c.S3Region, "us-east-1"},
		"OIDCIssuer":      {c.OIDCIssuer, "https://cilogon.org"},
		"InstitutionsAPI": {c.InstitutionsAPI, "https://topology-institutions.osg-htc.org/api"},
		"GitHubRepo":      {c.GitHubRepo, "opensciencegrid/topology"},
	}
	for field, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want default %q", field, c.got, c.want)
		}
	}
	if !c.S3UsePathStyle {
		t.Error("S3UsePathStyle default = false, want true")
	}
}

func TestLoad_OverridesFromEnv(t *testing.T) {
	clearTopologyEnv(t)
	t.Setenv("TOPOLOGY_APP_ENV", "production")
	t.Setenv("TOPOLOGY_PORT", "9090")
	t.Setenv("TOPOLOGY_DATABASE_URL", "postgres://example/db")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.AppEnv != "production" {
		t.Errorf("AppEnv = %q, want production", c.AppEnv)
	}
	if c.Port != "9090" {
		t.Errorf("Port = %q, want 9090", c.Port)
	}
	if c.DatabaseURL != "postgres://example/db" {
		t.Errorf("DatabaseURL = %q", c.DatabaseURL)
	}
	// A var that wasn't overridden must still fall back to its default.
	if c.S3Region != "us-east-1" {
		t.Errorf("S3Region = %q, want the untouched default us-east-1", c.S3Region)
	}
}

func TestIsProduction(t *testing.T) {
	cases := []struct {
		appEnv string
		want   bool
	}{
		{"production", true},
		{"Production", true},
		{"PRODUCTION", true},
		{"development", false},
		{"", false},
		{"staging", false},
	}
	for _, c := range cases {
		got := (&Config{AppEnv: c.appEnv}).IsProduction()
		if got != c.want {
			t.Errorf("IsProduction() with AppEnv=%q = %v, want %v", c.appEnv, got, c.want)
		}
	}
}

func TestValidateServer(t *testing.T) {
	t.Run("missing database URL is always required", func(t *testing.T) {
		c := &Config{AppEnv: "development"}
		err := c.ValidateServer()
		if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
			t.Errorf("got %v, want an error naming DATABASE_URL", err)
		}
	})

	t.Run("development mode doesn't require S3/OIDC", func(t *testing.T) {
		c := &Config{AppEnv: "development", DatabaseURL: "postgres://x"}
		if err := c.ValidateServer(); err != nil {
			t.Errorf("got %v, want no error", err)
		}
	})

	t.Run("production mode requires S3 bucket and OIDC client id", func(t *testing.T) {
		c := &Config{AppEnv: "production", DatabaseURL: "postgres://x"}
		err := c.ValidateServer()
		if err == nil {
			t.Fatal("got no error, want one naming the missing production requirements")
		}
		if !strings.Contains(err.Error(), "S3_BUCKET") {
			t.Errorf("error %q doesn't mention S3_BUCKET", err)
		}
		if !strings.Contains(err.Error(), "OIDC_CLIENT_ID") {
			t.Errorf("error %q doesn't mention OIDC_CLIENT_ID", err)
		}
	})

	t.Run("production mode with everything set passes", func(t *testing.T) {
		c := &Config{
			AppEnv: "production", DatabaseURL: "postgres://x",
			S3Bucket: "my-bucket", OIDCClientID: "my-client-id",
		}
		if err := c.ValidateServer(); err != nil {
			t.Errorf("got %v, want no error", err)
		}
	})
}

func TestEnsureMasterKey_FromInstanceKey(t *testing.T) {
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i)
	}
	c := &Config{InstanceKey: hex.EncodeToString(raw), MasterKeyPath: filepath.Join(t.TempDir(), "unused.key")}
	if err := c.EnsureMasterKey(); err != nil {
		t.Fatalf("EnsureMasterKey: %v", err)
	}
	if got := c.MasterKey(); hex.EncodeToString(got) != hex.EncodeToString(raw) {
		t.Errorf("MasterKey() = %x, want %x", got, raw)
	}
}

func TestEnsureMasterKey_RejectsInvalidInstanceKey(t *testing.T) {
	for _, bad := range []string{"not-hex-at-all", "deadbeef", strings.Repeat("ab", 31), strings.Repeat("ab", 33)} {
		c := &Config{InstanceKey: bad}
		if err := c.EnsureMasterKey(); err == nil {
			t.Errorf("InstanceKey=%q: got no error, want one (must be exactly 32 bytes of hex)", bad)
		}
	}
}

func TestEnsureMasterKey_GeneratesAndPersists(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "master.key")
	c := &Config{MasterKeyPath: keyPath}
	if err := c.EnsureMasterKey(); err != nil {
		t.Fatalf("EnsureMasterKey: %v", err)
	}
	if len(c.MasterKey()) != 32 {
		t.Fatalf("generated key is %d bytes, want 32", len(c.MasterKey()))
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("key file was not persisted: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("key file mode = %o, want 0600 -- a world/group-readable master key is a real secret-exposure risk", perm)
	}

	// A restart (fresh Config, same path) must read the SAME key back, not
	// generate a new one -- regenerating on every restart would silently
	// make all previously-encrypted data undecryptable.
	c2 := &Config{MasterKeyPath: keyPath}
	if err := c2.EnsureMasterKey(); err != nil {
		t.Fatalf("EnsureMasterKey (second load): %v", err)
	}
	if hex.EncodeToString(c2.MasterKey()) != hex.EncodeToString(c.MasterKey()) {
		t.Error("a second EnsureMasterKey against the same path generated a different key instead of reusing the persisted one")
	}
}

func TestEnsureMasterKey_TwoInstancesGetDifferentKeys(t *testing.T) {
	c1 := &Config{MasterKeyPath: filepath.Join(t.TempDir(), "master.key")}
	c2 := &Config{MasterKeyPath: filepath.Join(t.TempDir(), "master.key")}
	if err := c1.EnsureMasterKey(); err != nil {
		t.Fatal(err)
	}
	if err := c2.EnsureMasterKey(); err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(c1.MasterKey()) == hex.EncodeToString(c2.MasterKey()) {
		t.Error("two independently generated master keys collided -- randomness is broken")
	}
}

func TestEnsureMasterKey_CorruptPersistedFileIsRegenerated(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "master.key")
	if err := os.WriteFile(keyPath, []byte("not valid hex and not 32 bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &Config{MasterKeyPath: keyPath}
	if err := c.EnsureMasterKey(); err != nil {
		t.Fatalf("EnsureMasterKey with a corrupt persisted file: %v", err)
	}
	if len(c.MasterKey()) != 32 {
		t.Fatalf("got %d bytes, want a freshly generated 32-byte key", len(c.MasterKey()))
	}
}
