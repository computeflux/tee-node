FROM wetee/ego-ubuntu-24-04:1.9.0
WORKDIR /

ADD hack/build/dsecret  /

RUN mkdir -p /token_chain_data

CMD ["/bin/sh", "-c" ,"OE_SIMULATION=1 ego run dsecret"]
