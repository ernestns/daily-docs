#!/usr/bin/env bash
set -euo pipefail

REMOTE="${REMOTE:-remote}"
REPO_DIR="${REPO_DIR:-~/daily-docs}"
BRANCH="${BRANCH:-main}"
APP_NAME="${APP_NAME:-dailydocs}"
DEPLOY_HELPER="${DEPLOY_HELPER:-/usr/local/bin/$APP_NAME-install}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8080/health}"

# shellcheck disable=SC2029
ssh "$REMOTE" \
	"REPO_DIR='$REPO_DIR' BRANCH='$BRANCH' APP_NAME='$APP_NAME' DEPLOY_HELPER='$DEPLOY_HELPER' HEALTH_URL='$HEALTH_URL' bash -s" <<'EOF'
set -euo pipefail

case "$REPO_DIR" in
	"~")
		REPO_DIR="$HOME"
		;;
	"~/"*)
		REPO_DIR="$HOME/${REPO_DIR#"~/"}"
		;;
esac

cd "$REPO_DIR"

echo "==> Updating source"
git fetch origin "$BRANCH"
git checkout "$BRANCH"
git pull --ff-only origin "$BRANCH"

echo "==> Building application"
./scripts/build.sh

echo "==> Installing binary and restarting service"
sudo "$DEPLOY_HELPER"

echo "==> Waiting for health check"
for _ in 1 2 3 4 5; do
	if curl --fail --silent --show-error "$HEALTH_URL"; then
		echo
		echo "Deploy complete."
		exit 0
	fi
	sleep 1
done

echo "Health check failed after restart." >&2
systemctl status "$APP_NAME.service" --no-pager >&2 || true
exit 1
EOF
