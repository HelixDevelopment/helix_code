# HelixTrack Deployment Guide — 2026-06-21

## Overview

Deployment guide for HelixTrack across all platforms.

---

## Prerequisites

### Development
- Go 1.22+
- Node.js 18+
- Angular CLI 19
- Docker + Docker Compose
- Android Studio (for Android)
- Xcode (for iOS)
- Rust + Tauri CLI (for Desktop)

### Production
- PostgreSQL 12+
- Redis (optional, for caching)
- Nginx (reverse proxy)
- SSL certificate

---

## Backend Deployment

### Development
```bash
cd core/Application
go run main.go
```

### Docker
```bash
cd containers
docker compose -f compose.helixtrack.yml up -d
```

### Production
```bash
# Build
cd core/Application
go build -o htCore main.go

# Run
./htCore --space-root=/data/spaces/_default
```

---

## Web Client Deployment

### Development
```bash
cd web_client
npm install
npm start
# Access at http://localhost:4200
```

### Production Build
```bash
cd web_client
npm run build --configuration=production
# Output in dist/helixtrack-client/
```

### Docker
```bash
# Build image
docker build -t helixtrack-web .

# Run
docker run -p 80:80 helixtrack-web
```

---

## Desktop Client Deployment

### Development
```bash
cd desktop_client
npm install
npm run tauri dev
```

### Build
```bash
cd desktop_client
npm run tauri build
# Output in src-tauri/target/release/
```

### Platforms
- macOS: .dmg
- Windows: .msi
- Linux: .AppImage

---

## Android Client Deployment

### Development
```bash
cd android_client
./gradlew installDebug
```

### Build
```bash
cd android_client
./gradlew assembleRelease
# Output in app/build/outputs/apk/
```

### Play Store
1. Update version in build.gradle
2. Generate signed APK
3. Upload to Play Console

---

## iOS Client Deployment

### Development
```bash
cd ios_client
open HelixTrack.xcodeproj
# Run on simulator
```

### Build
```bash
cd ios_client
xcodebuild -scheme HelixTrack -configuration Release
```

### App Store
1. Update version in Xcode
2. Archive
3. Upload to App Store Connect

---

## Database Setup

### SQLite (Development)
```bash
# Auto-created on first run
# Location: core/Application/Database/Definition.sqlite
```

### PostgreSQL (Production)
```sql
CREATE DATABASE helixtrack;
CREATE USER helixtrack WITH PASSWORD 'your_password';
GRANT ALL PRIVILEGES ON DATABASE helixtrack TO helixtrack;
```

---

## Environment Variables

### Backend
```bash
HT_CONFIG=/path/to/config.json
HT_SPACE_ROOT=/data/spaces/_default
HT_DB_TYPE=sqlite  # or postgres
HT_PG_PASSWORD=your_password
```

### Web Client
```bash
NG_APP_API_URL=http://localhost:8080
NG_APP_WS_URL=ws://localhost:8080/ws
```

---

## Monitoring

### Health Check
```bash
curl http://localhost:8080/health
```

### Logs
```bash
# Backend
docker logs helixtrack-core

# Web Client
# Check browser console
```

---

## Backup

### Database
```bash
# SQLite
cp Definition.sqlite Definition.sqlite.backup

# PostgreSQL
pg_dump helixtrack > backup.sql
```

### Files
```bash
# Spaces data
tar -czf spaces-backup.tar.gz /data/spaces/
```

---

## Cross-references
- [Architecture](/Volumes/T7/Projects/helix_code/docs/helixtrack/ARCHITECTURE.md)
- [API Reference](/Volumes/T7/Projects/helix_code/docs/helixtrack/API_REFERENCE.md)
