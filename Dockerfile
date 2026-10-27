FROM wetee/ego-ubuntu-24-04:1.10.1
WORKDIR /

ADD hack/build/dsecret  /

RUN mkdir -p /token_chain_data

# 非 TEE 节点的 ClientSgxVerify 通过独立的 sgx-verify-service 容器验证 report，
# 服务地址用 SGX_VERIFY_SERVICE_URL 指定（见 hack/node2-deploy-docker.sh）。
CMD ["/bin/sh", "-c" ,"OE_SIMULATION=1 ego run dsecret"]
