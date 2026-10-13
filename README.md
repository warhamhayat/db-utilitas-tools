# dbprobe

CLI + library Go untuk **uji koneksi & menjalankan query** ke berbagai database
(MySQL/MariaDB, PostgreSQL, Oracle, SQL Server).

dbprobe sengaja dibuat **sederhana** — hanya punya dua kemampuan:

1. **`connectivity`** — memastikan bisa connect ke database (ping + latency).
2. **`query`** — menjalankan SQL yang **kamu tulis sendiri**.

Tidak ada SQL yang di-hardcode di dalam kode. Semua kebutuhan lain — daftar tabel,
jumlah koneksi, daftar user, max connections, versi server, dsb. — ditulis sebagai
`query_checks` di file config. Mau cek apa pun, tinggal tulis query-nya.

---

## Kebutuhan & Build

- **Go 1.26+** (lihat `go.mod`).
- Untuk Oracle dipakai driver `go-ora` (pure Go) — **tidak** perlu Oracle Instant Client.

```bash
cd DB
go build -o dbprobe ./cmd/dbprobe
```

```bash
./dbprobe engines
# Engine : mysql, oracle, postgres, sqlserver
# Checks : connectivity, query
# Default: connectivity, query
```

---

## Cara Pakai

### 1. Buat config, lalu jalankan `check`

```yaml
# configs/example.yaml
targets:
  - name: local-mysql
    engine: mysql
    dsn: "user:${MYSQL_PASSWORD}@tcp(127.0.0.1:3306)/appdb"
    checks: [connectivity, query]
    query_checks:
      - name: "list tabel"
        sql: "SHOW TABLES"
      - name: "jumlah koneksi"
        sql: "SELECT COUNT(*) AS n FROM information_schema.processlist"
```

```bash
export MYSQL_PASSWORD=secret
./dbprobe check --config configs/example.yaml
```

Contoh output:

```
TARGET       CHECK                  STATUS  MESSAGE
local-mysql  connectivity           OK      koneksi OK
local-mysql  query:list tabel       OK      12 baris
local-mysql  query:jumlah koneksi   OK      1 baris

Summary: 3 OK · 0 WARN · 0 SKIP · 0 FAIL
```

- Hasil query dirender sebagai tabel di bawah ringkasan.
- Bila tidak ada `query_checks`, check `query` akan `SKIP`.

### 2. Uji koneksi saja (`ping`)

```bash
./dbprobe ping --config configs/minimal.yaml   # = check --checks connectivity
```

### 3. Query ad-hoc (tanpa tulis di config)

```bash
# output default: json
./dbprobe query --config configs/example.yaml --target local-mysql \
  --sql "SELECT table_name FROM information_schema.tables"

# baca query dari file .sql (relatif ke direktori kerja)
./dbprobe query --config configs/example.yaml --target local-mysql --file my-query.sql

# tampilan tabel
./dbprobe query --config configs/example.yaml --target local-mysql --format table \
  --sql "SHOW TABLES"

# izinkan DML/DDL (default read-only)
./dbprobe query --config configs/example.yaml --target local-mysql --allow-write \
  --sql "INSERT INTO pasien (nik) VALUES ('444')"

# perbesar batas baris
./dbprobe query --config configs/example.yaml --target local-mysql --max-rows 5000 \
  --sql "SELECT table_name FROM information_schema.tables"
```

### 4. Mode CI

```bash
./dbprobe check --config configs/example.yaml --ci; echo "exit=$?"
```

---

## CLI Reference

```text
dbprobe [command]

Commands:
  check     Jalankan check pada target
  ping      Uji konektivitas saja
  query     Jalankan query custom ke satu target (ad-hoc)
  engines   Tampilkan engine dan check yang tersedia
```

### `dbprobe check`

