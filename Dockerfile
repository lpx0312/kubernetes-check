FROM golang:1.13.9-stretch AS builder

# 添加代理配置
ENV GOPROXY=https://goproxy.cn \
    GOSUMDB=sum.golang.google.cn

WORKDIR /app
COPY . .
RUN go mod download -x
RUN CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -v -a -o pod-monitor.exe ./cmd/pod-monitor

FROM mcr.microsoft.com/windows/nanoserver:1809
COPY --from=builder /app/pod-monitor.exe /pod-monitor.exe
ENTRYPOINT ["pod-monitor.exe"] 