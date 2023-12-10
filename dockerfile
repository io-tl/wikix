FROM dokken/centos-7 AS rootfs-builder
ARG GOVERSION=go1.21.5.linux-amd64.tar.gz

WORKDIR /
COPY . build

RUN wget https://go.dev/dl/${GOVERSION}
RUN tar xzf ${GOVERSION}

RUN  ulimit -n 1024 && yum -y install make gcc glibc-devel.i686 glibc-devel.x86_64 libgcc.x86_64 libgcc.i686

WORKDIR /build

ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64
RUN  /go/bin/go build -ldflags="-s -w" -o wikix.x86_64

ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=386
RUN  /go/bin/go build -ldflags="-s -w" -o wikix.x32

FROM scratch AS export-stage
COPY --from=rootfs-builder /build/wikix.x86_64 .
COPY --from=rootfs-builder /build/wikix.x32 .