| Flag | Default | Keterangan |
|---|---|---|
| `--config` | `configs/example.yaml` | path file konfigurasi |
| `--target` | — | hanya jalankan untuk target bernama ini |
| `--checks` | dari config / default | daftar check, mis. `connectivity,query` |
| `--format` | `table` | `table` atau `json` |
| `--out` | stdout | tulis output ke file |
| `--no-color` | `false` | matikan warna |
| `--ci` | `false` | exit `1` bila ada `FAIL` |
| `--allow-write` | `false` | izinkan query non-`SELECT` |
| `--timeout` | `0` (tanpa batas) | timeout per check, mis. `15s` |

### `dbprobe ping`

Flag: `--config`, `--target`, `--format`, `--no-color`.

### `dbprobe query`

| Flag | Default | Keterangan |
|---|---|---|
| `--config` | `configs/example.yaml` | path file konfigurasi |
| `--target` | — | nama target di config (**wajib**) |
| `--sql` | — | query SQL (atau pakai `--file`) |
| `--file` | — | baca query dari file `.sql` |
| `--format` | `json` | `table` atau `json` |
| `--max-rows` | `0` | batas baris hasil (`0` = default `1000`) |
| `--allow-write` | `false` | izinkan query non-`SELECT` |
| `--no-color` | `false` | matikan warna |

> Tidak ada flag `--threshold`.

---

## Konfigurasi

Config contoh: `configs/example.yaml`, `configs/minimal.yaml`, `configs/oracle.yaml`,
`configs/portaverse.yaml`.

Struktur root YAML (wajib key `targets`):

```yaml
targets:
  - name: ...
    engine: ...
    dsn: ...
```

### Struktur target — wajib / opsional

| Field | Wajib? | Keterangan |
|---|---|---|
| `name` | ✅ | nama target |
| `engine` | ✅ | `mysql` \| `postgres` \| `oracle` \| `sqlserver` |
| `dsn` | ✅ | connection string (mendukung `${ENV_VAR}`) |
| `password` | ❌ | password terpisah; di-encode otomatis (lihat bawah) |
| `checks` | ❌ | daftar check; kosong = default `[connectivity, query]` |
| `query_checks` | ❌ | query custom + validasi (opsional) |

### `query_checks` — field

| Field | Wajib? | Keterangan |
|---|---|---|
| `name` | ❌ | opsional, tapi disarankan |
| `sql` | ✅\* | query SQL |
| `file` | ✅\* | alternatif `sql` (baca dari file `.sql`, relatif ke lokasi config) |
| `params` | ❌ | parameter bernama, mis. `{ dx: "impaksi" }` |
| `timeout` | ❌ | default `10s` |
| `max_rows` | ❌ | default `1000` |
| `readonly` | ❌ | default `true`; `--allow-write` membaliknya |
| `expect` | ❌ | bila kosong, query hanya dijalankan tanpa divalidasi |

\* salah satu dari `sql` atau `file` harus ada.

### `expect` — validasi hasil (opsional)

| Field | Keterangan |
|---|---|
| `row_count` | jumlah baris yang diharapkan |
| `row_count_op` | operator (`=`, `>`, `>=`, `<`, `<=`); default `=` |
| `cell` | validasi satu sel: `{ column, row, op, value }` |
| `max_duration` | batas latency query, mis. `2s` |

Operator `cell.op`: `=`, `!=`, `>`, `<`, `>=`, `<=`, `contains`, `regex`.

```yaml
      - name: "hitung impaksi"
        sql: "SELECT COUNT(*) AS n FROM pasien WHERE diagnosa = :dx"
        params: { dx: "impaksi" }
        expect:
          cell: { column: "n", op: ">=", value: 1 }
```

### Placeholder per engine

Parameter bernama (`:nama`) di SQL otomatis menjadi placeholder engine:

| Engine | Placeholder |
|---|---|
| `mysql` | `?` |
| `postgres` | `$1` |
| `oracle` | `:1` |
| `sqlserver` | `@p1` |

Cast PostgreSQL `::text` tidak ikut berubah.

---

## Password & karakter khusus

Ada dua cara menulis password:

**Cara 1 — langsung di DSN** (cocok bila password tidak punya karakter khusus):

```yaml
    dsn: "oracle://system:Secret-2024@127.0.0.1:1521/FREEPDB1"
```

