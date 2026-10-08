package provisioning

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

const ownerLabel = "app.wpmgr.provisioning-id"

func resourceName(r Record) string { return "wpmgr_" + strings.ReplaceAll(r.SiteID.String(), "-", "") }
func composeDocument(r Record, dir string) ([]byte, error) {
	name := resourceName(r)
	env := map[string]string{"WORDPRESS_DB_HOST": r.Config.MariaDBAddress, "WORDPRESS_DB_NAME": name, "WORDPRESS_DB_USER": name, "WORDPRESS_DB_PASSWORD_FILE": "/run/secrets/db_password"}
	secrets := map[string]any{"db_password": map[string]string{"file": filepath.Join(dir, "db_password")}}
	names := []string{"db_password"}
	for _, key := range []string{"AUTH_KEY", "SECURE_AUTH_KEY", "LOGGED_IN_KEY", "NONCE_KEY", "AUTH_SALT", "SECURE_AUTH_SALT", "LOGGED_IN_SALT", "NONCE_SALT"} {
		n := strings.ToLower(key)
		env["WORDPRESS_"+key+"_FILE"] = "/run/secrets/" + n
		secrets[n] = map[string]string{"file": filepath.Join(dir, n)}
		names = append(names, n)
	}
	if r.Config.MariaDBTLS {
		env["WORDPRESS_CONFIG_EXTRA"] = "define('MYSQL_CLIENT_FLAGS', MYSQLI_CLIENT_SSL);"
	}
	volume := name + "_wordpress"
	shared := map[string]any{"volumes": []string{volume + ":/var/www/html"}, "networks": []string{"proxy"}, "extra_hosts": []string{"host.docker.internal:host-gateway"}, "labels": map[string]string{ownerLabel: r.ID.String()}, "pull_policy": "never"}
	wp := map[string]any{}
	cli := map[string]any{}
	for k, v := range shared {
		wp[k] = v
		cli[k] = v
	}
	wp["image"] = r.Config.WordPressImage
	wp["environment"] = env
	wp["secrets"] = names
	wp["restart"] = "unless-stopped"
	wp["container_name"] = name
	wp["labels"] = map[string]string{ownerLabel: r.ID.String(), "caddy": r.Domain, "caddy.reverse_proxy": "{{upstreams 80}}", "caddy_ingress_network": r.Config.ProxyNetwork}
	cli["image"] = r.Config.WPCLIImage
	cli["user"] = "33:33"
	cliEnv := map[string]string{"HOME": "/tmp"}
	for k, v := range env {
		cliEnv[k] = v
	}
	cli["environment"] = cliEnv
	cli["profiles"] = []string{"cli"}
	cli["secrets"] = names
	return json.Marshal(map[string]any{"name": name, "services": map[string]any{"wordpress": wp, "cli": cli}, "volumes": map[string]any{volume: map[string]any{"external": true}}, "networks": map[string]any{"proxy": map[string]any{"external": true, "name": r.Config.ProxyNetwork}}, "secrets": secrets})
}
