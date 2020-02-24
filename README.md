# ton-dice-web-worker
resolving bets and send them to the dice server to save

## build 
```MY_KEY=$(cat ~/.ssh/id_rsa)```

```docker build --build-arg SSH_PRIVATE_KEY="$MY_KEY" -t dice-worker .```

## run
```docker run --name dice-worker --network dice-network -e STORAGE_HOST=dice-server -e STORAGE_PORT=5300 -e TON_API_HOST=ton-api -e TON_API_PORT=5400 --entrypoint=/bin/sh -it -d dice-worker```
