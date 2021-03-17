#!/bin/sh
# kubernetes probe port
port8000=`netstat -ltn | grep 8000 | wc -l`
port10000=`netstat -ltn | grep 10000 | wc -l`
if [[ $port10000 -eq 1 && $port8000 -eq 1 ]]
then
exit 0
else
exit 1
fi
