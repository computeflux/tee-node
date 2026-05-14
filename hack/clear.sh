# get shell path
SOURCE="$0"
while [ -h "$SOURCE"  ]; do
    DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /*  ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
ROOT_DIR=/srv

cd $ROOT_DIR/node0
rm -rf ./token_chain_data/BFT
rm -rf ./token_chain_data/wetee
rm -rf ./token_chain_data/data/cs.wal
mkdir -p ./token_chain_data/data
cat <<EOF > ./token_chain_data/data/priv_validator_state.json
{
  "height": "0",
  "round": 0,
  "step": 3
}
EOF

cd $ROOT_DIR/node1
rm -rf ./token_chain_data/BFT
rm -rf ./token_chain_data/wetee
rm -rf ./token_chain_data/data/cs.wal
mkdir -p ./token_chain_data/data
cat <<EOF > ./token_chain_data/data/priv_validator_state.json
{
  "height": "0",
  "round": 0,
  "step": 3
}
EOF

cd $ROOT_DIR/node2
rm -rf ./token_chain_data/BFT
rm -rf ./token_chain_data/wetee
rm -rf ./token_chain_data/data/cs.wal
mkdir -p ./token_chain_data/data
cat <<EOF > ./token_chain_data/data/priv_validator_state.json
{
  "height": "0",
  "round": 0,
  "step": 3
}
EOF
