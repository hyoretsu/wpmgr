package provisioning

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type fakeCommands struct {
	calls    [][]string
	output   [][]byte
	failures []error
}

func (f *fakeCommands) Run(_ context.Context, args []string, _ []byte) ([]byte, error) {
	i := len(f.calls)
	f.calls = append(f.calls, args)
	return f.output[i], f.failures[i]
}
func TestResourceCollisionNeverCreates(t *testing.T) {
	f := &fakeCommands{output: [][]byte{[]byte(`[{"Labels":{"app.wpmgr.provisioning-id":"other"}}]`)}, failures: []error{nil}}
	r := &Runtime{Command: f}
	err := r.ensureResource(context.Background(), "volume", "fixture", "mine")
	if err == nil || err.Error() != "volume_collision" || len(f.calls) != 1 {
		t.Fatalf("collision adopted: %v %v", err, f.calls)
	}
}
func TestDockerFailureIsNotResourceAbsence(t *testing.T) {
	f := &fakeCommands{output: [][]byte{nil, nil}, failures: []error{errors.New("inspect failed"), errors.New("daemon unavailable")}}
	r := &Runtime{Command: f}
	err := r.ensureResource(context.Background(), "volume", "fixture", "mine")
	if err == nil || err.Error() != "docker_unavailable" || len(f.calls) != 2 {
		t.Fatalf("failed inspect adopted: %v %v", err, f.calls)
	}
}
func TestOwnedVolumeResumesWithoutCreate(t *testing.T) {
	f := &fakeCommands{output: [][]byte{[]byte(`[{"Labels":{"app.wpmgr.provisioning-id":"mine"}}]`)}, failures: []error{nil}}
	if err := (&Runtime{Command: f}).ensureResource(context.Background(), "volume", "fixture", "mine"); err != nil || len(f.calls) != 1 {
		t.Fatalf("resume failed: %v", err)
	}
}
func TestComposeSharesConfigAndPublishesNoPorts(t *testing.T) {
	rec := Record{ID: uuid.New(), SiteID: uuid.New(), Domain: "fixture.example", Config: RuntimeConfig{MariaDBAddress: "host.docker.internal:3306", ProxyNetwork: "proxy", WordPressImage: "wordpress:6.8.2-php8.3-apache", WPCLIImage: "wordpress:cli-2.12.0-php8.3", MariaDBTLS: true}}
	b, err := composeDocument(rec, "/secure/state")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Services map[string]struct {
			Environment map[string]string
			Ports       []string
			ExtraHosts  []string `json:"extra_hosts"`
			Secrets     []string
		}
	}
	if json.Unmarshal(b, &doc) != nil {
		t.Fatal("invalid compose")
	}
	wp, cli := doc.Services["wordpress"], doc.Services["cli"]
	for key, value := range wp.Environment {
		if cli.Environment[key] != value {
			t.Fatalf("CLI missing %s", key)
		}
	}
	if len(wp.Ports) != 0 || len(cli.Ports) != 0 || len(wp.ExtraHosts) == 0 || len(cli.Secrets) == 0 {
		t.Fatal("unsafe compose geometry")
	}
	if !strings.Contains(wp.Environment["WORDPRESS_CONFIG_EXTRA"], "MYSQLI_CLIENT_SSL") {
		t.Fatal("TLS absent")
	}
}
func TestInstallPHPParsesAndRetainsInstalledCredentials(t *testing.T) {
	if _, err := exec.LookPath("php"); err != nil {
		t.Fatal("PHP syntax verification binary unavailable")
	}
	script := installScript(Record{Domain: "fixture.example", Title: "Quote ' title", AdminUsername: "admin", AdminEmail: "admin@example.com"}, Credentials{AdminPassword: "secret ' \\"})
	cmd := exec.Command("php", "-l")
	cmd.Stdin = strings.NewReader("<?php " + script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("invalid install PHP: %s", out)
	}
	if strings.Contains(script, "update_option") || !strings.Contains(script, "if(!is_blog_installed())") {
		t.Fatal("initialized site changed")
	}
}
func TestRuntimeErrorsExposeSafeCodes(t *testing.T) {
	err := runtimeError("https_unavailable")
	var coded interface{ Code() string }
	if !errors.As(err, &coded) || coded.Code() != "https_unavailable" {
		t.Fatal("failure lost actionable code")
	}
}

