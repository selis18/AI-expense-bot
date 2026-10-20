# AI Expense Bot

[English](#english) | [Русский](#русский)

## English

A Telegram bot for tracking expenses. Given a message such as “кофе 350” (“coffee 350”), the model extracts the amount, category, and date. The application saves the expense in PostgreSQL.

Currently, the bot supports adding one expense per message. Viewing, editing, deleting expenses, and the web panel are not implemented yet. The application has not been tested with live Telegram and LLM APIs.

### Setup

You need Docker Compose, a Telegram bot token, and access to an LLM API that supports Chat Completions tool calls.

1. Copy `.env.example` to `.env`.
2. Set `TELEGRAM_TOKEN`, `LLM_ENDPOINT`, `LLM_API_KEY`, and `LLM_MODEL`.
3. Check `FAMILY_TIMEZONE` and the PostgreSQL password.
4. Run:

```sh
docker compose up --build
```

Set `LLM_ENDPOINT` to the full Chat Completions URL. The API must support `tools` and `tool_choice: auto`. The Amvera connection has not been tested yet.

Compose passes database settings through `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, and `PGPASSWORD`. Use `DATABASE_URL` when running the application outside a container. The PostgreSQL port is accessible only within the Compose network.

Readiness URL: `http://localhost:8080/readyz`. The `/healthz` route checks the HTTP server; `/readyz` checks the database connection. Neither checks Telegram or LLM availability.

Open a private chat with the bot and send `кофе 350`. Each message must include a description and an amount. Group messages and follow-up clarifications are not supported yet.

To inspect saved expenses:

```sh
docker compose exec postgres psql -U expenses -d expenses -c "SELECT id,user_id,amount,category,spent_at,comment FROM expenses ORDER BY id;"
```

The database stores `amount` in kopecks: 350 rubles is saved as 35000. The model passes amounts as decimal strings in rubles, such as `"350.50"`. Conversion does not use floating-point arithmetic.

### Development and tests

Go 1.26.5 or later is required. The application reads environment variables; it does not load `.env` itself. Compose loads that file.

```sh
go test ./...
go vet ./...
go build ./cmd/expense-bot
```

The PostgreSQL test is skipped unless `TEST_DATABASE_URL` is set. Set it to a test database connection string, then run:

```sh
go test -v ./internal/store
```

The test creates a separate schema, checks migrations, repeated operations, and rollback on failure, then drops that schema. The connection must use a test database with `CREATE SCHEMA` permissions.

### Project structure

The application uses Go, chi v5 for routing, Resty v2 for Telegram and LLM requests, and pgx v5 for PostgreSQL.

```text
cmd/expense-bot       startup, shutdown, and readiness checks
internal/config      configuration
internal/expense     amounts and expense validation
internal/dialog      message handling and saved responses
internal/llm         model requests with the create_expense tool
internal/store       PostgreSQL and the embedded migration
internal/telegram    receiving messages and sending replies
```

The expense and bot response are saved in one transaction. Ownership comes from the sender's Telegram ID. Each message can create one expense. The Telegram event identifier (`update_id`) lets the application detect repeated processing and return the saved response.

### Limitations

- Messages from all users are processed sequentially. A failed reply delays subsequent messages.
- A reply may be sent again after a failure. Saving `update_id` in the same transaction prevents duplicate expenses.
- The model receives only the current message, without conversation history.
- The container build, PostgreSQL migration, and complete flow from message to database record have not been verified yet. The database test requires `TEST_DATABASE_URL`; LLM tests use a local HTTP server.

## Русский

Бот для учёта расходов в Telegram. Из сообщения «кофе 350» модель извлекает сумму, категорию и дату, а приложение сохраняет расход в PostgreSQL.

Пока реализовано добавление одной траты за сообщение. Просмотр, изменение, удаление расходов и веб-панель ещё не готовы. Работа с реальными Telegram и LLM API пока не проверена.

### Запуск

Нужны Docker Compose, токен Telegram-бота и доступ к LLM API с поддержкой вызова инструментов в формате Chat Completions.

1. Скопировать `.env.example` в `.env`.
2. Заполнить `TELEGRAM_TOKEN`, `LLM_ENDPOINT`, `LLM_API_KEY`, `LLM_MODEL`.
3. Проверить часовой пояс `FAMILY_TIMEZONE` и пароль PostgreSQL.
4. Выполнить:

```sh
docker compose up --build
```

В `LLM_ENDPOINT` указывается полный URL метода Chat Completions. API должен поддерживать `tools` и `tool_choice: auto`. Подключение через Amvera ещё не проверено.

Compose передаёт настройки БД через переменные `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER` и `PGPASSWORD`. `DATABASE_URL` нужен для запуска приложения вне контейнера. Порт PostgreSQL доступен только внутри сети Compose.

Адрес проверки готовности: `http://localhost:8080/readyz`. Маршрут `/healthz` проверяет работу HTTP-сервера, `/readyz` — соединение с БД. Доступность Telegram и LLM они не проверяют.

Открыть личный чат с ботом и отправить `кофе 350`. Каждое сообщение должно содержать описание и сумму. Групповые сообщения и уточнения в следующих репликах пока не поддерживаются.

Для просмотра записей:

```sh
docker compose exec postgres psql -U expenses -d expenses -c "SELECT id,user_id,amount,category,spent_at,comment FROM expenses ORDER BY id;"
```

Поле `amount` в БД хранит копейки: 350 рублей сохраняется как 35000. Модель передаёт сумму строкой в рублях, например `"350.50"`. Преобразование выполняется без чисел с плавающей точкой.

### Разработка и тесты

Нужен Go 1.26.5 или новее. Приложение читает переменные окружения; само `.env` не загружает (его читает Compose).

```sh
go test ./...
go vet ./...
go build ./cmd/expense-bot
```

Тест PostgreSQL пропускается без `TEST_DATABASE_URL`. Для запуска указать строку подключения к тестовой БД в окружении и выполнить:

```sh
go test -v ./internal/store
```

Тест создаёт отдельную схему, проверяет миграцию, повтор операции и откат при ошибке, затем удаляет созданную схему. Подключение должно вести к тестовой БД с правами `CREATE SCHEMA`.

### Структура

Приложение написано на Go. Для маршрутизации используется chi v5, для запросов к Telegram и LLM — Resty v2, для PostgreSQL — pgx v5.

```text
cmd/expense-bot       запуск, завершение и проверка готовности
internal/config      настройки
internal/expense     суммы и проверка полей расхода
internal/dialog      обработка сообщений и сохранённых ответов
internal/llm         запрос к модели с инструментом create_expense
internal/store       PostgreSQL и встроенная миграция
internal/telegram    получение сообщений и отправка ответов
```

Расход и ответ бота сохраняются в одной транзакции. Владелец определяется по Telegram ID отправителя. Каждое сообщение может создать одну трату. По идентификатору события Telegram (`update_id`) приложение распознаёт повторную обработку и возвращает сохранённый ответ.

### Ограничения

- Сообщения всех пользователей обрабатываются последовательно. Ошибка отправки ответа задерживает обработку следующих сообщений.
- После сбоя бот может отправить ответ повторно. Повторное создание траты предотвращается записью `update_id` в той же транзакции.
- Модель получает только текущее сообщение, без истории переписки.
- Сборка контейнера, миграция на PostgreSQL и полный путь от сообщения до записи в БД пока не проверены. Тест БД требует `TEST_DATABASE_URL`; тесты LLM используют локальный HTTP-сервер.
