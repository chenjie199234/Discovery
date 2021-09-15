FROM debian:buster
RUN apt-get update && apt-get install -y ca-certificates curl procps neovim && mkdir /root/app && mkdir /root/app/k8sconfig && mkdir /root/app/remoteconfig
WORKDIR /root/app
COPY main probe.sh AppConfig.json SourceConfig.json ./
ENTRYPOINT ["./main"]