**Cara 2 — field `password`** (aman untuk karakter khusus, tanpa encode manual):

```yaml
    dsn: "oracle://app_user@db.example.com:1521/ORCLPDB1"
    password: "${ORACLE_PASSWORD}"     # ditulis apa adanya
```

DSN bergaya URL (`scheme://`) diperlakukan sebagai **URL**. Karakter `#` menandai awal
*fragment*, sehingga password terpotong. Karakter yang bermasalah di DSN URL:

| Kategori | Karakter | Saran |
|---|---|---|
| ❌ gagal parse | `#` `/` `?` spasi `^` `[` `]` `{` `}` `\|` `\` `"` `<` `>` | pakai field `password` (atau encode, `#` → `%23`) |
| ⚠️ ambigu | `%` `@` `:` | pakai field `password` |
| ✅ aman | `A-Za-z0-9` `- . _ ~ ! $ & ' ( ) * + , ; =` | boleh langsung di DSN |

- Perlu field `password` → contoh `S1ng#M4mV*-rs` (ada `#`).
- Tidak perlu → contoh `Secret-2024`.
- **DSN MySQL bukan URL**, jadi `#`, `!`, `^` aman.

Kalau DSN URL gagal di-parse, dbprobe menampilkan `FAIL` pada check `dsn` + hint encoding.

---

## Contoh: cek apa pun lewat query

```yaml
    query_checks:
      - name: "list tabel"
        sql: "SHOW TABLES"
      - name: "koneksi saat ini"
        sql: "SELECT COUNT(*) AS n FROM information_schema.processlist"
      - name: "max connections"
        sql: "SHOW VARIABLES LIKE 'max_connections'"
      - name: "list user"
        sql: "SELECT user, host FROM mysql.user"
```

Padanan engine lain:

| Kebutuhan | PostgreSQL | Oracle | SQL Server |
|---|---|---|---|
| list tabel | `SELECT table_name FROM information_schema.tables WHERE table_schema='public'` | `SELECT table_name FROM user_tables` | `SELECT name FROM sys.tables` |
| jumlah koneksi | `SELECT count(*) AS n FROM pg_stat_activity` | `SELECT COUNT(*) AS n FROM v$session WHERE type='USER'` | `SELECT COUNT(*) AS n FROM sys.dm_exec_sessions` |
| versi | `SELECT version() AS v` | `SELECT version FROM product_component_version WHERE ROWNUM=1` | `SELECT @@VERSION` |

---

## Uji cepat dengan Docker

### MySQL

```bash
docker run --rm -d --name dbprobe-mysql \
  -e MYSQL_ROOT_PASSWORD=secret -p 3306:3306 mysql:8.4

until docker exec dbprobe-mysql mysqladmin ping -h127.0.0.1 -uroot -psecret --silent; do sleep 2; done

docker exec -i dbprobe-mysql mysql -uroot -psecret <<'SQL'
CREATE DATABASE IF NOT EXISTS appdb;
USE appdb;
CREATE TABLE IF NOT EXISTS pasien (
  id INT PRIMARY KEY AUTO_INCREMENT,
  nik VARCHAR(16) NOT NULL,
  diagnosa VARCHAR(64) NOT NULL
);
INSERT INTO pasien (nik, diagnosa) VALUES ('111','impaksi'), ('222','lain');
SQL
```

```yaml
targets:
  - name: local-mysql
    engine: mysql
    dsn: "root:${MYSQL_PASSWORD}@tcp(127.0.0.1:3306)/appdb"
    checks: [connectivity, query]
    query_checks:
      - name: "list tabel"
        sql: "SHOW TABLES"
      - name: "hitung impaksi"
        sql: "SELECT COUNT(*) AS n FROM pasien WHERE diagnosa = :dx"
        params: { dx: "impaksi" }
        expect: { cell: { column: "n", op: ">=", value: 1 } }
```

### PostgreSQL

```bash
docker run --rm -d --name dbprobe-pg \
  -e POSTGRES_PASSWORD=secret -e POSTGRES_DB=appdb -p 5432:5432 postgres:16
until docker exec dbprobe-pg pg_isready -U postgres >/dev/null 2>&1; do sleep 2; done
```

