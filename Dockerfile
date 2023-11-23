FROM node:18.18.2-alpine3.18 AS frontend

RUN apk add --no-cache git just

WORKDIR /app

COPY web/package.json web/package-lock.json ./

RUN npm install

COPY web/ ./

RUN npm run build
# output artifacts are in /app/dist

FROM golang:1.21-alpine3.18 AS builder

RUN apk add --no-cache git

WORKDIR /app
RUN mkdir -p /app/bin

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o /app/bin/server main.go
# output artifacts are in /app/bin

FROM alpine:3.14.3

RUN apk add --no-cache ca-certificates

WORKDIR /opt/habanero

RUN mkdir -p web
COPY --from=frontend /app/dist ./web/dist
COPY --from=builder /app/bin/server ./server

EXPOSE 8080

CMD ["./server"]
