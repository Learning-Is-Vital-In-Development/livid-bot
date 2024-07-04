FROM golang:1.22.4 AS builder
WORKDIR /app
ENV DISCORD_BOT_TOKEN="your_bot_token_here"
ENV DISCORD_APPLICATION_ID="your_application_id_here"
ENV DISCORD_GUILD_ID="your_guild_id_here"
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
CMD ["./main"]
