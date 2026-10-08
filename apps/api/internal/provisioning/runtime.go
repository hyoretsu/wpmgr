package provisioning

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/mosamlife/wpmgr/apps/api/internal/httpclient"
)

// Commander executes bounded host commands. Output is never included in errors.
type Commander interface {
	Run(context.Context, []string, []byte) ([]byte, error)
}
type hostCommander struct{}

func (hostCommander) Run(ctx context.Context, args []string, input []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stderr = io.Discard
	return cmd.Output()
}

type Runtime struct {
	StateDir, MariaDBUsername, MariaDBPassword string
	Enrollment                                 Enrollment
	Command                                    Commander
	HTTP                                       *httpclient.Client
}

func NewRuntime(dir, user, password string, enrollment Enrollment) *Runtime {
	return &Runtime{StateDir: dir, MariaDBUsername: user, MariaDBPassword: password, Enrollment: enrollment, Command: hostCommander{}, HTTP: httpclient.New(httpclient.DefaultConfig())}
}

type RuntimeError string

func (e RuntimeError) Error() string { return string(e) }
func (e RuntimeError) Code() string  { return string(e) }
func runtimeError(code string) error { return RuntimeError(code) }
func (r *Runtime) Run(ctx context.Context, rec Record, creds Credentials, progress func(string) error) error {
	if r.Enrollment == nil || r.Command == nil || r.HTTP == nil || !filepath.IsAbs(r.StateDir) {
		return runtimeError("worker_configuration")
	}
	dir := filepath.Join(r.StateDir, rec.ID.String())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return runtimeError("state_unavailable")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return runtimeError("state_unavailable")
	}
	secret := func(n, v string) error {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte(v), 0600); err != nil {
			return err
		}
		return os.Chown(p, 33, 33)
	}
	if err := secret("operation.php", ""); err != nil {
		return runtimeError("state_unavailable")
	}
	if err := secret("db_password", creds.DatabasePassword); err != nil {
		return runtimeError("state_unavailable")
	}
	for key, value := range creds.Salts {
		if strings.ContainsAny(key, "/\\.") {
			return runtimeError("worker_configuration")
		}
		if err := secret(strings.ToLower(key), value); err != nil {
			return runtimeError("state_unavailable")
		}
	}
	compose, err := composeDocument(rec, dir)
	if err != nil {
		return runtimeError("worker_configuration")
	}
	file := filepath.Join(dir, "compose.json")
	if err = os.WriteFile(file, compose, 0600); err != nil {
		return runtimeError("state_unavailable")
	}
	run := func(args ...string) ([]byte, error) {
		return r.Command.Run(ctx, append([]string{"docker"}, args...), nil)
	}
	cli := func(script string) ([]byte, error) {
		return r.runCLI(ctx, rec, file, script)
	}
	for _, image := range []string{rec.Config.WordPressImage, rec.Config.WPCLIImage} {
		if _, err = run("image", "inspect", image); err != nil {
			return runtimeError("approved_image_unavailable")
		}
	}
	if _, err = run("network", "inspect", rec.Config.ProxyNetwork); err != nil {
		return runtimeError("proxy_network_unavailable")
	}
	if err = progress("database"); err != nil {
		return err
	}
	if err = r.database(ctx, rec, creds); err != nil {
		return err
	}
	if err = progress("volume"); err != nil {
		return err
	}
	volume := resourceName(rec) + "_wordpress"
	if err = r.ensureResource(ctx, "volume", volume, rec.ID.String()); err != nil {
		return err
	}
	if err = progress("wordpress"); err != nil {
		return err
	}
	if out, e := run("container", "inspect", resourceName(rec)); e == nil {
		if !hasOwner(out, rec.ID.String()) {
			return runtimeError("container_collision")
		}
	} else {
		if _, e = run("ps", "-a", "--format", "{{.Names}}"); e != nil {
			return runtimeError("docker_unavailable")
		}
	}
	if _, err = run("compose", "-f", file, "up", "-d", "--no-deps", "--pull", "never", "wordpress"); err != nil {
		return runtimeError("wordpress_start_failed")
	}
	if err = progress("install"); err != nil {
		return err
	}
	script := installScript(rec, creds)
	// WP config creation happens in the WordPress entrypoint; allow a bounded startup window.
	for attempt := 0; attempt < 12; attempt++ {
		_, err = cli(script)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return runtimeError("wordpress_install_failed")
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		return runtimeError("wordpress_install_failed")
	}
	marker := instanceMarker(rec, creds)
	if _, err = cli(identityScript(marker)); err != nil {
		return runtimeError("installation_identity_failed")
	}
	if err = progress("agent"); err != nil {
		return err
	}
	connected, err := r.Enrollment.Connected(ctx, rec.TenantID, rec.SiteID)
	if err != nil {
		return runtimeError("agent_status_failed")
	}
	if !connected {
		pkg, _ := json.Marshal(rec.Config.AgentPackage)
		if _, err = cli(`require '/var/www/html/wp-load.php';if(!class_exists('WPMgr\\Agent\\Enrollment')){WP_CLI::runcommand('plugin install '.escapeshellarg(json_decode('` + escapePHP(string(pkg)) + `',true)).' --activate', ['exit_error'=>true]);}`); err != nil {
			return runtimeError("agent_install_failed")
		}
		code, e := r.Enrollment.Mint(ctx, rec.TenantID, rec.SiteID)
		if e != nil {
			return runtimeError("agent_enrollment_failed")
		}
		v, _ := json.Marshal(map[string]string{"url": rec.Config.ControlPlaneURL, "code": code})
		script = `require '/var/www/html/wp-load.php';$v=json_decode(base64_decode('` + base64PHP(v) + `'),true); $k=new \WPMgr\Agent\Keystore();$s=new \WPMgr\Agent\Settings();$s->setControlPlaneUrl($v['url']);$e=new \WPMgr\Agent\Enrollment($k,$s,new \WPMgr\Agent\Signer($k),new \WPMgr\Agent\Commands\MetadataCommand(new \WPMgr\Agent\Support\AgeIdentity($k)));$a=$e->enroll($v['code']);if (!$a['ok']) {WP_CLI::halt(1);}`
		if _, err = cli(script); err != nil {
			return runtimeError("agent_enrollment_failed")
		}
	}
	connected, err = r.Enrollment.Connected(ctx, rec.TenantID, rec.SiteID)
	if err != nil || !connected {
		return runtimeError("agent_enrollment_failed")
	}
	if err = progress("https"); err != nil {
		return err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+rec.Domain+"/?rest_route=/", nil)
	resp, err := r.HTTP.DoOnce(req)
	if err != nil {
		return runtimeError("https_unavailable")
	}
	defer resp.Body.Close()
	if !verifiedHTTPS(resp, rec, marker) {
		return runtimeError("https_unavailable")
	}

	return nil
}
func hasOwner(out []byte, id string) bool {
	var items []struct {
		Labels map[string]string
		Config struct{ Labels map[string]string }
	}
	if json.Unmarshal(out, &items) != nil || len(items) != 1 {
		return false
	}
	return items[0].Labels[ownerLabel] == id || items[0].Config.Labels[ownerLabel] == id
}
func (r *Runtime) ensureResource(ctx context.Context, kind, name, id string) error {
	out, err := r.Command.Run(ctx, []string{"docker", kind, "inspect", name}, nil)
	if err == nil {
		if !hasOwner(out, id) {
			return runtimeError(kind + "_collision")
		}
		return nil
	}
	// A failed inspect is not evidence of absence: confirm list before creating.
	out, err = r.Command.Run(ctx, []string{"docker", kind, "ls", "--format", "{{.Name}}"}, nil)
	if err != nil {
		return runtimeError("docker_unavailable")
	}
	for _, n := range strings.Fields(string(out)) {
		if n == name {
			return runtimeError(kind + "_collision")
		}
	}
	_, err = r.Command.Run(ctx, []string{"docker", kind, "create", "--label", ownerLabel + "=" + id, name}, nil)
	if err != nil {
		return runtimeError(kind + "_create_failed")
	}
	out, err = r.Command.Run(ctx, []string{"docker", kind, "inspect", name}, nil)
	if err != nil || !hasOwner(out, id) {
		return runtimeError(kind + "_collision")
	}
	return nil
}
func (r *Runtime) database(ctx context.Context, rec Record, creds Credentials) error {
	c := mysql.NewConfig()
	c.User = r.MariaDBUsername
	c.Passwd = r.MariaDBPassword
	c.Net = "tcp"
	c.Addr = rec.Config.MariaDBAddress
	c.Timeout = 10 * time.Second
	c.ReadTimeout = 30 * time.Second
	c.WriteTimeout = 30 * time.Second
	if rec.Config.MariaDBTLS {
		c.TLSConfig = "true"
	}
	db, err := sql.Open("mysql", c.FormatDSN())
	if err != nil {
		return runtimeError("database_unavailable")
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return runtimeError("database_unavailable")
	}
	defer conn.Close()
	name := databaseName(rec)
	var lock int
	if err = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", name).Scan(&lock); err != nil || lock != 1 {
		return runtimeError("database_busy")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(cleanup, "DO RELEASE_LOCK(?)", name)
	}()
	var exists int
	if err = conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?", name).Scan(&exists); err != nil {
		return runtimeError("database_unavailable")
	}
	if exists != 0 {
		var owner string
		if conn.QueryRowContext(ctx, "SELECT SCHEMA_COMMENT FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=?", name).Scan(&owner) != nil || owner != rec.ID.String() {
			return runtimeError("database_collision")
		}
		return r.databaseUser(ctx, conn, c, name, creds.DatabasePassword, true)
	}

	if err = conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM mysql.user WHERE User=?", name).Scan(&exists); err != nil {
		return runtimeError("database_privileges_missing")
	}
	if exists != 0 {
		return runtimeError("database_user_collision")
	}
	if _, err = conn.ExecContext(ctx, "CREATE DATABASE `"+name+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci COMMENT '"+rec.ID.String()+"'"); err != nil {
		return runtimeError("database_create_failed")
	}

	return r.databaseUser(ctx, conn, c, name, creds.DatabasePassword, false)
}
func (r *Runtime) databaseUser(ctx context.Context, conn *sql.Conn, c *mysql.Config, name, password string, resume bool) error {
	var exists int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM mysql.user WHERE User=?", name).Scan(&exists); err != nil {
		return runtimeError("database_privileges_missing")
	}
	if exists != 0 {
		if !resume {
			return runtimeError("database_user_collision")
		}
		own := *c
		own.User = name
		own.Passwd = password
		check, err := sql.Open("mysql", own.FormatDSN())
		if err != nil {
			return runtimeError("database_user_collision")
		}
		defer check.Close()
		if check.PingContext(ctx) != nil {
			return runtimeError("database_user_collision")
		}
	}
	// Native password hashes avoid SQL quoting of plaintext passwords.
	user := "'" + name + "'@'%'"
	digest := sha1.Sum([]byte(password))
	double := sha1.Sum(digest[:])
	pw := fmt.Sprintf("'*%X'", double)
	if exists == 0 {
		if _, err := conn.ExecContext(ctx, "CREATE USER "+user+" IDENTIFIED BY PASSWORD "+pw); err != nil {
			return runtimeError("database_user_create_failed")
		}
	}
	if _, err := conn.ExecContext(ctx, "GRANT ALL PRIVILEGES ON `"+name+"`.* TO "+user); err != nil {
		return runtimeError("database_grant_failed")
	}
	return nil
}

