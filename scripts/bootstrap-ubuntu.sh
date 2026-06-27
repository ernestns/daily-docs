#!/usr/bin/env bash
set -euo pipefail

APP_NAME="${APP_NAME:-dailydocs}"
BINARY_NAME="${BINARY_NAME:-dailydocs}"
APP_USER="${APP_USER:-dailydocs}"
APP_GROUP="${APP_GROUP:-dailydocs}"
DEPLOY_USER="${DEPLOY_USER:-deploy}"
DEPLOY_GROUP="${DEPLOY_GROUP:-$DEPLOY_USER}"
DEPLOY_HOME="${DEPLOY_HOME:-/home/$DEPLOY_USER}"
DEPLOY_REPO_DIR="${DEPLOY_REPO_DIR:-$DEPLOY_HOME/daily-docs}"
DEPLOY_HELPER="${DEPLOY_HELPER:-/usr/local/bin/$APP_NAME-install}"
REPO_URL="${REPO_URL:-https://github.com/ernestns/daily-docs.git}"
APP_DIR="${APP_DIR:-/opt/dailydocs}"
DATA_DIR="${DATA_DIR:-$APP_DIR/data}"
DB_PATH="${DB_PATH:-$DATA_DIR/dailydocs.sqlite}"
APP_ADDR="${APP_ADDR:-127.0.0.1:8080}"
DOMAIN="${DOMAIN:-dailydocs.dev}"

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ "$(id -u)" -ne 0 ]]; then
	echo "Run this script as root." >&2
	exit 1
fi

if ! command -v apt-get >/dev/null 2>&1; then
	echo "This bootstrap script is intended for Ubuntu/Debian systems with apt-get." >&2
	exit 1
fi

echo "==> Installing system packages"
apt-get update
env DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates curl git golang-go caddy sudo

echo "==> Creating application user"
if ! getent group "$APP_GROUP" >/dev/null 2>&1; then
	groupadd --system "$APP_GROUP"
fi

if ! id "$APP_USER" >/dev/null 2>&1; then
	useradd --system --gid "$APP_GROUP" --home "$APP_DIR" --shell /usr/sbin/nologin "$APP_USER"
fi

echo "==> Creating deploy user"
if ! getent group "$DEPLOY_GROUP" >/dev/null 2>&1; then
	groupadd "$DEPLOY_GROUP"
fi

if ! id "$DEPLOY_USER" >/dev/null 2>&1; then
	useradd --create-home --gid "$DEPLOY_GROUP" --home-dir "$DEPLOY_HOME" --shell /bin/bash "$DEPLOY_USER"
fi

mkdir -p "$DEPLOY_HOME"
chown "$DEPLOY_USER:$DEPLOY_GROUP" "$DEPLOY_HOME"
chmod 0755 "$DEPLOY_HOME"

if [[ -n "${DEPLOY_SSH_PUBLIC_KEY:-}" ]]; then
	echo "==> Installing deploy SSH key"
	install -d -m 0700 -o "$DEPLOY_USER" -g "$DEPLOY_GROUP" "$DEPLOY_HOME/.ssh"
	touch "$DEPLOY_HOME/.ssh/authorized_keys"
	if ! grep -qxF "$DEPLOY_SSH_PUBLIC_KEY" "$DEPLOY_HOME/.ssh/authorized_keys"; then
		printf '%s\n' "$DEPLOY_SSH_PUBLIC_KEY" >>"$DEPLOY_HOME/.ssh/authorized_keys"
	fi
	chown "$DEPLOY_USER:$DEPLOY_GROUP" "$DEPLOY_HOME/.ssh/authorized_keys"
	chmod 0600 "$DEPLOY_HOME/.ssh/authorized_keys"
else
	cat >&2 <<EOF
==> DEPLOY_SSH_PUBLIC_KEY is not set.
    Created $DEPLOY_USER, but did not install an SSH key.
    Root SSH has not been changed.
EOF
fi

echo "==> Preparing deploy repository"
if [[ ! -d "$DEPLOY_REPO_DIR/.git" ]]; then
	install -d -m 0755 -o "$DEPLOY_USER" -g "$DEPLOY_GROUP" "$(dirname "$DEPLOY_REPO_DIR")"
	sudo -u "$DEPLOY_USER" git clone "$REPO_URL" "$DEPLOY_REPO_DIR"
fi
chown -R "$DEPLOY_USER:$DEPLOY_GROUP" "$DEPLOY_REPO_DIR"

echo "==> Preparing application directory"
mkdir -p "$APP_DIR/bin" "$DATA_DIR"
chown -R "$APP_USER:$APP_GROUP" "$DATA_DIR"

echo "==> Building application"
"$REPO_DIR/scripts/build.sh"
install -m 0755 "$REPO_DIR/bin/$BINARY_NAME" "$APP_DIR/bin/$BINARY_NAME"

echo "==> Writing deploy helper"
tee "$DEPLOY_HELPER" >/dev/null <<EOF
#!/usr/bin/env bash
set -euo pipefail

SOURCE_BINARY="$DEPLOY_REPO_DIR/bin/$BINARY_NAME"
TARGET_BINARY="$APP_DIR/bin/$BINARY_NAME"

if [[ ! -x "\$SOURCE_BINARY" ]]; then
	echo "Expected executable binary at \$SOURCE_BINARY" >&2
	exit 1
fi

install -m 0755 "\$SOURCE_BINARY" "\$TARGET_BINARY"
systemctl restart "$APP_NAME.service"
EOF
chown root:root "$DEPLOY_HELPER"
chmod 0755 "$DEPLOY_HELPER"

echo "==> Writing deploy sudoers rule"
tee "/etc/sudoers.d/$APP_NAME-deploy" >/dev/null <<EOF
$DEPLOY_USER ALL=(root) NOPASSWD: $DEPLOY_HELPER
EOF
chmod 0440 "/etc/sudoers.d/$APP_NAME-deploy"
visudo -cf "/etc/sudoers.d/$APP_NAME-deploy"

echo "==> Writing systemd service"
tee "/etc/systemd/system/$APP_NAME.service" >/dev/null <<EOF
[Unit]
Description=DailyDocs web application
After=network.target

[Service]
Type=simple
User=$APP_USER
Group=$APP_GROUP
WorkingDirectory=$APP_DIR
Environment=ADDR=$APP_ADDR
Environment=DB_PATH=$DB_PATH
ExecStart=$APP_DIR/bin/$BINARY_NAME
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

echo "==> Writing Caddy site"
mkdir -p /etc/caddy
cp /etc/caddy/Caddyfile "/etc/caddy/Caddyfile.$(date +%Y%m%d%H%M%S).bak" 2>/dev/null || true
tee /etc/caddy/Caddyfile >/dev/null <<EOF
$DOMAIN {
	reverse_proxy $APP_ADDR
}
EOF

echo "==> Starting services"
systemctl daemon-reload
systemctl enable "$APP_NAME.service"
systemctl restart "$APP_NAME.service"
systemctl enable --now caddy
caddy validate --config /etc/caddy/Caddyfile
systemctl reload caddy

echo "==> Local smoke check"
curl --fail --silent --show-error "http://$APP_ADDR/health"
echo

cat <<EOF

Bootstrap complete.

Application:
  systemctl status $APP_NAME.service
  journalctl -u $APP_NAME.service -f

Deploy user:
  ssh $DEPLOY_USER@$DOMAIN
  cd $DEPLOY_REPO_DIR
  sudo $DEPLOY_HELPER

Public checks:
  https://$DOMAIN
  https://$DOMAIN/health
EOF
