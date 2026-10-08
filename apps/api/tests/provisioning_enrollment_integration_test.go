//go:build provisioning_integration

package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mosamlife/wpmgr/apps/api/internal/domain"
	"github.com/mosamlife/wpmgr/apps/api/internal/httpclient"
	"github.com/mosamlife/wpmgr/apps/api/internal/provisioning"
	"github.com/mosamlife/wpmgr/apps/api/internal/site"
)

// This fixture exercises Runtime and real WordPress enrollment against the
// production site lifecycle and application-role Postgres pool. Local HTTP
// artifact/control-plane fixtures bypass production LoadConfig HTTPS validation
// and allow the exact artifact URL through WordPress safe-download checks.
// Production transport remains unchanged; ACME uses a local HTTPS fixture.
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

type realEnrollmentFixture struct{ repo site.Repo }

func (f realEnrollmentFixture) Mint(ctx context.Context, tenantID, siteID uuid.UUID) (string, error) {
	return site.MintProvisioningEnrollmentCode(ctx, f.repo, tenantID, siteID)
}

func (f realEnrollmentFixture) Connected(ctx context.Context, tenantID, siteID uuid.UUID) (bool, error) {
	st, err := f.repo.Get(ctx, tenantID, siteID)
	return st.ConnectionState == site.StateConnected, err
}

func TestRuntimeRealFleetEnrollmentAndRetry(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("integration requires root for production secret file ownership; run compiled test in a root Docker container with host network/socket and identical state bind")
	}
	for _, v := range []string{"WPMGR_TEST_WORDPRESS_IMAGE", "WPMGR_TEST_WPCLI_IMAGE", "WPMGR_TEST_MARIADB_IMAGE", "WPMGR_TEST_STATE_DIR", "WPMGR_TEST_AGENT_ZIP"} {
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
	app := startPostgres(t)
	siteRepo := site.NewRepo(app)
	tenantID := seedTenant(t, app, "runtime-enrollment-"+uuid.NewString())
	validator := domain.NewValidator()
	clock := domain.SystemClock{}
	svc := site.NewService(siteRepo, validator, clock)
	svc.SetConnectionService(site.NewConnectionService(siteRepo, validator, nil, nil, clock, nil))
	router := gin.New()
	site.NewHandler(svc, nil, base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))).RegisterPublic(router)
	var enrollCalls atomic.Int32
	artifact, err := os.ReadFile(os.Getenv("WPMGR_TEST_AGENT_ZIP"))
	if err != nil || len(artifact) == 0 {
		t.Fatalf("read agent artifact: %v", err)
	}
	controlPlane := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/agent.zip" {
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(artifact)
			return
		}
		if r.URL.Path == "/enroll" {
			enrollCalls.Add(1)
		}
		router.ServeHTTP(w, r)
	}))
	cpListener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	controlPlane.Listener = cpListener
	controlPlane.Start()
	defer controlPlane.Close()
	cpURL := "http://host.docker.internal:" + fmt.Sprint(cpListener.Addr().(*net.TCPAddr).Port)
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
	pending, err := siteRepo.CreatePending(ctx, tenantID, "https://"+record.Domain, record.Title, nil)
	if err != nil {
		t.Fatal(err)
	}
	record.TenantID = tenantID
	record.SiteID = pending.ID
	// Resource names derive from the actual pre-created Fleet site.
	name = "wpmgr_" + strings.ReplaceAll(record.SiteID.String(), "-", "")
	record.Config = provisioning.RuntimeConfig{MariaDBAddress: fmt.Sprintf("host.docker.internal:%d", dbPort), ProxyNetwork: network, WordPressImage: os.Getenv("WPMGR_TEST_WORDPRESS_IMAGE"), WPCLIImage: os.Getenv("WPMGR_TEST_WPCLI_IMAGE"), AgentPackage: cpURL + "/agent.zip", ControlPlaneURL: cpURL}
	creds := provisioning.Credentials{AdminPassword: "persistent_fixture_password", DatabasePassword: "persistent_database_password", Salts: map[string]string{}}
	for _, k := range []string{"AUTH_KEY", "SECURE_AUTH_KEY", "LOGGED_IN_KEY", "NONCE_KEY", "AUTH_SALT", "SECURE_AUTH_SALT", "LOGGED_IN_SALT", "NONCE_SALT"} {
		creds.Salts[k] = "fixture-salt-" + k
	}
	runtime := provisioning.NewRuntime(os.Getenv("WPMGR_TEST_STATE_DIR"), "root", "fixture_password", realEnrollmentFixture{repo: siteRepo})
	runtime.Command = fixtureCommander{t: t}
	runtime.HTTP = httpclient.New(httpclient.Config{Timeout: 10 * time.Second, AllowPrivateNetworks: true, InsecureSkipTLSVerify: true})
	// WP-CLI safe downloads reject private fixture hosts and dynamic ports.
	// Allow only this fixture artifact URL inside this disposable WordPress.
	artifactURL := cpURL + "/agent.zip"
	fixtureMU := fmt.Sprintf(`<?php
add_filter('http_request_host_is_external', static function($external, $host, $url) {
    return $url === %q ? true : $external;
}, 10, 3);
add_filter('http_allowed_safe_ports', static function($ports, $host, $url) {
    if ($url === %q) { $ports[] = %d; }
    return $ports;
}, 10, 3);
`, artifactURL, artifactURL, cpListener.Addr().(*net.TCPAddr).Port)
	progress := func(step string) error {
		if step != "agent" {
			return nil
		}
		file := filepath.Join(os.Getenv("WPMGR_TEST_STATE_DIR"), record.ID.String(), "compose.json")
		command := exec.CommandContext(ctx, "docker", "compose", "-f", file, "run", "--rm", "--no-deps", "-T", "cli", "wp", "eval-file", "--skip-wordpress", "/dev/stdin")
		command.Stdin = strings.NewReader(`require '/var/www/html/wp-load.php';$dir=WP_CONTENT_DIR.'/mu-plugins';if(!wp_mkdir_p($dir)||file_put_contents($dir.'/wpmgr-enrollment-fixture.php',base64_decode('` + base64.StdEncoding.EncodeToString([]byte(fixtureMU)) + `'))===false){WP_CLI::halt(1);}`)
		if out, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("install fixture download allowance: %w: %s", err, out)
		}
		return nil
	}
	if err = runtime.Run(ctx, record, creds, progress); err == nil || err.Error() != "https_unavailable" {
		t.Fatalf("expected retained HTTPS failure, got %v", err)
	}
	connected, err := siteRepo.Get(ctx, tenantID, record.SiteID)
	if err != nil || connected.ConnectionState != site.StateConnected || connected.ID != pending.ID {
		t.Fatalf("same pending site must connect: %v %+v", err, connected)
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
	if enrollCalls.Load() != 1 {
		t.Fatalf("retry/restart re-enrolled connected site: %d", enrollCalls.Load())
	}
	command = exec.CommandContext(ctx, "docker", "compose", "-f", file, "run", "--rm", "--no-deps", "-T", "cli", "wp", "eval-file", "--skip-wordpress", "/dev/stdin")
	command.Stdin = strings.NewReader(fmt.Sprintf(`require '/var/www/html/wp-load.php';if(get_option('wpmgr_agent_site_id')!==%q||get_option('wpmgr_agent_tenant_id')!==%q||get_option('wpmgr_agent_cp_url')!==%q){WP_CLI::halt(1);}`, record.SiteID.String(), tenantID.String(), cpURL))
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("persistent agent settings: %s", out)
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
