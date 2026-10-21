FROM node:24-alpine AS frontend
WORKDIR /web
RUN npm install -g pnpm@11.25.0
COPY web/package.json web/pnpm-lock.yaml web/.npmrc ./
RUN pnpm install --frozen-lockfile --ignore-scripts
COPY web/ ./
RUN pnpm build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /expense-bot ./cmd/expense-bot

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /expense-bot /app/expense-bot
COPY --from=frontend /web/dist /app/web/dist
USER app
EXPOSE 8080
ENTRYPOINT ["/app/expense-bot"]
