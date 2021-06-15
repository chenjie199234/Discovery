FROM debian:stable-slim
RUN apt-get update && apt-get install -y ca-certificates && mkdir /root/app && mkdir /root/app/k8sconfig && mkdir /root/app/remoteconfig
WORKDIR /root/app
ENV SERVER_VERIFY_DATA='<SERVER_VERIFY_DATA>' \
    RUN_ENV=<RUN_ENV>
COPY main probe.sh ./
ENTRYPOINT ["./main"]
