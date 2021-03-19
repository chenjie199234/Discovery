FROM busybox:1.28

RUN mkdir /root/app
WORKDIR /root/app
SHELL ["/bin/sh","-c"]
ENV SERVER_VERIFY_DATA='<SERVER_VERIFY_DATA>' \
    RUN_ENV=<RUN_ENV>
COPY main probe.sh ./
ENTRYPOINT ["./main"]
