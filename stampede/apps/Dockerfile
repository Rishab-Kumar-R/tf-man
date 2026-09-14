FROM golang:1.26-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS builder
ARG SERVICE
WORKDIR /src
COPY internal ./internal
COPY ${SERVICE} ./${SERVICE}
WORKDIR /src/${SERVICE}
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app .
LABEL authors="rishabkumar"

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=builder /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
