# Z-UI

一个接近 `3X-UI` 体验的 Xray/V2Ray 管理面板（Go 后端 + React 前端）。

## 快速启动（本地）

```bash
./start.sh
```

- 面板：`http://127.0.0.1:8081/login.html`
- 后端健康检查：`http://127.0.0.1:8081/api/v1/health`

## 像 3X-UI 一样“下载发布包”

构建可分发的压缩包（`tar.gz`）：

```bash
# 用法: scripts/build-download.sh <version> <os> <arch>
scripts/build-download.sh v0.1.0 linux amd64
```

输出：

- `release/z-ui-v0.1.0-linux-amd64.tar.gz`
- `release/z-ui-v0.1.0-linux-amd64.sha256`

你可以把这个压缩包上传到服务器或 GitHub Release，让用户直接下载。

## 服务器部署（VPS）

完整步骤见：`ops/README.md`

包含：

- `systemd` 服务模板
- `nginx` 反代模板
- `ssl issue/renew` 一键命令
- `doctor/backup/restore` 运维命令

## 常用 CLI

```bash
# 初始化账号与面板地址（自动生成用户密码）
./backend/z-ui init https://panel.example.com

# 申请与续期证书
./backend/z-ui ssl issue panel.example.com your@email.com
./backend/z-ui ssl renew

# 运维诊断与备份
./backend/z-ui doctor
./backend/z-ui backup /opt/z-ui/data/z-ui-backup.zip
./backend/z-ui restore /opt/z-ui/data/z-ui-backup.zip
```

