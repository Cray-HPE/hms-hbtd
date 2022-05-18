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

killall -9 fake-hsm
killall -9 hbtd

echo "Starting Fake HSM..."
./fake-hsm > /tmp/fake-hsm.out 2>&1 &
## get PID of fake-hsm
hsmPID=$!
sleep 2

echo "Starting HBTD..."

./hbtd --debug=3 --sm_url=http://10.0.2.15:27999/hsm/v1 --kv_url="mem:" --use_telemetry=no  > /tmp/hbtd.out 2>&1 &

sleep 5

while [ 1 -eq 1 ]; do
	sss=`date +"%S"`
	(( sm10 = sss % 60 ))
	if (( sm10 == 0 )); then
		break
	fi
	sleep 1
done

echo "Start hurling HBs."


hb x0c0s0b0n0 1000
hb x0c0s0b0n1 1001
hb x0c0s0b2n3 1011
sleep 3 # T+3
hb x0c0s0b0n2 1002
hb x0c0s0b0n3 1003
hb x0c0s0b1n0 1004
hb x0c0s0b1n1 1005
hb x0c0s0b1n2 1006
hb x0c0s0b1n3 1007
hb x0c0s0b2n0 1008
hb x0c0s0b2n1 1009
hb x0c0s0b2n2 1010

# T+10 HB checker x0c0s0b0n0 x0c0s0b0n1 late by 10 sec, WARN
#                 x0c0s0b2n3            late by 10 sec, WARN

sleep 8 # T+11
hb x0c0s0b0n0 1000
hb x0c0s0b0n1 1001

sleep 1 # T+12
hb x0c0s0b0n0 1000
hb x0c0s0b0n1 1001
hb x0c0s0b0n2 1002
hb x0c0s0b0n3 1003
hb x0c0s0b1n0 1004
hb x0c0s0b1n1 1005
hb x0c0s0b1n2 1006
hb x0c0s0b1n3 1007
hb x0c0s0b2n0 1008
hb x0c0s0b2n1 1009
hb x0c0s0b2n2 1010

# T+15 HB checker x0c0s0b0n0 x0c0s0b0n1 restarted
# T+20 HB checker no lates

sleep 9 # T+21
hb x0c0s0b0n0 1000
hb x0c0s0b0n1 1001
hb x0c0s0b0n2 1002
hb x0c0s0b0n3 1003
hb x0c0s0b1n0 1004
hb x0c0s0b1n1 1005
hb x0c0s0b1n2 1006
hb x0c0s0b1n3 1007
hb x0c0s0b2n0 1008
hb x0c0s0b2n1 1009

# T+25 HB checker x0c0s0b2n2 overdue 13 sec WARN

sleep 6 # T+27
hb x0c0s0b0n0 1000
hb x0c0s0b0n1 1001
hb x0c0s0b0n2 1002
hb x0c0s0b0n3 1003
hb x0c0s0b1n0 1004
hb x0c0s0b1n1 1005
hb x0c0s0b1n2 1006
hb x0c0s0b1n3 1007
hb x0c0s0b2n0 1008
hb x0c0s0b2n1 1009
hb x0c0s0b2n2 1010

# T+30 HB checker x0c0s0b2n2 HB restarted
#                 x0c0s0b2n3 dead

sleep 8 # T+35
hb x0c0s0b0n0 1000
hb x0c0s0b0n1 1001
hb x0c0s0b0n2 1002
hb x0c0s0b0n3 1003
hb x0c0s0b1n0 1004
hb x0c0s0b1n1 1005
hb x0c0s0b1n2 1006
hb x0c0s0b1n3 1007
hb x0c0s0b2n0 1008
hb x0c0s0b2n1 1009
hb x0c0s0b2n3 1011

# T+35 HB checker  x0c0s0b2n3 restarted
# T+40 HB checker x0c0s0b2n2 late 13 sec WARN

sleep 8 # T+43
hb x0c0s0b0n0 1000
hb x0c0s0b0n1 1001
hb x0c0s0b0n2 1002
hb x0c0s0b0n3 1003
hb x0c0s0b1n0 1004
hb x0c0s0b1n1 1005
hb x0c0s0b1n2 1006
hb x0c0s0b1n3 1007
hb x0c0s0b2n0 1008
hb x0c0s0b2n1 1009

# T+45 HB checker x0c0s0b2n3 overdue 10 sec WARN
# T+50 HB checker
# T+55 HB checker  1000-1009 overdue by 12 sec. x0c0s0b2n2 and x0c0s0b2n3
#                  no warning because they are already in WARN
# T+60 HB checker x0c0s0b2n2 late 33 sec ALERT
# T+65 HB checker x0c0s0b2n3 late 30 sec ALERT (might miss)
# T+70 HB checker x0c0s0b2n3 late 34 sec ALERT (if missed)
# T+75 HB checker 1000-1009 overdue 31 sec, ALERT

exit 0

#special cases:

# Make HSM return errors, but quickly; hurl more HBs; then fix HSM.
# Do this by curl-ing a bulk state update with componentID[0] == "STOP"
# Fix by curl-ing with componentID[0] == "START"

curl -X PATCH -d '{"ComponentIDs":["STOP"]}' http://10.0.2.15:27999/hsm/v1/State/BulkStateData

# curl some HBs

curl -X PATCH -d '{"ComponentIDs":["START"]}' http://10.0.2.15:27999/hsm/v1/State/BulkStateData

# Check that desired HBs got through by looking at fake-hsm logs


# Make HSM stall (kill -SIGSTOP); hurl more HBs; then fix HSM.
