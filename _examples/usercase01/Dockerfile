FROM golang:1.17-alpine AS build
COPY ./  /eg/
WORKDIR /eg/_examples/usercase01
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go get -d -v

RUN go build -o /eg/usercase01 main.go


# runtime
FROM alpine
COPY --from=build /eg/usercase01  /eg/usercase01
ENTRYPOINT [ "/eg/usercase01" ]
CMD [ "--broker", "k8s", "--port", "9527" ]
