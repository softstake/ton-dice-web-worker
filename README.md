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

## ENV VARS
	* CONTRACT_ADDR - contract address, required variable, no default value.
	* KEY_FILE_BASE - full path (with filename) to the private key file, required variable, default value is `owner` just for development purposes.
	* STORAGE_HOST - host of the `ton-dice-web-server` service, required variable, no default value.
	* STORAGE_PORT - port of the `ton-dice-web-server` service, default value is `5300`.
	* TON_API_HOST - host of the `ton-dice-api` service, required variable, no default value.
	* TON_API_PORT - port of the `ton-dice-api` service, default value is '5400'. 

## build 
```MY_KEY=$(cat ~/.ssh/id_rsa)```

```docker build --build-arg SSH_PRIVATE_KEY="$MY_KEY" -t dice-worker .```

## run
```docker run --name dice-worker --network dice-network -e STORAGE_HOST=dice-server -e STORAGE_PORT=5300 -e TON_API_HOST=ton-api -e TON_API_PORT=5400 --entrypoint=/bin/sh -it -d dice-worker```
