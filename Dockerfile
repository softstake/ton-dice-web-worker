FROM golang:alpine as builder

ARG GITHUB_TOKEN

RUN apk add --no-cache git

RUN git config --global url."https://${GITHUB_TOKEN}:@github.com/".insteadOf "https://github.com/"

RUN git clone https://github.com/tonradar/ton-api.git

WORKDIR /go/src/build
ADD . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dice-worker ./cmd

FROM poma/ton
WORKDIR /app
COPY --from=builder /go/src/build/dice-worker /app/
COPY --from=builder /go/src/build/resolve-query.fif /app/
COPY --from=builder /go/src/build/owner.pk /app/
COPY --from=builder /go/src/build/trxlt.save.default /app/
RUN cp trxlt.save.default trxlt.save

ENTRYPOINT ./dice-worker