func base64PHP(v []byte) string { return base64.StdEncoding.EncodeToString(v) }
func escapePHP(v string) string {
	return strings.ReplaceAll(strings.ReplaceAll(v, "\\", "\\\\"), "'", "\\'")
}

func installScript(rec Record, creds Credentials) string {
	payload, _ := json.Marshal(map[string]string{"url": "https://" + rec.Domain, "domain": rec.Domain, "title": rec.Title, "user": rec.AdminUsername, "email": rec.AdminEmail, "password": creds.AdminPassword})
	return `$v=json_decode(base64_decode('` + base64PHP(payload) + `'),true);define('WP_INSTALLING',true);$_SERVER['HTTP_HOST']=$v['domain'];$_SERVER['REQUEST_URI']='/';$_SERVER['HTTPS']='on';require '/var/www/html/wp-load.php';require '/var/www/html/wp-admin/includes/upgrade.php';if(!is_blog_installed()){wp_install($v['title'],$v['user'],$v['email'],true,'',$v['password']);}if(!is_blog_installed()||get_option('home')!==$v['url']||get_option('siteurl')!==$v['url']||get_option('blogname')!==$v['title']){WP_CLI::halt(1);}`
}

// Database names contain no SQL grant wildcards. Docker names stay independent.
func databaseName(r Record) string { return "wpmgr" + strings.ReplaceAll(r.SiteID.String(), "-", "") }