func TestDatabaseNamesDoNotExpandGrantScope(t *testing.T) {
	record := Record{SiteID: uuid.MustParse("12345678-1234-1234-1234-123456789abc")}
	name := databaseName(record)
	if strings.ContainsAny(name, "_%") || name == resourceName(record) {
		t.Fatalf("database grant wildcard: %q", name)
	}
	record.Config = RuntimeConfig{MariaDBAddress: "host.docker.internal:3306"}
	raw, err := composeDocument(record, "/state")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Services map[string]struct{ Environment map[string]string }
	}
	if json.Unmarshal(raw, &doc) != nil {
		t.Fatal("bad compose")
	}
	for _, service := range []string{"wordpress", "cli"} {
		env := doc.Services[service].Environment
		if env["WORDPRESS_DB_NAME"] != name || env["WORDPRESS_DB_USER"] != name {
			t.Fatal("runtime and wordpress database names differ")
		}
	}
}

func TestHTTPSRejectsDifferentInstallation(t *testing.T) {
	record := Record{ID: uuid.New(), Domain: "fixture.example", Title: "Fixture"}
	expected := instanceMarker(record, Credentials{AdminPassword: "persistent-secret"})
	body := `{"url":"https://fixture.example","name":"Fixture","namespaces":["wp/v2"]}`
	response := func(marker string) *http.Response {
		r := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
		r.Header.Set("X-WPMgr-Provisioning-ID", marker)
		return r
	}
	for _, marker := range []string{"", "other"} {
		if verifiedHTTPS(response(marker), record, expected) {
			t.Fatal("unrelated HTTPS marked complete")
		}
	}
	if !verifiedHTTPS(response(expected), record, expected) {
		t.Fatal("owned installation rejected")
	}
	wrong := response(expected)
	wrong.Body = io.NopCloser(strings.NewReader(`{"url":"https://other.example","name":"Fixture","namespaces":["wp/v2"]}`))
	if verifiedHTTPS(wrong, record, expected) {
		t.Fatal("wrong domain accepted")
	}
	script := identityScript(expected)
	cmd := exec.Command("php", "-l")
	cmd.Stdin = strings.NewReader("<?php " + script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("identity PHP invalid: %s", out)
	}
}

func TestVolumeCreationRaceRechecksOwnership(t *testing.T) {
	f := &fakeCommands{output: [][]byte{nil, []byte(""), []byte("fixture"), []byte(`[{"Labels":{"app.wpmgr.provisioning-id":"foreign"}}]`)}, failures: []error{errors.New("missing"), nil, nil, nil}}
	if err := (&Runtime{Command: f}).ensureResource(context.Background(), "volume", "fixture", "mine"); err == nil || err.Error() != "volume_collision" {
		t.Fatalf("foreign volume adopted: %v", err)
	}
}
func TestRunningCLIBlocksRetry(t *testing.T) {
	f := &fakeCommands{output: [][]byte{[]byte(`[{"Id":"a7fb760e8347bdb07038f8c92dcf8db65f10b7fb56ce153901ecf36314229d94","Config":{"Labels":{"app.wpmgr.provisioning-id":"mine"}},"State":{"Running":true,"Status":"running"}}]`)}, failures: []error{nil}}
	if err := (&Runtime{Command: f}).checkCLI(context.Background(), "fixture_cli", "mine"); err == nil || err.Error() != "cli_busy" || len(f.calls) != 1 {
		t.Fatalf("running CLI overlapped: %v", err)
	}
}

func TestStoppedCLIRemovalBindsInspectedContainerID(t *testing.T) {
	const containerID = "a7fb760e8347bdb07038f8c92dcf8db65f10b7fb56ce153901ecf36314229d94"
	f := &fakeCommands{
		output:   [][]byte{[]byte(`[{"Id":"` + containerID + `","Config":{"Labels":{"app.wpmgr.provisioning-id":"mine"}},"State":{"Running":false,"Status":"exited"}}]`), nil},
		failures: []error{nil, nil},
	}
	if err := (&Runtime{Command: f}).checkCLI(context.Background(), "fixture_cli", "mine"); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 2 || strings.Join(f.calls[1], " ") != "docker container rm "+containerID {
		t.Fatalf("removal must target inspected container identity: %v", f.calls)
	}
}

func TestStoppedCLIWithoutContainerIDBlocksRemoval(t *testing.T) {
	for _, idField := range []string{"", `"Id":"",`} {
		t.Run(idField, func(t *testing.T) {
			f := &fakeCommands{
				output:   [][]byte{[]byte(`[{` + idField + `"Config":{"Labels":{"app.wpmgr.provisioning-id":"mine"}},"State":{"Running":false,"Status":"exited"}}]`), nil},
				failures: []error{nil, nil},
			}
			if err := (&Runtime{Command: f}).checkCLI(context.Background(), "fixture_cli", "mine"); err == nil || err.Error() != "cli_collision" || len(f.calls) != 1 {
				t.Fatalf("helper without immutable identity was removed: %v %v", err, f.calls)
			}
		})
	}
}
