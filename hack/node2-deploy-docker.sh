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

NAME=tee-provider-2
DSECRET_DIR=/srv/token_node2/token_chain_data

SIDE_CHAIN_PORT=30136
SIDE_CHAIN_RPC_PORT=30137
GQL_PORT=30138
CHAIN_ENV=test

# sgx-verify-service 统一用本地服务器的公网地址（宿主机 systemd 服务监听 0.0.0.0:8004，
# 见 hack/sgx-verify-service-deploy.sh）；如需指向其它地址可用环境变量覆盖。
SVS_HOST="${SVS_HOST:-xiaobai.asyou.me}"

# 幂等：先移除旧容器（等同原来的 kubectl delete）
docker rm -f $NAME 2>/dev/null || true

# 镜像 CMD 已内置 OE_SIMULATION=1（SGX 模拟模式），无需挂载 SGX 设备，直接本地 docker 部署
docker run -d \
    --name $NAME \
    --restart unless-stopped \
    -v $DSECRET_DIR:/token_chain_data \
    -p $SIDE_CHAIN_PORT:$SIDE_CHAIN_PORT \
    -p $SIDE_CHAIN_RPC_PORT:$SIDE_CHAIN_RPC_PORT \
    -p $GQL_PORT:$GQL_PORT \
    -e SIDE_CHAIN_PORT=$SIDE_CHAIN_PORT \
    -e GQL_PORT=$GQL_PORT \
    -e CHAIN_ENV=$CHAIN_ENV \
    -e SGX_VERIFY_SERVICE_URL=http://$SVS_HOST:8004 \
    -e NOTIFY_PUSH_SECRET=4a56b7a0d5eb5af09a5ec9ed00fa1518861219ef5a22d2d8eac329068c83aa6c \
    -e NOTIFY_ACTIVES_URL=http://xiaobai.asyou.me:9100 \
    $img

docker ps --filter name=$NAME