func instanceMarker(rec Record, creds Credentials) string {
	h := hmac.New(sha256.New, []byte(creds.AdminPassword))
	h.Write([]byte(rec.ID.String()))
	return hex.EncodeToString(h.Sum(nil))
}
func identityScript(marker string) string {
	code := `<?php add_filter('rest_post_dispatch',function($response){$response->header('X-WPMgr-Provisioning-ID','` + marker + `');return $response;});`
	return `require '/var/www/html/wp-load.php';$path=WP_CONTENT_DIR.'/mu-plugins/wpmgr-provisioning-identity.php';$code=base64_decode('` + base64PHP([]byte(code)) + `');if(file_exists($path)){if(file_get_contents($path)!==$code){WP_CLI::halt(1);}}else{if(!wp_mkdir_p(dirname($path))||file_put_contents($path,$code,LOCK_EX)===false){WP_CLI::halt(1);}}`
}

func verifiedHTTPS(resp *http.Response, rec Record, marker string) bool {
	var identity struct {
		URL        string   `json:"url"`
		Name       string   `json:"name"`
		Namespaces []string `json:"namespaces"`
	}
	if resp.Header.Get("X-WPMgr-Provisioning-ID") != marker || resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&identity) != nil || identity.URL != "https://"+rec.Domain || identity.Name != rec.Title || len(identity.Namespaces) == 0 {
		return false
	}
	return true
}

