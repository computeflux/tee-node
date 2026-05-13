FROM wetee/ego-ubuntu-24-04:1.8.1
WORKDIR /

ADD hack/build/dsecret  /

RUN mkdir -p /token_chain_data

CMD ["/bin/sh", "-c" ,"ego run dsecret"]
