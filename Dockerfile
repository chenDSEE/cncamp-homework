FROM golang:1.16-alpine AS build

# build go server
RUN go env -w GOPROXY=https://goproxy.cn,direct
WORKDIR /go/src/homework
COPY . /go/src/homework
RUN go mod download && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o /bin/server homework.go


FROM scratch
COPY --from=build /bin/server /bin/server
ENV AIMERNAME=aimer2
ENTRYPOINT ["/bin/server"]
