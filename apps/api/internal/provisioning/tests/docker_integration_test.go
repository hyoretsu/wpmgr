//go:build provisioning_integration

package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mosamlife/wpmgr/apps/api/internal/httpclient"
	"github.com/mosamlife/wpmgr/apps/api/internal/provisioning"
)

// This fixture exercises the production Runtime, a real external MariaDB,
// WordPress/WP-CLI and persistent Docker resources. Enrollment is a stub; ACME
// is represented by a local HTTPS fixture and is not claimed as verified.
type fixtureCommander struct{ t *testing.T }

func (c fixtureCommander) Run(ctx context.Context, args []string, input []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = bytes.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.t.Logf("fixture command failure %v: %s", args, out)
	}
	return out, err
}

type enrolledFixture struct{}

func (enrolledFixture) Mint(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", fmt.Errorf("unexpected enrollment mint")
}
func (enrolledFixture) Connected(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}
func TestRuntimeDatabaseInstallRestartAndRetry(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("integration requires root for production secret file ownership; run compiled test in a root Docker container with host network/socket and identical state bind")
	}
	for _, v := range []string{"WPMGR_TEST_WORDPRESS_IMAGE", "WPMGR_TEST_WPCLI_IMAGE", "WPMGR_TEST_MARIADB_IMAGE", "WPMGR_TEST_STATE_DIR"} {
		if os.Getenv(v) == "" {
			t.Fatalf("missing %s", v)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	docker := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, "docker", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("docker %v: %s", args, out)
		}
		return strings.TrimSpace(string(out))
	}
	record := provisioning.Record{ID: uuid.New(), SiteID: uuid.New(), TenantID: uuid.New(), Title: "Fixture title", AdminUsername: "fixtureadmin", AdminEmail: "fixture@example.com"}
	name := "wpmgr_" + strings.ReplaceAll(record.SiteID.String(), "-", "")
	network := "provisioning-fixture-" + record.ID.String()
	dbContainer := network + "-db"
	docker("network", "create", network)
	defer func() {
		cleanup, c := context.WithTimeout(context.Background(), 30*time.Second)
		defer c()
		for _, args := range [][]string{{"rm", "-f", name, dbContainer}, {"volume", "rm", name + "_wordpress"}, {"network", "rm", network}} {
			_ = exec.CommandContext(cleanup, "docker", args...).Run()
		}
	}()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dbPort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	docker("run", "-d", "--pull=never", "--network", "host", "--name", dbContainer, "-e", "MARIADB_ROOT_PASSWORD=fixture_password", os.Getenv("WPMGR_TEST_MARIADB_IMAGE"), "--port="+fmt.Sprint(dbPort))
	db, err := sql.Open("mysql", fmt.Sprintf("root:fixture_password@tcp(127.0.0.1:%d)/?timeout=2s", dbPort))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for attempt := 0; attempt < 45; attempt++ {
		err = db.PingContext(ctx)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		t.Fatal("fixture MariaDB failed readiness")
	}
	var denyHTTPS atomic.Bool
	denyHTTPS.Store(true)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if denyHTTPS.Load() {
			w.WriteHeader(503)
			return
		}
		raw := docker("inspect", name)
		var inspect []struct {
			NetworkSettings struct {
				Networks map[string]struct{ IPAddress string }
			}
		}
		if json.Unmarshal([]byte(raw), &inspect) != nil || len(inspect) != 1 {
			t.Fatal("inspect fixture wordpress")
		}
		target, _ := url.Parse("http://" + inspect[0].NetworkSettings.Networks[network].IPAddress)
		r.Header.Set("X-Forwarded-Proto", "https")
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ModifyResponse = func(resp *http.Response) error {
			t.Logf("fixture HTTPS response status=%d marker=%t", resp.StatusCode, resp.Header.Get("X-WPMgr-Provisioning-ID") != "")
			t.Logf("fixture location=%s", resp.Header.Get("Location"))
			return nil
		}
		proxy.ServeHTTP(w, r)
	}))
	defer server.Close()
	record.Domain = strings.TrimPrefix(server.URL, "https://")
	record.Config = provisioning.RuntimeConfig{MariaDBAddress: fmt.Sprintf("host.docker.internal:%d", dbPort), ProxyNetwork: network, WordPressImage: os.Getenv("WPMGR_TEST_WORDPRESS_IMAGE"), WPCLIImage: os.Getenv("WPMGR_TEST_WPCLI_IMAGE")}
	creds := provisioning.Credentials{AdminPassword: "persistent_fixture_password", DatabasePassword: "persistent_database_password", Salts: map[string]string{}}
	for _, k := range []string{"AUTH_KEY", "SECURE_AUTH_KEY", "LOGGED_IN_KEY", "NONCE_KEY", "AUTH_SALT", "SECURE_AUTH_SALT", "LOGGED_IN_SALT", "NONCE_SALT"} {
		creds.Salts[k] = "fixture-salt-" + k
	}
	runtime := provisioning.NewRuntime(os.Getenv("WPMGR_TEST_STATE_DIR"), "root", "fixture_password", enrolledFixture{})
	runtime.Command = fixtureCommander{t: t}
	runtime.HTTP = httpclient.New(httpclient.Config{Timeout: 10 * time.Second, AllowPrivateNetworks: true, InsecureSkipTLSVerify: true})
	progress := func(string) error { return nil }
	if err = runtime.Run(ctx, record, creds, progress); err == nil || err.Error() != "https_unavailable" {
		t.Fatalf("expected retained HTTPS failure, got %v", err)
	}
	denyHTTPS.Store(false)
	if err = runtime.Run(ctx, record, creds, progress); err != nil {
		t.Fatalf("retry: %v", err)
	}
	docker("restart", name)
	if err = runtime.Run(ctx, record, creds, progress); err != nil {
		t.Fatalf("restart: %v", err)
	}
	file := filepath.Join(os.Getenv("WPMGR_TEST_STATE_DIR"), record.ID.String(), "compose.json")
	command := exec.CommandContext(ctx, "docker", "compose", "-f", file, "run", "--rm", "--no-deps", "-T", "cli", "wp", "eval-file", "--skip-wordpress", "/dev/stdin")
	command.Stdin = strings.NewReader(`require '/var/www/html/wp-load.php';$u=get_user_by('login','fixtureadmin');if(!wp_check_password('persistent_fixture_password',$u->user_pass,$u->ID)||get_option('blogname')!=='Fixture title'){WP_CLI::halt(1);}`)
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("persistent installation: %s", out)
	}
	databaseName := "wpmgr" + strings.ReplaceAll(record.SiteID.String(), "-", "")
	sibling := strings.Replace(databaseName, "wpmgr", "wpmgr_", 1)
	if _, err = db.ExecContext(ctx, "CREATE DATABASE `"+sibling+"`"); err != nil {
		t.Fatal(err)
	}
	scoped, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(127.0.0.1:%d)/", databaseName, creds.DatabasePassword, dbPort))
	if err != nil {
		t.Fatal(err)
	}
	defer scoped.Close()
	if _, err = scoped.ExecContext(ctx, "CREATE TABLE `"+sibling+"`.forbidden(value INT)"); err == nil {
		t.Fatal("site database grants escaped own database")
	}
}
