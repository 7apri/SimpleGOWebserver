# Panels – Backend

Backendový server aplikace Panels napsaný v jazyce **Go**. Poskytuje REST API, správu uživatelů, autentizaci, data o počasí a poloze, analytiku a emailové notifikace.
copilot byl využit jen při vytváření tohoto readme

---

## Obsah

- [Použité technologie](#použité-technologie)
- [Architektura a struktura projektu](#architektura-a-struktura-projektu)
- [Databázové schéma](#databázové-schéma)
- [Funkcionalita](#funkcionalita)
  - [Autentizace a správa uživatelů](#autentizace-a-správa-uživatelů)
  - [Dvoufaktorová autentizace (2FA)](#dvoufaktorová-autentizace-2fa)
  - [OAuth2 přihlášení](#oauth2-přihlášení)
  - [Služba polohy](#služba-polohy)
  - [Služba počasí](#služba-počasí)
  - [Analytika a logování](#analytika-a-logování)
  - [Email](#email)
  - [Internacionalizace (i18n)](#internacionalizace-i18n)
  - [Šablony](#šablony)
  - [Cache](#cache)
  - [Omezení rychlosti (Rate limiting)](#omezení-rychlosti-rate-limiting)
  - [CSRF ochrana](#csrf-ochrana)
  - [WebSocket](#websocket)
- [API endpointy](#api-endpointy)
- [Bezpečnost](#bezpečnost)
- [Nasazení (Docker)](#nasazení-docker)
- [Proměnné prostředí](#proměnné-prostředí)

---

## Použité technologie

### Jazyk a runtime

| Technologie | Verze | Účel |
|---|---|---|
| **Go** | 1.25.5 | Hlavní programovací jazyk serveru |

### Databáze a úložiště

| Technologie | Verze | Účel |
|---|---|---|
| **PostgreSQL** | (s rozšířeními) | Hlavní relační databáze |
| **PostGIS** | – | Geospatální dotazy a indexy pro lokace |
| **pg_trgm** | – | Trigram fulltext vyhledávání názvů měst |
| **fuzzystrmatch** | – | Fuzzy shoda řetězců |
| **citext** | – | Porovnávání textu bez ohledu na velikost písmen (e-mail, uživatelské jméno) |
| **pgcrypto** | – | Generování UUID v databázi |
| **Redis** | 7-alpine | In-memory úložiště pro sessiony, 2FA pendingová data a rate-limiting čítače |

### Infrastruktura a nasazení

| Technologie | Verze | Účel |
|---|---|---|
| **Docker / Docker Compose** | – | Kontejnerizace všech služeb |
| **Nginx** (fholzer/nginx-brotli) | – | Reverzní proxy, servírování statických souborů, Brotli komprese, TLS |
| **Mailpit** | latest | Lokální SMTP server pro vývoj a testování emailů |

### Go knihovny (přímé závislosti)

| Balíček | Účel |
|---|---|
| `bytedance/sonic` | Vysokovýkonné JSON kódování/dekódování (náhrada `encoding/json`) |
| `golang-jwt/jwt/v5` | Tvorba a validace JWT tokenů |
| `google/uuid` | Generování UUID identifikátorů |
| `gorilla/websocket` | WebSocket spojení (hot-reload notifikace) |
| `hashicorp/golang-lru/v2` | LRU cache pro cold vrstvu víceúrovňového cache |
| `jackc/pgx/v5` | Nativní PostgreSQL ovladač s connection poolem |
| `mileusna/useragent` | Parsování User-Agent hlavičky (název prohlížeče, OS) |
| `pquerna/otp` | Generování a validace TOTP kódů (RFC 6238) |
| `redis/go-redis/v9` | Redis klient |
| `skip2/go-qrcode` | Generování QR kódu pro 2FA nastavení |
| `golang.org/x/crypto` | Argon2id hashování hesel |
| `golang.org/x/sync` | `errgroup` pro paralelní úlohy, `singleflight` pro deduplikaci dotazů |
| `golang.org/x/text` | Unicode normalizace (recovery kódy) |
| `golang.org/x/time` | Token-bucket rate limiter |

---

## Architektura a struktura projektu

```
.
├── cmd/server/main.go        # Vstupní bod aplikace
├── internal/
│   ├── analytics/            # Sledování HTTP požadavků a logování
│   ├── auth/                 # Autentizace, JWT, 2FA, OAuth2, CSRF
│   ├── cache/                # Generický víceúrovňový LRU cache
│   ├── database/             # Inicializace PostgreSQL connection poolu
│   ├── email/                # Emailový manager (SMTP, šablony)
│   ├── ex-api/               # Klienti pro externí API (OpenWeatherMap, ip-api)
│   ├── i18n/                 # Správa jazykových překladů
│   ├── location/             # Vyhledávání a ukládání lokací
│   ├── redis/                # Inicializace Redis klienta
│   ├── server/               # HTTP server, router, handlery, rate limiter
│   ├── templates/            # Správa HTML šablon
│   ├── weather/              # Počasí – načítání, cachování, ukládání
│   └── web/                  # Pomocné typy pro HTTP handlery a middleware
├── pkg/util/                 # Pomocné funkce (env proměnné apod.)
├── site/
│   ├── i18n/                 # Jazykové soubory (cs, en)
│   ├── static/               # Statické soubory (CSS, JS, obrázky)
│   └── templates/            # HTML a emailové šablony
├── docker/                   # Docker konfigurační soubory
│   ├── docker-compose.yml
│   ├── Dockerfile.server
│   ├── Dockerfile.database
│   └── nginx/
├── init-schema.sql           # Inicializační SQL schéma databáze
└── go.mod / go.sum           # Go závislosti
```

Server při spuštění inicializuje všechny služby v tomto pořadí:
1. PostgreSQL connection pool (`database`)
2. Redis klient (`redis`)
3. Analytics a logging service
4. Konfigurace z proměnných prostředí
5. i18n manager a template manager
6. Klienti externích API (OpenWeatherMap, ip-api)
7. Location service a Weather service
8. Email manager
9. Auth handler (včetně OAuth2 providerů)
10. HTTP server na portu `:8080`

Graceful shutdown čeká na dokončení všech front (location saver, weather saver) před zavřením databáze a Redis.

---

## Databázové schéma

### `locations`
Tabulka měst a lokalit se sloupci pro fulltextové vyhledávání a geospatální indexy.

- Automaticky generovaný `search_vector` (tsvector) z názvu města, státu a země
- Automaticky generovaný `geom` bod (PostGIS, SRID 4326) z lat/lon
- GiST index na geometrii, GIN indexy na tsvector a trigramech
- JSON sloupec `local_names` s lokálními překlady názvu

### `weather_current_cache`
UNLOGGED tabulka pro aktuální počasí (rychlé zápisy, JSON blob).

### `weather_history`
Historie a předpovědi počasí agregovaná po dnech – teploty, vlhkost, vítr, tlak, UV index.

### `users`
Uživatelé s rolí, preferovaným jazykem, jednotkami, příznakem ověření a soft-delete (`deleted_at`).

### `user_credentials`
Přihlašovací a bezpečnostní pověření (heslo Argon2id, TOTP secret, recovery kódy). Každý uživatel může mít právě jeden záznam druhu `passkey` a jeden `totp`.

### `refresh_sessions`
Refresh tokeny – uloženy jako SHA-256 hash, s IP adresou, device name a příznakem „zapamatovat si mě".

### `user_challenges`
Jednorázové výzvy pro reset hesla a ověření emailu (token hash + kód hash, počet pokusů, expirace).

### `tasks`
Uživatelské úkoly/poznámky s popisem, stavem dokončení a termínem.

### `analytics`
Záznamy HTTP požadavků: cesta, metoda, HTTP status, doba zpracování, IP, User-Agent.

### `logs`
Aplikační logy (level, zpráva, JSON kontext).

---

## Funkcionalita

### Autentizace a správa uživatelů

Systém implementuje kompletní autentizační tok:

**Registrace**
- Validace uživatelského jména a emailu (case-insensitive díky `citext`)
- Hashování hesla algoritmem **Argon2id** (64 MB paměti, 3 průchody, 2 vlákna)
- Po registraci je odeslán ověřovací email

**Přihlášení**
- CSRF ochrana pro všechny POST endpointy
- Přihlášení vrací **JWT access token** (platnost 15 minut) a **refresh token** (platnost 30 dní)
- Kvantizované zpoždění odpovědi (300 ms ± 50 ms) zabraňuje timing útokům
- Při přihlášení z nového zařízení je odeslán bezpečnostní email
- Volitelný příznak „zapamatovat si mě" nastaví trvalé refresh cookies

**Obnova tokenů**
- `GET /api/auth/refresh` – tiché obnovení access tokenu pomocí refresh tokenu (HttpOnly cookie)
- Refresh token je uložen jako SHA-256 hash, nikdy v plaintextu

**Odhlášení**
- `GET /api/auth/logout` – vymaže access a refresh cookie

**Reset hesla**
- Třífázový flow: iniciace → ověření kódu z emailu → potvrzení nového hesla
- Kód i token jsou uloženy jako SHA-256 hash, expirace 15 minut

**Ověření emailu**
- Stejný třífázový flow jako reset hesla

---

### Dvoufaktorová autentizace (2FA)

Implementace standardu **TOTP** (RFC 6238):

1. **Inicializace** (`POST /api/auth/2fa/init`)
   - Vygeneruje TOTP klíč, zašifruje secret pomocí **AES-256-GCM** a uloží do Redis (platnost 10 minut)
   - Vrátí base64 PNG obrázek QR kódu pro naskenování autentizační aplikací

2. **Aktivace** (`POST /api/auth/2fa/enable`)
   - Ověří TOTP kód, uloží zašifrovaný secret do PostgreSQL
   - Vygeneruje 10 recovery kódů (3 náhodná slova z i18n slovníku, hashovaná Argon2id)

3. **Přihlášení s 2FA** (`POST /api/auth/2fa/login`)
   - Omezení na 5 neúspěšných pokusů za 15 minut (Redis čítač)

4. **Recovery kódy** (`POST /api/auth/2fa/recovery/verify`)
   - Každý kód lze použít pouze jednou (po použití je smazán z databáze)
   - Regenerace 10 nových kódů: `POST /api/auth/2fa/recovery/regen`

---

### OAuth2 přihlášení

Podporovaní provideři: **GitHub** a **Google**

- `GET /api/auth/e/login` – přesměrování na OAuth2 provider
- `GET /api/auth/e/callback` – zpracování callback s authorization code
- `POST /api/auth/e/finalize` – dokončení registrace/přihlášení (výběr uživatelského jména při první registraci)
- `GET /api/auth/e/cancel` – zrušení probíhající OAuth2 autentizace

---

### Služba polohy

Vyhledávání a ukládání geografických lokalit:

- **Vyhledávání textem** – kombinace fulltextového vyhledávání (tsvector) a trigram similarity
- **Geolokace dle IP** – pomocí [ip-api.com](http://ip-api.com) klienta s rate limiterem (40 req/min)
- **Reverzní geokódování** – přes OpenWeatherMap Geocoding API
- **Víceúrovňový cache** – hot vrstva (`sync.Map` s TTL) + cold vrstva (LRU, 500 položek)
  - Automatická propagace z cold do hot po N přístupcích (singleflight deduplikace)
  - Janitor každých 10 minut maže expirované hot záznamy
- **Asynchronní ukládání** – nové lokality se ukládají do fronty a dávkově zapisují do PostgreSQL

---

### Služba počasí

Aktuální počasí a historická data prostřednictvím **OpenWeatherMap API**:

- `GET /api/weather` – aktuální počasí pro zadanou lokalitu
- Data jsou přeložena do jazyka uživatele pomocí i18n manageru
- **Víceúrovňový cache** stejný jako u lokací (LRU 500 položek, TTL 2 hodiny)
- **singleflight** – souběžné požadavky na stejnou lokalitu zavolají API pouze jednou
- **Asynchronní ukládání** do `weather_current_cache` a `weather_history` (dávky až 100 záznamů)

---

### Analytika a logování

**Analytics middleware** zaznamenává každý HTTP požadavek:
- URL cesta, HTTP metoda, stavový kód, doba zpracování (mikrosekundy)
- IP adresa klienta, User-Agent
- Uživatelské ID (pokud je přihlášen)

Data jsou periodicky (každých 30 sekund) přesouvána z Redis bufferu do PostgreSQL tabulky `analytics`.

**Custom slog handler** zapisuje aplikační logy (INFO, WARN, ERROR) do tabulky `logs` s JSON kontextem.

---

### Email

EmailManager odesílá transakční emaily přes SMTP:

| Typ emailu | Kdy se odesílá |
|---|---|
| Ověření emailu | Po registraci nebo manuální žádosti |
| Reset hesla | Po inicializaci resetu hesla |
| Nové přihlášení | Při přihlášení z dosud neznámého zařízení/IP |

Emaily jsou renderovány pomocí HTML šablon s podporou více jazyků. V prostředí vývoje se používá **Mailpit** jako lokální SMTP server.

---

### Internacionalizace (i18n)

- Podporované jazyky: **čeština (cs)** a **angličtina (en)**
- Jazykové soubory jsou načítány ze složky `site/i18n/`
- Jazyk je detekován z HTTP požadavku (cookie nebo Accept-Language hlavička)
- Middleware `i18n.Middleware` nastavuje jazyk do kontextu požadavku
- i18n se využívá i pro generování recovery kódů (výběr náhodných slov z lokálního slovníku)

---

### Šablony

**TemplateManager** spravuje HTML šablony pro stránky i emaily:
- Šablony jsou načítány ze složky `site/templates/`
- Podporuje ETag-based caching pro efektivní HTTP caching
- Statické soubory jsou servírovány ze složky `site/static/`
- V dev režimu WebSocket hot-reload automaticky aktualizuje šablony bez restartu serveru

---

### Cache

Generický **TieredCache** (`internal/cache/cache.go`) s plnou podporou Go generik:

```
Hot vrstva (sync.Map)          ← velmi rychlé čtení, TTL expirace
      ↑ propagace po N přístupcích
Cold vrstva (LRU Cache)        ← hashicorp/golang-lru, omezená velikost
      ↑ load miss
Databáze / externí API
```

- **Promotion worker** – po dosažení prahu přístupů přesune záznam z cold do hot
- **Janitor goroutine** – pravidelně maže celou hot vrstvu (atomic pointer swap)
- **ShardedCache** – shardovaná LRU cache pro paralelní přístup bez globálního zámku

---

### Omezení rychlosti (Rate limiting)

Per-user a per-IP rate limiter implementovaný tokenovým kbelíkem (`golang.org/x/time/rate`):

| Stack | Limit |
|---|---|
| Základní API | 5 req/s |
| Auth endpointy | 1 req/s, burst 5 |
| Ext. OAuth2 | 2 req/s, burst 5 |
| Ověření uživatelského jména | 15 req/s |

- Neautentizovaní uživatelé jsou limitováni dle IP adresy
- Uživatelé s rolí `dev` nejsou limitováni
- Limiter záznamy starší než 1 hodinu jsou automaticky odstraňovány

---

### CSRF ochrana

- CSRF token je generován jako kryptograficky náhodný řetězec a uložen v cookie
- Middleware ověřuje token u všech POST požadavků na auth endpointy
- `GET /api/csrf` – endpoint pro získání CSRF tokenu (pro SPA)

---

### WebSocket

`GET /ws-reload` – WebSocket endpoint pro hot-reload notifikace:
- Při změně šablon server pošle zprávu všem připojeným klientům
- Ping/keepalive každých 45 sekund pro udržení spojení

---

## API endpointy

### Veřejné

| Metoda | Cesta | Popis |
|---|---|---|
| `GET` | `/` | Hlavní stránka |
| `GET` | `/sign-in` | Přihlašovací stránka |
| `GET` | `/sign-up` | Registrační stránka |
| `GET` | `/2fa` | Stránka pro zadání 2FA kódu |
| `GET` | `/password-reset` | Stránka pro reset hesla |
| `GET` | `/account-verify` | Stránka pro ověření účtu |
| `GET` | `/api/health` | Health check |
| `GET` | `/api/csrf` | Získání CSRF tokenu |
| `GET` | `/api/auth/refresh` | Obnova access tokenu |

### Autentizace (CSRF chráněné)

| Metoda | Cesta | Popis |
|---|---|---|
| `POST` | `/api/auth/register` | Registrace nového uživatele |
| `POST` | `/api/auth/login` | Přihlášení |
| `GET` | `/api/auth/logout` | Odhlášení |
| `POST` | `/api/verify-username` | Ověření dostupnosti uživatelského jména |
| `POST` | `/api/auth/2fa/init` | Inicializace 2FA |
| `POST` | `/api/auth/2fa/enable` | Aktivace 2FA |
| `POST` | `/api/auth/2fa/login` | Přihlášení s TOTP kódem |
| `POST` | `/api/auth/2fa/recovery/verify` | Přihlášení s recovery kódem |
| `POST` | `/api/auth/2fa/recovery/regen` | Regenerace recovery kódů |
| `POST` | `/api/auth/reset/init` | Inicializace resetu hesla |
| `POST` | `/api/auth/reset/check` | Ověření kódu pro reset |
| `POST` | `/api/auth/reset/confirm` | Potvrzení nového hesla |
| `POST` | `/api/auth/verify/init` | Inicializace ověření emailu |
| `POST` | `/api/auth/verify/check` | Ověření kódu z emailu |
| `POST` | `/api/auth/verify/confirm` | Potvrzení ověření emailu |
| `GET` | `/api/auth/e/login` | OAuth2 přihlášení |
| `GET` | `/api/auth/e/callback` | OAuth2 callback |
| `POST` | `/api/auth/e/finalize` | Dokončení OAuth2 registrace |
| `GET` | `/api/auth/e/cancel` | Zrušení OAuth2 flow |

### Chráněné (vyžadují přihlášení)

| Metoda | Cesta | Popis |
|---|---|---|
| `GET` | `/api/weather` | Aktuální počasí |
| `GET` | `/api/location` | Vyhledání / geolokace |
| `GET` | `/2fa/setup` | Stránka nastavení 2FA |
| `GET` | `/page` | Testovací stránka |
| `GET` | `/ws-reload` | WebSocket hot-reload |

---

## Bezpečnost

| Oblast | Implementace |
|---|---|
| Hashování hesel | Argon2id (64 MB, 3 průchody, 2 vlákna) |
| Šifrování TOTP secretů | AES-256-GCM s náhodným nonce |
| Hashování tokenů | SHA-256 (refresh tokeny, challenge tokeny) |
| JWT | HMAC-SHA256, platnost 15 min |
| Cookies | `HttpOnly`, `Secure`, `SameSite=Lax` |
| CSRF | Double-submit cookie pattern |
| Timing útoky | Constant-time compare (`crypto/subtle`), kvantizované prodlevy |
| Rate limiting | Token bucket per user/IP |
| 2FA brute-force | Max 5 pokusů za 15 minut (Redis) |
| Password reset brute-force | Max 10 pokusů (databázový čítač) |
| Recovery kódy | Jednorázové použití, Argon2id hash |
| Unicode normalizace | NFC normalizace recovery kódů |

---

## Nasazení (Docker)

Celá infrastruktura je orchestrována pomocí Docker Compose:

```bash
cd docker
docker compose up --build
```

Spuštěné služby:
- **server** – Go aplikace na portu 8080 (interní)
- **database** – PostgreSQL s PostGIS a dalšími rozšířeními
- **redis** – Redis 7 s AOF persistencí a heslem
- **nginx** – Reverzní proxy na portech 80 a 443 (Brotli komprese, TLS via Let's Encrypt)
- **mailpit** – Lokální email server (dostupný na `/mail`)

Komunikace mezi serverem a databází probíhá přes Unix socket (sdílený Docker volume `dbsocket`) pro maximální výkon.

---

## Proměnné prostředí

Nastavují se v souboru `docker/.env`:

| Proměnná | Popis |
|---|---|
| `WEATHER_API_KEY` | API klíč pro OpenWeatherMap |
| `ACCESS_SECRET_AUTH` | Tajný klíč pro podepisování JWT access tokenů |
| `TWO_FACTOR_SECRET_AUTH` | Klíč pro AES šifrování TOTP secretů (32 bytes) |
| `PROVIDER_SECRET_AUTH` | Tajný klíč pro OAuth2 provider state tokeny |
| `GITHUB_CLIENT_ID_AUTH` | GitHub OAuth2 Client ID |
| `GITHUB_CLIENT_SECRET_AUTH` | GitHub OAuth2 Client Secret |
| `GITHUB_REDIRECT_URL_AUTH` | GitHub OAuth2 callback URL |
| `GOOGLE_CLIENT_ID_AUTH` | Google OAuth2 Client ID |
| `GOOGLE_CLIENT_SECRET_AUTH` | Google OAuth2 Client Secret |
| `GOOGLE_REDIRECT_URL_AUTH` | Google OAuth2 callback URL |
| `DB_USER` | PostgreSQL uživatel |
| `DB_PASSWORD` | PostgreSQL heslo |
| `DB_NAME` | Název databáze |
| `DB_SOCKET_PATH` | Cesta k Unix socketu PostgreSQL |
| `REDIS_PASSWORD` | Redis heslo |
| `REDIS_ADDRESS` | Redis adresa (výchozí: `redis:6379`) |
| `I18N_PATH` | Cesta k i18n souborům (výchozí: `./i18n`) |
| `TMPL_PATH` | Cesta k šablonám (výchozí: `./templates`) |
| `STAT_PATH` | Cesta ke statickým souborům (výchozí: `./static`) |