func (r *Runtime) runCLI(ctx context.Context, rec Record, file, script string) ([]byte, error) {
	name := resourceName(rec) + "_cli"
	if err := r.checkCLI(ctx, name, rec.ID.String()); err != nil {
		return nil, err
	}
	path := filepath.Join(filepath.Dir(file), "operation.php")
	standalone := strings.Contains(script, "define('WP_INSTALLING',true)")
	if !standalone {
		script = strings.ReplaceAll(script, "require '/var/www/html/wp-load.php';", "")
	}
	if standalone {
		script = strings.ReplaceAll(script, "WP_CLI::halt(1)", "exit(1)")
	}
	if err := os.WriteFile(path, []byte("<?php "+script), 0600); err != nil {
		return nil, runtimeError("state_unavailable")
	}
	if err := os.Chown(path, 33, 33); err != nil {
		return nil, runtimeError("state_unavailable")
	}
	args := []string{"docker", "compose", "-f", file, "run", "--name", name, "--rm", "--no-deps", "-T", "cli"}
	if standalone {
		args = append(args, "php", "/run/secrets/operation")
	} else {
		args = append(args, "wp", "eval-file", "/run/secrets/operation")
	}
	return r.Command.Run(ctx, args, nil)

}
func (r *Runtime) checkCLI(ctx context.Context, name, id string) error {
	out, err := r.Command.Run(ctx, []string{"docker", "container", "inspect", name}, nil)
	if err != nil {
		out, err = r.Command.Run(ctx, []string{"docker", "ps", "-a", "--format", "{{.Names}}"}, nil)
		if err != nil {
			return runtimeError("docker_unavailable")
		}
		for _, n := range strings.Fields(string(out)) {
			if n == name {
				return runtimeError("cli_collision")
			}
		}
		return nil
	}
	if !hasOwner(out, id) {
		return runtimeError("cli_collision")
	}
	var items []struct {
		State struct {
			Running bool
			Status  string
		}
	}
	if json.Unmarshal(out, &items) != nil || len(items) != 1 {
		return runtimeError("cli_collision")
	}
	if items[0].State.Running || items[0].State.Status == "created" || items[0].State.Status == "restarting" {
		return runtimeError("cli_busy")
	}
	// Only a stopped, owned helper may be removed; site volumes stay intact.
	if _, err = r.Command.Run(ctx, []string{"docker", "container", "rm", name}, nil); err != nil {
		return runtimeError("cli_busy")
	}
	return nil
}