```yaml
targets:
  - name: local-pg
    engine: postgres
    dsn: "postgres://postgres:${PG_PASSWORD}@127.0.0.1:5432/appdb?sslmode=disable"
    checks: [connectivity, query]
    query_checks:
      - name: "verifikasi"
        sql: "SELECT 1 AS ok"
        expect: { cell: { column: "ok", op: "=", value: 1 } }
```

### Oracle

```bash
docker run --rm -d --name dbprobe-oracle \
  -p 1521:1521 -e ORACLE_PASSWORD=secret gvenzl/oracle-free:23-slim
# siap memakan waktu beberapa menit: docker logs -f dbprobe-oracle
```

```yaml
targets:
  - name: oracle-local
    engine: oracle
    dsn: "oracle://system:${ORACLE_PASSWORD}@127.0.0.1:1521/FREEPDB1"
    checks: [connectivity, query]
    query_checks:
      - name: "versi"
        sql: "SELECT version FROM product_component_version WHERE ROWNUM = 1"
```

> Autonomous Database (wallet): `dsn: "oracle://admin@mydb_high?configDir=./wallet"` + `password: "${ORACLE_PASSWORD}"`.

### SQL Server

```bash
docker run --rm -d --name dbprobe-mssql -p 1433:1433 \
  -e ACCEPT_EULA=Y -e 'MSSQL_SA_PASSWORD=Str0ng!Pass' \
  mcr.microsoft.com/mssql/server:2022-latest
```

```yaml
targets:
  - name: mssql-local
    engine: sqlserver
    dsn: "sqlserver://sa:${MSSQL_PASSWORD}@127.0.0.1:1433?database=master"
    checks: [connectivity, query]
```

### Graceful degradation (query gagal → SKIP, bukan crash)

```bash
docker exec -i dbprobe-mysql mysql -uroot -psecret <<'SQL'
CREATE USER IF NOT EXISTS 'lowpriv'@'%' IDENTIFIED BY 'lowpass';
GRANT SELECT ON appdb.* TO 'lowpriv'@'%';
FLUSH PRIVILEGES;
SQL
```

```yaml
targets:
  - name: mysql-lowpriv
    engine: mysql
    dsn: "lowpriv:lowpass@tcp(127.0.0.1:3306)/appdb"
    checks: [connectivity, query]
    query_checks:
      - name: "list user (butuh privilege)"
        sql: "SELECT user, host FROM mysql.user"
```

Hasil: `connectivity` **OK**, `query:list user` **SKIP** + hint (tanpa akses `mysql.user`).
Proses tetap selesai normal.

### Bersih-bersih

```bash
docker stop dbprobe-mysql dbprobe-pg dbprobe-oracle dbprobe-mssql 2>/dev/null
unset MYSQL_PASSWORD PG_PASSWORD ORACLE_PASSWORD MSSQL_PASSWORD
```

---

## Docker image & Kubernetes

Menjalankan dbprobe sebagai container — berguna untuk **troubleshooting di dalam
cluster**, di dekat aplikasi (mirip `kubectl run -it busybox`).

### Build image

```bash
cd DB
docker build -t dbprobe:latest .

# multi-arch + push ke registry (kebanyakan node k8s = linux/amd64)
docker buildx build --platform linux/amd64 \
  -t registry.example.com/tools/dbprobe:latest --push .
```

### Jalan lokal

```bash
docker run --rm -it --network host \
  -e MYSQL_PASSWORD=secret \
  -v "$PWD/configs:/config:ro" \
  dbprobe:latest check --config /config/example.yaml
```

> `--network host` (Linux) agar bisa menjangkau DB di `localhost` host. Di
> macOS/Windows pakai `host.docker.internal` di DSN, tanpa `--network host`.
> Password bisa dioper lewat `-e`.

### Kubernetes

#### A. Ephemeral container di pod aplikasi (paling mirip "busybox")

