# get shell path
SOURCE="$0"
while [ -h "$SOURCE"  ]; do
    DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /*  ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
cd $DIR/../

tag=`date "+%Y-%m-%d-%H_%M"`

# build binary
# 注意: ego-go 自带一套 patch 过的 Go 工具链（/opt/ego/go，当前 go1.26.1），
# 与系统 go（如 1.26.6）共用同一个 build cache 时会混入不兼容的目标文件，
# 链接时报 "invalid relocation ... not in [0,N)"。这里用独立 GOCACHE 隔离。
GOCACHE="$HOME/.cache/ego-go-build" ego-go build -o ./hack/build/dsecret ./main.go

# build sgx verify service (cgo: 静态链接 /opt/ego/lib 的 liboehostverify; see tools/sgx-verify-service)
CGO_CFLAGS=-I/opt/ego/include CGO_LDFLAGS=-L/opt/ego/lib go build -o ./hack/build/sgx-verify-service ./tools/sgx-verify-service/

# sign binary
cd ./hack/build/
ego sign dsecret

cd $DIR/../

# 构建镜像
docker build -t wetee/tee-provider:$tag .
docker push wetee/tee-provider:$tag

echo "wetee/tee-provider:$tag" > ./hack/.version
