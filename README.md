# Shortlink Service (Go + MySQL + Redis)

## Features

- Generate short code and store mapping in MySQL
- Resolve code to long URL (HTTP 302 redirect)
- MySQL **UNIQUE(code)** index for fast lookup
- Redis Cache-Aside:
  - cache hit -> return
  - cache miss -> query DB -> fill cache
  - **cache penetration**: cache null (`__NULL__`) with short TTL

---

## Tech Stack

- Go, Gin
- MySQL 8.0
- Redis 7
- Docker Compose

---

## Quick Start (Docker Compose)

```bash
docker compose up -d --build
```

## Service:

- API: http://localhost:8080
- MySQL exposed on localhost:3307 (container internal port is still 3306)
- Redis exposed on localhost:6379

### POST /shorten

```powershell
$body = @{ long_url = "https://example.com/abc" } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/shorten" -ContentType "application/json" -Body $body
```

Example response:

```txt
code short_url
GMUQ1F1Z http://localhost:8080/r/GMUQ1F1Z
```

### GET /r/{code} (302 redirect)

```powershell
Invoke-WebRequest -Uri "http://localhost:8080/r/GMUQ1F1Z" -MaximumRedirection 0 -UseBasicParsing
```

Expected:

StatusCode: 302
Location: https://example.com/abc

## MySQL Index & EXPLAIN Proof

Core query:

```sql
EXPLAIN SELECT long_url FROM short_links WHERE code='GMUQ1F1Z' LIMIT 1;
```

Result:

```txt
+----+-------------+-------------+------------+-------+---------------+---------+---------+-------+------+----------+-------+
| id | select_type | table | partitions | type | possible_keys | key | key_len | ref | rows | filtered | Extra |
+----+-------------+-------------+------------+-------+---------------+---------+---------+-------+------+----------+-------+
| 1 | SIMPLE | short_links | NULL | const | uk_code | uk_code | 66 | const | 1 | 100.00 | NULL |
+----+-------------+-------------+------------+-------+---------------+---------+---------+-------+------+----------+-------+
```

Interpretation:

key=uk_code, type=const, rows=1 → optimal index usage with fast lookup.

### Simulate missing/invalid index (IGNORE INDEX)

```sql
EXPLAIN SELECT long_url FROM short_links IGNORE INDEX (uk_code)
WHERE code='GMUQ1F1Z' LIMIT 1;
```

Result:

```txt
+----+-------------+-------------+------------+------+---------------+------+---------+------+------+----------+-------------+
| id | select_type | table | partitions | type | possible_keys | key | key_len | ref | rows | filtered | Extra |
+----+-------------+-------------+------------+------+---------------+------+---------+------+------+----------+-------------+
| 1 | SIMPLE | short_links | NULL | ALL | NULL | NULL | NULL | NULL | 1 | 100.00 | Using where |
+----+-------------+-------------+------------+------+---------------+------+---------+------+------+----------+-------------+
```

Interpretation:

type=ALL with key=NULL indicates a full table scan, which we aim to avoid.

## Redis Cache Penetration Protection (Cache Null)

For non-existing codes, cache a null marker (**NULL**) with short TTL to prevent cache penetration.

Repro:

```powershell
Invoke-WebRequest -Uri "http://localhost:8080/r/NOEXIST1234" -UseBasicParsing
```

Then check Redis:

```bash
docker exec -it shortlink-redis-1 redis-cli
GET sl:code:NOEXIST1234
TTL sl:code:NOEXIST1234
```

Example output:

- GET -> "**NULL**"
- TTL -> 5 (short TTL, e.g. <= 30s)
