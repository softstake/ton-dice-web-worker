# ton-dice-web-worker
resolving bets and send them to the dice server to save

Resolve bets:
 - ton-api grpc 

 Save bets:
 - ton-dice-web-server grpc

 TODO:
  - remove unused lite-client
  - remove unused store package
  - move config to ENV vars
  - remove fift stuff
  - change Makefile to use access token instead of SSH key
  - Update README.md to actual


## build 
```MY_KEY=$(cat ~/.ssh/id_rsa)```

```docker build --build-arg SSH_PRIVATE_KEY="$MY_KEY" -t dice-worker .```

## run
```docker run --name dice-worker --network dice-network -e STORAGE_HOST=dice-server -e STORAGE_PORT=5300 -e TON_API_HOST=ton-api -e TON_API_PORT=5400 --entrypoint=/bin/sh -it -d dice-worker```
