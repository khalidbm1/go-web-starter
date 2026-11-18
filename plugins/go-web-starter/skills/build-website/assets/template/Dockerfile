FROM node:22-alpine AS frontend
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY site.json /src/site.json
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go site.json ./
COPY migrations/ migrations/
RUN CGO_ENABLED=0 go build -trimpath -o /website .

FROM alpine:3.23
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend /website /usr/local/bin/website
COPY --from=frontend /src/web/dist/ web/dist/
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["website"]
