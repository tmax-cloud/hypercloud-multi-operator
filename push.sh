#!/bin/sh
## step1 - set env
registry="192.168.9.12:5000"
ckmaster="192.168.9.194"
testdir="/root/shkim"
repo="$(pwd | tr "/" " " | awk '{print $NF}')"
branch="$(git symbolic-ref --short -q HEAD)"
latest=1
for tag in $(curl -X GET http://"$registry"/v2/"$repo"-"$branch"/tags/list 2>/dev/null | jq '.tags[]' | sed 's/\"//g' | sed 's/v//g' | sed 's/.0//g')
do
    if [ "$latest" -lt "$tag" ]; then
        latest="$tag"
    fi
done
publish=$(($latest + 1))

## step2 - build image & push to docker registry
echo "\n\nSTART: build to version \"v"$publish".0\"\n\n"
make generate manifest
img="$registry"/hypercloud-multi-operator-"$branch":v"$publish".0
#img="$registry"/hypercloud-multi-operator-"$branch":v"$1".0
make docker-build IMG="$img"
make docker-push IMG="$img"

## step3 - scp crd files to test env
scp ./config/crd/bases/claim.tmax.io_clusterclaims.yaml root@"$ckmaster":"$testdir"/cc-crd-v"$publish".0.yaml
scp ./config/crd/bases/cluster.tmax.io_clustermanagers.yaml root@"$ckmaster":"$testdir"/clm-crd-v"$publish".0.yaml
scp ./config/crd/bases/cluster.tmax.io_clusterregistrations.yaml root@"$ckmaster":"$testdir"/clr-crd-v"$publish".0.yaml