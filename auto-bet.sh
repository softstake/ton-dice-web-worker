#!/bin/bash
# Usage: ./auto-bet.sh <filename-base> <worker-addr> <telegram-id> <bet-amount> <num_games> [<seed>]

if [ $# -lt 5 ]
then
echo "Usage: ${0} <pk_file_base> <wallet_addr> <dice_addr> <bet-amount> <num_games>"
exit 0
fi

file_base=$1
wallet_addr=$2
dice_addr=$3
bet_amount=$4
num_games=$5

for num in $(seq 1 $num_games);
do
roll_under=$(jot -r 1 2 98)

seq=$(./bin/lite-client -C ton-lite-client-test1.config.json -c "runmethod kQD9mP8cffBI5p8YbR0PeoLHfUzFCnL3r-uoL-SR-4JIRyhT seqno" -v 0 2> >(grep result) | awk '/\[ [0-9]* \]/{print $3}')
echo "seq: ${seq}, roll_under: ${roll_under}"

./bet-query.fif ${seq} ${file_base} ${wallet_addr} ${dice_addr} ${bet_amount} ${roll_under} ${num} && ./bin/lite-client -C ton-lite-client-test1.config.json -c "sendfile bet-query.boc";
sleep 60;
done