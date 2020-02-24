FROM golang:alpine as builder

ARG SSH_PRIVATE_KEY
RUN mkdir /root/.ssh/
RUN echo "${SSH_PRIVATE_KEY}" > /root/.ssh/id_rsa
RUN chmod 400 /root/.ssh/id_rsa

RUN apk add --no-cache openssh
RUN apk add --no-cache git
RUN git config --global url."git@github.com:".insteadOf "https://github.com/"

RUN touch /root/.ssh/known_hosts
RUN ssh-keyscan github.com >> /root/.ssh/known_hosts

RUN git clone git@github.com:tonradar/ton-api.git

WORKDIR /go/src/build
ADD . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dice-worker ./cmd

FROM poma/ton
WORKDIR /app
COPY --from=builder /go/src/build/dice-worker /app/
COPY --from=builder /go/src/build/config.yml /app/
COPY --from=builder /go/src/build/bet-query.fif /app/
COPY --from=builder /go/src/build/resolve-query.fif /app/
COPY --from=builder /go/src/build/ton-lite-client-test1.config.json /app/
COPY --from=builder /go/src/build/tonlib.config.json.example /app/
COPY --from=builder /go/src/build/denis.pk /app/
COPY --from=builder /go/src/build/trxlt.save.default /app/
RUN cp trxlt.save.default trxlt.save

ENTRYPOINT ./dice-worker