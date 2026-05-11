# get shell path
SOURCE="$0"
while [ -h "$SOURCE"  ]; do
    DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /*  ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
cd $DIR/../

img=$(cat ./hack/.version)

export DSECRET_IMAGE=$img
export DSECRET_DIR=/srv/node1/chain_data
export NAME=dsecret-1

export SIDE_CHAIN_PORT=30120
export SIDE_CHAIN_RPC_PORT=30121
export GQL_PORT=30125
export CHAIN_ENV=test


echo '' > ./hack/k8s.yaml
envsubst < ./hack/k8s-temp.yaml > ./hack/k8s.yaml

# 部署镜像
kubectl delete deployment dsecret-1 -n worker-addon
kubectl delete service dsecret-1-service -n worker-addon
kubectl create -f ./hack/k8s.yaml