FROM busybox:1.28

RUN mkdir /root/app
WORKDIR /root/app
SHELL ["/bin/sh","-c"]
ENV DISCOVERY_SERVER_VERIFY_DATA='<DISCOVERY_SERVER_VERIFY_DATA>' \
    RUN_ENV=<RUN_ENV>
COPY main probe.sh ./
ENTRYPOINT ["./main"]
