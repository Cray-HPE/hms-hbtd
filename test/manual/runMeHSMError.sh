#!/bin/bash
#
# MIT License
#
# (C) Copyright [2022] Hewlett Packard Enterprise Development LP
#
# Permission is hereby granted, free of charge, to any person obtaining a
# copy of this software and associated documentation files (the "Software"),
# to deal in the Software without restriction, including without limitation
# the rights to use, copy, modify, merge, publish, distribute, sublicense,
# and/or sell copies of the Software, and to permit persons to whom the
# Software is furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included
# in all copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
# THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
# OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
# ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
# OTHER DEALINGS IN THE SOFTWARE.
#

hbPld='{"Component":"AAAA", "Hostname": "BBBB", "NID": "CCCC", "Status": "OK", "Timestamp": "DDDD"}'
ord=1

hb () {
	node=$1
	nid=$2
	host="${node}.domain"
	ts="Now+${ord}"

	pld=`echo $hbPld | sed "s/AAAA/$node/" | \
	                   sed "s/BBBB/$host/" | \
	                   sed "s/CCCC/$nid/"  | \
	                   sed "s/DDDD/$ts/"`
	(( ord = ord + 1 ))

	date
	curl -D hout -X POST -d "$pld" http://10.0.2.15:28500/hmi/v1/heartbeat
}

sendhb () {
	hb x0c0s0b0n0 1000
	hb x0c0s0b0n1 1001
	hb x0c0s0b0n2 1002
	hb x0c0s0b0n3 1003
	hb x0c0s0b1n0 1004
	hb x0c0s0b1n1 1005
	hb x0c0s0b1n2 1006
	hb x0c0s0b1n3 1007
}

killall -9 fake-hsm
killall -9 hbtd

echo "Starting Fake HSM..."
./fake-hsm > /tmp/fake-hsm.out 2>&1 &
## get PID of fake-hsm
hsmPID=$!
sleep 2

echo "Starting HBTD..."

./hbtd --debug=3 --sm_url=http://10.0.2.15:27999/hsm/v2 --kv_url="mem:" --use_telemetry=no  > /tmp/hbtd.out 2>&1 &

sleep 5

while [ 1 -eq 1 ]; do
	sss=`date +"%S" | sed 's/^0//'`
	echo "Seconds: ${sss} waiting for 0..."
	(( sm10 = sss % 60 ))
	if (( sm10 == 0 )); then
		break
	fi
	sleep 1
done

echo "Start hurling HBs."


#special cases:

# Make HSM return errors, but quickly; hurl more HBs; then fix HSM.
# Do this by curl-ing a bulk state update with componentID[0] == "STOP"
# Fix by curl-ing with componentID[0] == "START"

sendhb
# T+0 8 HB started
sleep 5 # T+5
sendhb
sleep 5 # T+10

# T+10 no notifications

curl -X PATCH -d '{"ComponentIDs":["STOP"]}' http://10.0.2.15:27999/hsm/v2/State/Components/BulkStateData

# T+15 Detect 8 HB stopped, warning, errors sending
# T+20 Re-attempt to send to HSM, will fail

sleep 11 # T+21

sendhb
hb x0c0s0b2n0 1008

# T+21 x0c0s0b2n0 started.

curl -X PATCH -d '{"ComponentIDs":["START"]}' http://10.0.2.15:27999/hsm/v2/State/Components/BulkStateData

# T+25 8 HB restarted
# T+25 Re-send HB changes to HSM, should succeed

sleep 7 # T+28
sendhb

# T+30 no notifications
# T+35,  x0c0s0b2n0 overdue 14 sec WARN sent
# T+40 8 HBs WARN
# T+55 x0c0s0b2n0 dead
# T+60 8 HBs declared dead
