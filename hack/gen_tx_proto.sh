# get shell path
SOURCE="$0"
while [ -h "$SOURCE"  ]; do
    DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /*  ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
cd $DIR/../pkg/model


protoc --proto_path=. --gogofast_out=. tx.proto

rm -f ../../../libos-entry/model/tx.pb.go
cp tx.pb.go ../../../libos-entry/model/
cp -r protoio ../../../libos-entry/model/
