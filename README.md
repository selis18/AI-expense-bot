# AI Expense Bot

## English

Family expense tracking through Telegram and a web panel. The bot understands requests such as “кофе 350” (coffee, 350 rubles), records expenses, finds previous purchases, calculates totals, and handles edits and deletion. Each Telegram user can access only their own expenses. The panel manages expenses for all family members.

Built with Go, chi, Resty, PostgreSQL, React, and TypeScript. The model chooses from five tools; the server validates arguments, enforces ownership, and calculates totals in SQL. Amounts are stored as integer kopecks and exchanged as decimal strings in rubles.

### Run

1. Copy `.env.example` to `.env`.
2. Set `POSTGRES_PASSWORD`, `TELEGRAM_TOKEN`, `LLM_API_KEY`, and `LLM_MODEL`. Check `FAMILY_TIMEZONE`.
3. Start the application:

```sh
docker compose up --build
```

Open [localhost:8080](http://localhost:8080). Compose builds the panel into the application image, runs database migrations, and keeps PostgreSQL data in a volume. The panel has no authentication and is exposed on localhost. Users appear in the panel after writing to the bot.

Set `BOT_ENABLED=false` to run the panel and API without Telegram or LLM credentials. A fresh database has no users; this mode does not create demonstration data.

The default LLM endpoint is `https://inference.waw0.amvera.ru/v1/chat/completions`. Set `LLM_MODEL` to an available model that supports Chat Completions `tools` and `tool_choice: auto`. Model availability and tool support must be checked with your Amvera account; the DeepSeek R1 integration has not been verified. See [Amvera inference documentation](https://docs.amvera.ru/LLM/doc-inference-ru.html).

### Usage

Write to the bot in a private chat:

- `кофе 350`
- `такси 1200 вчера`
- `покажи траты за неделю`
- `сколько потратил на еду в этом месяце`
- `последнее было не 350, а 450`
- `удали такси`

Missing details lead to a clarification question. The last six exchanges within 30 minutes are available to the model. If several expenses match an edit or deletion, reply with a number from the list. A selected expense remains available for 10 minutes. Deletion requires a separate `да` reply; `нет` or `отмена` cancels it. A changed expense requires confirmation again.

The panel provides participant, date, and category filters, totals, pagination, creation, editing, and deletion. A new expense belongs to an existing participant; editing does not change its owner. Dates entered in the panel are calendar dates; the form initially uses the browser's current date. The bot resolves relative dates using `FAMILY_TIMEZONE`.

[HTTP API reference](docs/api.md)

### Configuration

| Variable | Purpose / default |
| --- | --- |
| `POSTGRES_PASSWORD` | Database password in Compose |
| `DATABASE_URL` | PostgreSQL connection for a local Go process |
| `BOT_ENABLED` | `true`; set `false` for panel-only mode |
| `TELEGRAM_TOKEN` | Bot token |
| `LLM_ENDPOINT` | Full Chat Completions URL; Amvera by default |
| `LLM_API_KEY`, `LLM_MODEL` | Provider key and model ID |
| `LLM_TIMEOUT` | Timeout per model request, `30s`, maximum `2m` |
| `FAMILY_TIMEZONE` | `Asia/Yekaterinburg` |
| `HTTP_ADDR` | `:8080`; Compose uses a fixed internal port |
| `WEB_DIR` | Built panel directory, `web/dist` |

The Go process reads environment variables; it does not load `.env`. Compose reads `.env` and supplies PostgreSQL connection settings through `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, and `PGPASSWORD`.

`/healthz` checks the HTTP server. `/readyz` checks the database. Neither checks external APIs.

### Development and tests

Use Go 1.26.5+, Node.js 24, pnpm 11.25.0, and PostgreSQL 17.

```sh
cd web
pnpm install --frozen-lockfile --ignore-scripts
pnpm build
cd ..
go test ./...
go vet ./...
go build -o bin/expense-bot ./cmd/expense-bot
```

Set `DATABASE_URL` and the other application variables before running the binary. For frontend development, `pnpm dev` in `web` proxies `/api` to the Go server on port 8080.

Set `TEST_DATABASE_URL` to a test database and rerun `go test -count=1 ./...` to include database tests. They create and remove isolated schemas and require `CREATE SCHEMA` permission. Without this variable, database tests are skipped.

For browser tests, run the application against a separate test database with at least one user. Then run in `web`:

```sh
pnpm exec playwright install chromium
pnpm test
```

`PANEL_URL` overrides `http://127.0.0.1:8080`. `BROWSER_CHANNEL=msedge` uses installed Edge. Browser tests create, edit, and delete their own expense. They also test filtering and recovery from a failed request.

Local verification passed: Go tests including PostgreSQL integration tests, `go vet`, backend and frontend builds, and browser tests against the real API. HTTP test servers exercise LLM responses and Telegram failures. CI is configured to repeat tests and check Compose startup and data persistence. CI and the Docker build have not been run in this environment; live Telegram and Amvera calls still require credentials.

### Design and limits

`internal/expense` contains validation and action types; `store` owns SQL, migrations, and saved dialogue state; `dialog` handles messages; `llm` and `telegram` use Resty; `httpapi` serves the panel API through chi. `web` contains the React panel.

An operation and its reply are committed together with the Telegram update ID. Reprocessing returns the saved reply without repeating the expense operation. History is recorded afterwards and can be recovered when the update is retried. Delete confirmation stores a snapshot and checks it under a database lock.

The application serves one family, uses rubles, and processes Telegram messages sequentially. One message can execute one tool, with one correction attempt for invalid arguments. Bot lists show up to ten expenses. Temporary sending failures delay subsequent updates; permanent recipient errors are skipped. After a network failure a reply can be delivered twice, but the expense operation is not repeated. Run only one polling instance per bot token. Kafka and RAG are not used for this scope.

## Русский

Учёт семейных расходов через Telegram и веб-панель. Бот добавляет, ищет, считает, изменяет и удаляет траты по обычным сообщениям. В Telegram доступны только собственные расходы; в панели — расходы всех участников.

Стек: Go, chi, Resty, PostgreSQL, React и TypeScript. Модель выбирает один из пяти инструментов. Проверку аргументов и владельца выполняет сервер, суммы считает PostgreSQL. В БД деньги хранятся целыми копейками, в API передаются строками в рублях.

### Запуск

1. Скопировать `.env.example` в `.env`.
2. Заполнить `POSTGRES_PASSWORD`, `TELEGRAM_TOKEN`, `LLM_API_KEY`, `LLM_MODEL`. Проверить `FAMILY_TIMEZONE`.
3. Выполнить `docker compose up --build`.
4. Открыть [localhost:8080](http://localhost:8080).

Compose собирает панель вместе с Go-приложением, запускает миграции и хранит БД в отдельном томе. Панель без авторизации, порт доступен на localhost. Участники появляются после первого сообщения боту.

`BOT_ENABLED=false` запускает только панель и API без ключей внешних сервисов. На пустой БД участников нет; демонстрационные записи автоматически не создаются.

По умолчанию используется `https://inference.waw0.amvera.ru/v1/chat/completions`. В `LLM_MODEL` нужен доступный в аккаунте идентификатор модели с поддержкой `tools` и `tool_choice: auto`. Работа DeepSeek R1 через Amvera пока не проверена. [Документация Amvera](https://docs.amvera.ru/LLM/doc-inference-ru.html).

### Работа с расходами

Примеры: `кофе 350`, `такси 1200 вчера`, `покажи траты за неделю`, `сколько потратил на еду в этом месяце`, `последнее было не 350, а 450`, `удали такси`.

Бот уточняет недостающие сведения и получает последние шесть обменов сообщениями за 30 минут. При нескольких совпадениях можно выбрать номер из списка. Выбор хранится 10 минут. Удаление требует отдельного `да`; `нет` или `отмена` отменяет действие. Если запись успела измениться, подтверждение запрашивается заново.

В панели есть фильтры по участнику, датам и категории, суммы, страницы списка и полный CRUD. Создать трату можно для существующего участника; сменить владельца при редактировании нельзя. Начальная дата формы берётся из браузера. Относительные даты бота вычисляются в `FAMILY_TIMEZONE`.

### Проверки и ограничения

Команды разработки, таблица настроек и запуск браузерных тестов приведены выше. Приложение читает переменные окружения; `.env` самостоятельно не загружает. Для тестов PostgreSQL нужен `TEST_DATABASE_URL` с правами на создание схем. Без него эти тесты пропускаются.

Локально прошли тесты Go с настоящей PostgreSQL, `go vet`, сборка сервера и панели, браузерные проверки CRUD, фильтров и обработки ошибки. LLM и Telegram проверены через локальные HTTP-серверы. Добавлены CI-проверки, включая запуск Compose и сохранение данных после перезапуска. CI и Docker в текущем окружении не запускались; реальные вызовы Telegram и Amvera требуют ключей.

Одна семья, одна валюта — рубли. Сообщения Telegram обрабатываются последовательно, один инструмент на сообщение. При неверных аргументах есть одна попытка исправления. Список бота ограничен десятью записями. Временная ошибка отправки задерживает следующие сообщения; постоянный отказ получателя пропускается. После сетевого сбоя ответ может прийти повторно, но операция с расходом не повторяется. Для одного токена нужен один процесс polling. Kafka и RAG в этом объёме не используются.
