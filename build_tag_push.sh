#!/usr/bin/env bash
# MIT License
#
# (C) Copyright [2018-2021] Hewlett Packard Enterprise Development LP
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

IMAGE_NAME="cray-hbtd"

usage() {
        echo "$FUNCNAME: $0"
        echo "  -h | prints this help message"
        echo "  -l | hostname to push to, default localhost";
        echo "  -r | repo to push to, default cray";
        echo "  -f | forces build with --no-cache and --pull";
	echo "";
        exit 0
}


REPO="cray"
REGISTRY_HOSTNAME="localhost"
FORCE=" "
TAG=$USER

while getopts "hfl:r:t:" opt; do
  case $opt in
    h)
      usage;
      exit;;
    f)
      FORCE=" --no-cache --pull";;
    l)
      REGISTRY_HOSTNAME=${OPTARG};;
    r)
      REPO=${OPTARG};;
    t)
      TAG=${OPTARG};;
  esac
done

shift $((OPTIND-1))

echo "Building $FORCE and pushing to $REGISTRY_HOSTNAME in repo $REPO"

set -e
set -x
docker build $FORCE -t cray/$IMAGE_NAME:$TAG .
docker tag cray/$IMAGE_NAME:$TAG $REGISTRY_HOSTNAME/$REPO/$IMAGE_NAME:$TAG
docker push $REGISTRY_HOSTNAME/$REPO/$IMAGE_NAME:$TAG
