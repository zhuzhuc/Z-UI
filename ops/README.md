# Z-UI Production Deployment (VPS)

## 1) Build release package

Run from project root:

```bash
bash scripts/build-release.sh
# or build downloadable archive directly:
scripts/build-download.sh v0.1.0 linux amd64
```

Package output default path:

```bash
release/z-ui-YYYYmmdd-HHMMSS
```

## 2) Upload package to VPS

Assume target path is `/opt/z-ui`:

```bash
rsync -avz release/z-ui-YYYYmmdd-HHMMSS/ root@your-vps:/opt/z-ui/
```

Expected structure:

- `/opt/z-ui/backend/z-ui`
- `/opt/z-ui/front/dist`
- `/opt/z-ui/ops/*`
- `/opt/z-ui/runtime`
- `/opt/z-ui/data`

## 3) Install systemd service

```bash
mkdir -p /etc/z-ui /opt/z-ui/data /opt/z-ui/runtime
cp /opt/z-ui/ops/z-ui.env.example /etc/z-ui/z-ui.env
cp /opt/z-ui/ops/z-ui.service /etc/systemd/system/z-ui.service
systemctl daemon-reload
systemctl enable z-ui
systemctl restart z-ui
systemctl status z-ui
```

Initialize panel account and base URL (auto-generate admin user/password):

```bash
/opt/z-ui/backend/z-ui init https://panel.example.com
```

## 4) Install nginx reverse proxy

```bash
cp /opt/z-ui/ops/nginx-z-ui.conf /etc/nginx/conf.d/z-ui.conf
nginx -t
systemctl reload nginx
```

## 5) Enable HTTPS cert

```bash
mkdir -p /var/www/certbot
certbot certonly --webroot -w /var/www/certbot -d panel.example.com
systemctl reload nginx
```

Or use built-in CLI (3X-ui style one-command issue):

```bash
/opt/z-ui/backend/z-ui ssl issue panel.example.com your@email.com
# no email (not recommended)
/opt/z-ui/backend/z-ui ssl issue panel.example.com
```

Renew certificate:

```bash
/opt/z-ui/backend/z-ui ssl renew
```

## 6) Post-deploy checklist

- Change `PANEL_PASSWORD` and `PANEL_SECRET` in `/etc/z-ui/z-ui.env`.
- You can regenerate random admin credentials anytime with `z-ui init`.
- Confirm DB path exists and writable: `/opt/z-ui/data/zui.db`.
- Confirm runtime writable: `/opt/z-ui/runtime`.
- If using `XRAY_CONTROL=process`, set `XRAY_BIN=/opt/z-ui/bin/xray`.
- If using `XRAY_CONTROL=systemd`, ensure `xray` service is installed.

## 7) Useful commands

```bash
journalctl -u z-ui -f
systemctl restart z-ui
systemctl restart nginx

# diagnostics and backup
/opt/z-ui/backend/z-ui doctor
/opt/z-ui/backend/z-ui backup /opt/z-ui/data/z-ui-backup.zip
/opt/z-ui/backend/z-ui restore /opt/z-ui/data/z-ui-backup.zip
/opt/z-ui/backend/z-ui logs audit 100
```

Audit API:

```bash
curl -H "Authorization: Bearer <token>" "https://panel.example.com/api/v1/audit/logs?limit=100"
```

## 8) Login protection (anti brute-force)

Configure in `/etc/z-ui/z-ui.env`:

- `AUTH_MAX_FAILURES`: max failures before lock (default `5`)
- `AUTH_FAIL_WINDOW_SEC`: rolling window seconds (default `600`)
- `AUTH_LOCK_SEC`: lock duration seconds (default `900`)
