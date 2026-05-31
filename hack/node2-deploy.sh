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
export DSECRET_DIR=/srv/token_node2/token_chain_data
export NAME=tee-provider-2

export SIDE_CHAIN_PORT=30136
export SIDE_CHAIN_RPC_PORT=30137
export GQL_PORT=30138
export CHAIN_ENV=test


echo '' > ./hack/k8s.yaml
envsubst < ./hack/k8s-temp.yaml > ./hack/k8s.yaml

# 部署镜像
kubectl delete deployment tee-provider-2 -n worker-addon
kubectl delete service tee-provider-2-service -n worker-addon
kubectl create -f ./hack/k8s.yaml
