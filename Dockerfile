FROM alpine:latest

# needed because this container communicates with CUPS on another container
RUN apk add --no-cache docker-cli

# NOTE: because of these args, if you want to build this manually you've to add
#       e.g. --build-arg TARGETARCH=amd64 to $ docker build ...

# "amd64" | "arm" | ...
ARG TARGETARCH
# usually empty. for "linux/arm/v7" => "v7"
ARG TARGETVARIANT


ENTRYPOINT ["print-ui"]

CMD ["run"]


COPY "rel/print-ui_linux-$TARGETARCH" /bin/print-ui