`kubectl debug` menyisipkan container sementara ke pod yang sudah berjalan.
Network namespace-nya = milik pod itu, jadi menguji jalur jaringan **persis
seperti aplikasi melihatnya**.

```bash
kubectl debug -it <POD_APP> \
  --image=registry.example.com/tools/dbprobe:latest \
  --target=<CONTAINER_APP> \
  -- sh
```

Di dalam shell — ephemeral container tidak bisa mount volume, jadi tulis config
kecil dulu:

```sh
cat > /tmp/db.yaml <<'YAML'
targets:
  - name: app-db
    engine: mysql
    dsn: "app_user:${MYSQL_PASSWORD}@tcp(mysql-svc.default.svc.cluster.local:3306)/appdb"
    checks: [connectivity, query]
    query_checks:
      - name: "list tabel"
        sql: "SHOW TABLES"
YAML

export MYSQL_PASSWORD='...'
dbprobe check --config /tmp/db.yaml
```

#### B. Pod debug terpisah (mount ConfigMap + Secret)

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: dbprobe-debug
spec:
  restartPolicy: Never
  containers:
    - name: dbprobe
      image: registry.example.com/tools/dbprobe:latest
      args: ["check", "--config", "/config/db.yaml"]
      env:
        - name: MYSQL_PASSWORD
          valueFrom:
            secretKeyRef:
              name: app-db-secret
              key: password
      volumeMounts:
        - { name: config, mountPath: /config, readOnly: true }
  volumes:
    - name: config
      configMap:
        name: dbprobe-config
```

```bash
kubectl apply -f dbprobe-debug.yaml
kubectl logs -f dbprobe-debug
```

#### C. Catatan

- Karena pod berjalan di dalam cluster, nama service
  (`mysql-svc.namespace.svc.cluster.local`) di-resolve seperti yang dilihat
  aplikasi — inilah yang membuat dbprobe berguna untuk troubleshooting jaringan DB.
- Password sebaiknya dari Secret (`valueFrom.secretKeyRef`), bukan ditulis di YAML.
- Image dibuat `linux/amd64` (umumnya node k8s); sesuaikan bila node arm64.

---

## Status hasil & exit code

| Status | Arti |
|---|---|
| `OK` | berhasil |
| `WARN` | berhasil tapi perlu diperhatikan (mis. timeout, objek tidak ada) |
| `SKIP` | tidak dijalankan (permission / belum ada `query_checks`) + hint |
| `FAIL` | gagal (mis. tidak bisa connect, query error) |

Exit code: `0` tidak ada `FAIL` · `1` ada `FAIL` (`--ci`) atau query ad-hoc gagal · `2` error konfigurasi.

---

## Arsitektur

```
cmd/dbprobe        entrypoint CLI (cobra)
pkg/config         load YAML + ekspansi env + muat file SQL
pkg/engine         abstraksi Engine + registry (Open + Placeholder per engine)
pkg/check          hanya 2 check: connectivity & query
pkg/probe          orkestrasi: jalan + isolasi error (recover) per target/check
pkg/report         formatter table/json
pkg/model          Result/Status/Report
```

`pkg/check/query.go` engine-agnostic: bekerja di atas `*sql.DB`, jadi berlaku untuk
semua engine tanpa kode tambahan.

---

## Troubleshooting

| Gejala | Kemungkinan | Solusi |
|---|---|---|
| `cannot unmarshal !!seq into config.Config` | `targets:` tidak ada di YAML | tulis config sebagai `targets: [...]` |
| `connection refused` | DB belum siap / port salah | tunggu container siap; `docker ps` |
| `query` = `SKIP` | tidak punya privilege | normal; beri grant bila perlu |
| `ORA-00942` | no privilege (Oracle) atau objek tidak ada | sesuaikan query/grant |
| DSN URL gagal parse | karakter khusus belum di-encode | pakai field `password` |
| `${VAR}` kosong | env var belum di-export | `export NAMA_PASSWORD=...` |
| query `SKIP` "bukan SELECT" | mode read-only | `--allow-write` atau `readonly: false` |
