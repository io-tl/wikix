FROM golang:1.21 AS rootfs-builder

WORKDIR /
COPY . build

WORKDIR /build

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
RUN go build -ldflags="-s -w" -o wikix.x86_64

ENV CGO_ENABLED=0
ENV GOOS=windows
ENV GOARCH=amd64
RUN  go build  -o wikix.x86_64.exe

FROM scratch AS export-stage
COPY --from=rootfs-builder /build/wikix.x86_64 .
COPY --from=rootfs-builder /build/wikix.x86_64.exe .

