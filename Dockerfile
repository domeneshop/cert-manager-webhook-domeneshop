FROM golang:1.15.7-alpine AS build_deps

RUN apk add --no-cache git

WORKDIR /workspace
ENV GO111MODULE=on

COPY go.mod .
COPY go.sum .

RUN go mod download

FROM build_deps AS build

COPY . .

ARG GIT_COMMIT=""
ARG GIT_COMMIT_SHORT=""
ARG GIT_TAG=""
ARG GIT_BRANCH=""
ARG BUILD_DATE="1970-01-01T00:00:00Z"

RUN CGO_ENABLED=0 go build -o webhook -ldflags '-w -extldflags "-static"' .

FROM alpine:3.9

RUN apk add --no-cache ca-certificates

COPY --from=build /workspace/webhook /usr/local/bin/webhook

ENTRYPOINT ["webhook"]
