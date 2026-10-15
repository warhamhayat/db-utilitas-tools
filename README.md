# dbprobe

Tool kecil (Go) untuk **menguji koneksi** dan **menjalankan query** ke database:
MySQL/MariaDB, PostgreSQL, Oracle, SQL Server.

Hanya dua kemampuan:

- **`connectivity`** — cek bisa connect (ping + latency).
- **`query`** — jalankan SQL yang kamu tulis sendiri.

Tidak ada SQL di dalam kode. Butuh cek lain (daftar tabel, jumlah koneksi, daftar
user, versi, dll.)? Tulis sendiri sebagai `query_checks` di config.

---

## Install

### Cara 1 — unduh binary (tanpa Go)

Prebuilt yang disediakan: **Linux amd64** dan **Windows amd64** (agar rilis ringan).
Binary-nya **standalone** — sekali unduh langsung bisa dipakai, tanpa installer.

```bash
# Linux amd64
curl -fsSL https://github.com/warhamhayat/db-utilitas-tools/releases/latest/download/dbprobe_linux_amd64 -o dbprobe
chmod +x dbprobe
sudo mv dbprobe /usr/local/bin/

dbprobe engines
```

Windows: unduh `dbprobe_windows_amd64.exe` dari halaman Releases.
Setiap rilis menyertakan `SHA256SUMS` untuk verifikasi.

**Installer otomatis** (Linux/macOS) — deteksi OS/arch lalu unduh binary yang cocok.
Kalau platform-mu tidak punya prebuilt (mis. macOS), ia coba **build dari source**
bila Go terpasang:

```bash
curl -fsSL https://github.com/warhamhayat/db-utilitas-tools/releases/latest/download/install.sh | sh
```

### Cara 2 — build dari source (butuh Go 1.26+)

```bash
cd DB
go build -o dbprobe ./cmd/dbprobe
./dbprobe engines
# Engine : mysql, oracle, postgres, sqlserver
# Checks : connectivity, query
```

> Oracle memakai driver `go-ora` (pure Go) — tidak perlu Oracle Instant Client.

---

## Cara pakai

**1. Buat config:**

```yaml
# config.yaml
targets:
  - name: my-db
    engine: mysql
    dsn: "user:${MYSQL_PASSWORD}@tcp(127.0.0.1:3306)/appdb"
    checks: [connectivity, query]
    query_checks:
      - name: "list tabel"
        sql: "SHOW TABLES"
      - name: "jumlah koneksi"
        sql: "SELECT COUNT(*) AS n FROM information_schema.processlist"
```

**2. Jalankan:**

```bash
export MYSQL_PASSWORD=secret
./dbprobe check --config config.yaml
```

Output: tabel status per check; hasil query ditampilkan di bawahnya.

```
TARGET  CHECK                  STATUS  MESSAGE
my-db   connectivity           OK      koneksi OK
my-db   query:list tabel       OK      12 baris
my-db   query:jumlah koneksi   OK      1 baris

Summary: 3 OK · 0 WARN · 0 SKIP · 0 FAIL
```

**Cek koneksi saja:**

```bash
./dbprobe ping --config config.yaml
```

**Query cepat tanpa menulis di config:**

```bash
./dbprobe query --config config.yaml --target my-db --sql "SHOW TABLES" --format table
./dbprobe query --config config.yaml --target my-db --file my-query.sql
```

**Mode CI** (exit `1` bila ada `FAIL`):

```bash
./dbprobe check --config config.yaml --ci
```

---

## Perintah

| Perintah | Fungsi |
|---|---|
| `check` | jalankan check untuk semua target |
| `ping` | hanya `connectivity` |
| `query` | query ad-hoc ke satu target |
| `engines` | tampilkan daftar engine & check |

Flag penting:

- `check` → `--config` `--target` `--checks` `--format table\|json` `--out` `--ci` `--allow-write` `--timeout`
- `query` → `--config` `--target` `--sql` `--file` `--format json\|table` `--max-rows` `--allow-write`
- `ping` → `--config` `--target` `--format`

Exit code: `0` OK · `1` ada `FAIL` (`--ci`) atau query ad-hoc gagal · `2` error konfigurasi.

---

## Konfigurasi

Root YAML wajib punya key `targets`.

### Target

| field | wajib | keterangan |
|---|---|---|
| `name` | ✅ | nama target |
| `engine` | ✅ | `mysql` \| `postgres` \| `oracle` \| `sqlserver` |
| `dsn` | ✅ | connection string (mendukung `${ENV_VAR}`) |
| `password` | ❌ | password terpisah (di-encode otomatis) |
| `checks` | ❌ | default `[connectivity, query]` |
| `query_checks` | ❌ | query custom (opsional) |

### `query_checks`

| field | wajib | keterangan |
|---|---|---|
| `name` | ❌ | label query |
| `sql` \| `file` | ✅ | salat satu; `file` relatif ke file config |
| `params` | ❌ | parameter bernama, mis. `{ dx: "impaksi" }` |
| `timeout` | ❌ | default `10s` |
| `max_rows` | ❌ | default `1000` |
| `readonly` | ❌ | default `true` |
| `expect` | ❌ | validasi hasil (opsional) |

`expect`: `row_count`, `row_count_op` (`=`,`>`,`>=`,`<`,`<=`), `cell`
(`{ column, row, op, value }` dengan op `=`,`!=`,`>`,`<`,`>=`,`<=`,`contains`,`regex`),
`max_duration` (mis. `2s`).

```yaml
      - name: "hitung impaksi"
        sql: "SELECT COUNT(*) AS n FROM pasien WHERE diagnosa = :dx"
        params: { dx: "impaksi" }
        expect:
          cell: { column: "n", op: ">=", value: 1 }
```

Parameter `:nama` otomatis menjadi placeholder per engine: `?` (MySQL),
`$1` (PostgreSQL), `:1` (Oracle), `@p1` (SQL Server).

---

## Password & karakter khusus

DSN bergaya URL (`scheme://`) diperlakukan sebagai URL, dan `#` memotong password.
Kalau password punya karakter khusus, tulis di field `password` (di-encode otomatis):

```yaml
    dsn: "oracle://user@host:1521/svc"
    password: "${ORACLE_PASSWORD}"
```

Kalau password hanya berisi huruf/angka/simbol aman (`-._~!$&'()*+,;=`), boleh
langsung di DSN. DSN **MySQL bukan URL**, jadi `#`, `!`, `^` aman tanpa encoding.

---

## Docker & Kubernetes

**Build image:**

```bash
docker build -t dbprobe:latest .
docker run --rm -it --network host -v "$PWD/configs:/config:ro" \
  dbprobe:latest check --config /config/example.yaml
```

**Troubleshooting di dalam cluster** (mirip `kubectl run -it busybox`) — sisipkan
dbprobe ke pod aplikasi agar berbagi network namespace:

```bash
kubectl debug -it <POD_APP> --image=registry/tools/dbprobe:latest \
  --target=<CONTAINER_APP> -- sh
```

Di dalam shell (ephemeral container tidak bisa mount volume), tulis config kecil lalu jalankan:

```sh
cat > /tmp/db.yaml <<'YAML'
targets:
  - name: app-db
    engine: mysql
    dsn: "app_user:${MYSQL_PASSWORD}@tcp(mysql-svc.default.svc.cluster.local:3306)/appdb"
    checks: [connectivity]
YAML
export MYSQL_PASSWORD='...'
dbprobe check --config /tmp/db.yaml
```

Alternatif pod debug terpisah (mount ConfigMap + Secret) ada di
[`deploy/dbprobe-debug.yaml`](deploy/dbprobe-debug.yaml).
