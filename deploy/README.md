# Z-UI VPS Deployment

## 1. Build and copy files

```bash
cd /opt/z-ui/backend
go build -o z-ui .
mkdir -p /opt/z-ui/data
```

Copy frontend files to `/opt/z-ui/front`.

## 2. Install systemd service

```bash
cp /opt/z-ui/deploy/z-ui.service /etc/systemd/system/z-ui.service
systemctl daemon-reload
systemctl enable z-ui
systemctl restart z-ui
systemctl status z-ui
```

## 3. Configure Nginx

```bash
cp /opt/z-ui/deploy/nginx-z-ui.conf /etc/nginx/conf.d/z-ui.conf
nginx -t
systemctl reload nginx
```

## 4. HTTPS certificate

Use certbot:

```bash
mkdir -p /var/www/certbot
certbot certonly --webroot -w /var/www/certbot -d panel.example.com
systemctl reload nginx
```

## 5. Security checklist

- Change `PANEL_PASSWORD` and `PANEL_SECRET` in `z-ui.service`.
- Restrict panel access by firewall if possible.
- Keep Xray and system packages updated.
- Backup `/opt/z-ui/data/zui.db` regularly.

## 6. z-ui CLI operations

```bash
# Show service/runtime summary
/opt/z-ui/backend/z-ui status

# Change panel username/password (stored in DB)
/opt/z-ui/backend/z-ui set-user myadmin
/opt/z-ui/backend/z-ui set-pass 'MyStrongPassword'

# Generate self-signed cert (if you need temporary certs)
/opt/z-ui/backend/z-ui ssl self-signed panel.example.com /etc/z-ui/ssl/z-ui.crt /etc/z-ui/ssl/z-ui.key

# Basic download speed test
/opt/z-ui/backend/z-ui speedtest
```
